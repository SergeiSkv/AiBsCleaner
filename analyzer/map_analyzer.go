package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type MapAnalyzer struct{}

func NewMapAnalyzer() Analyzer {
	return &MapAnalyzer{}
}

func (ma *MapAnalyzer) Name() string {
	return "Map Optimization"
}

func (ma *MapAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	visitor := &mapVisitor{
		fset:      fset,
		filename:  filename,
		loopDepth: 0,
		issues:    make([]*models.Issue, 0, 8),
	}

	ast.Walk(visitor, file)
	return visitor.issues
}

type mapVisitor struct {
	fset      *token.FileSet
	filename  string
	loopDepth int
	issues    []*models.Issue
}

func (v *mapVisitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.ForStmt:
		v.loopDepth++
		if n.Body != nil {
			ast.Walk(v, n.Body)
		}
		v.loopDepth--
		return nil
	case *ast.RangeStmt:
		v.loopDepth++
		if n.Body != nil {
			ast.Walk(v, n.Body)
		}
		v.loopDepth--
		return nil
	case *ast.CallExpr:
		v.inspectCall(n)
	}
	return v
}

func (v *mapVisitor) inspectCall(call *ast.CallExpr) {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != funcMake {
		return
	}

	if len(call.Args) == 0 {
		return
	}

	if _, ok := call.Args[0].(*ast.MapType); !ok {
		return
	}

	// map created inside loop without capacity hint
	if v.loopDepth > 0 && len(call.Args) == 1 {
		pos := v.fset.Position(call.Pos())
		v.issues = append(v.issues, &models.Issue{
			File:       v.filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       models.IssueMapCapacity,
			Severity:   models.SeverityLevelMedium,
			Message:    "Map allocation inside loop without capacity hint",
			Suggestion: "Pass expected size to make(map[K]V, size) or reuse a cleared map",
		})
		return
	}

	// map literal created per iteration; flag as GC pressure
	if v.loopDepth > 0 {
		pos := v.fset.Position(call.Pos())
		v.issues = append(v.issues, &models.Issue{
			File:       v.filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       models.IssueMapCapacity,
			Severity:   models.SeverityLevelLow,
			Message:    "map allocation inside loop",
			Suggestion: "Reuse map or allocate once before the loop",
		})
	}
}
