package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type SliceAnalyzer struct{}

func NewSliceAnalyzer() Analyzer {
	return &SliceAnalyzer{}
}

func (sa *SliceAnalyzer) Name() string {
	return "Slice Optimization"
}

func (sa *SliceAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	visitor := &sliceVisitor{
		fset:      fset,
		filename:  filename,
		loopDepth: 0,
		issues:    make([]*models.Issue, 0, 8),
	}

	ast.Walk(visitor, file)
	return visitor.issues
}

type sliceVisitor struct {
	fset      *token.FileSet
	filename  string
	loopDepth int
	issues    []*models.Issue
}

func (v *sliceVisitor) Visit(node ast.Node) ast.Visitor {
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

func (v *sliceVisitor) inspectCall(call *ast.CallExpr) {
	if v.loopDepth == 0 {
		return
	}

	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != "append" {
		return
	}

	pos := v.fset.Position(call.Pos())
	v.issues = append(v.issues, &models.Issue{
		File:       v.filename,
		Line:       pos.Line,
		Column:     pos.Column,
		Position:   pos,
		Type:       models.IssueSliceCapacity,
		Severity:   models.SeverityLevelMedium,
		Message:    "append inside loop may cause repeated allocations",
		Suggestion: "Preallocate slice capacity or reuse a buffer",
	})
}
