package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type TimeAnalyzer struct{}

func NewTimeAnalyzer() Analyzer {
	return &TimeAnalyzer{}
}

func (ta *TimeAnalyzer) Name() string {
	return "Time Operations"
}

func (ta *TimeAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	visitor := &timeVisitor{
		fset:      fset,
		filename:  filename,
		loopDepth: 0,
		issues:    make([]*models.Issue, 0, 4),
	}

	ast.Walk(visitor, file)
	return visitor.issues
}

type timeVisitor struct {
	fset      *token.FileSet
	filename  string
	loopDepth int
	issues    []*models.Issue
}

func (v *timeVisitor) Visit(node ast.Node) ast.Visitor {
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

func (v *timeVisitor) inspectCall(call *ast.CallExpr) {
	if v.loopDepth == 0 {
		return
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok || pkgIdent.Name != pkgTime || sel.Sel.Name != methodNow {
		return
	}

	pos := v.fset.Position(call.Pos())
	v.issues = append(v.issues, &models.Issue{
		File:       v.filename,
		Line:       pos.Line,
		Column:     pos.Column,
		Position:   pos,
		Type:       models.IssueTimeNowInLoop,
		Severity:   models.SeverityLevelLow,
		Message:    "time.Now() inside loop",
		Suggestion: "Compute time once outside the loop",
	})
}
