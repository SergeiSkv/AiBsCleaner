package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type RegexAnalyzer struct{}

func NewRegexAnalyzer() Analyzer {
	return &RegexAnalyzer{}
}

func (ra *RegexAnalyzer) Name() string {
	return "Regex Performance"
}

func (ra *RegexAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	visitor := &regexVisitor{
		fset:      fset,
		filename:  filename,
		loopDepth: 0,
		issues:    make([]*models.Issue, 0, 4),
	}

	ast.Walk(visitor, file)
	return visitor.issues
}

type regexVisitor struct {
	fset      *token.FileSet
	filename  string
	loopDepth int
	issues    []*models.Issue
}

func (v *regexVisitor) Visit(node ast.Node) ast.Visitor {
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

func (v *regexVisitor) inspectCall(call *ast.CallExpr) {
	if v.loopDepth == 0 {
		return
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok || pkgIdent.Name != pkgRegexp {
		return
	}

	if sel.Sel.Name != methodCompile && sel.Sel.Name != methodMustCompile {
		return
	}

	pos := v.fset.Position(call.Pos())
	v.issues = append(v.issues, &models.Issue{
		File:       v.filename,
		Line:       pos.Line,
		Column:     pos.Column,
		Position:   pos,
		Type:       models.IssueRegexCompileInLoop,
		Severity:   models.SeverityLevelMedium,
		Message:    "Regular expression compiled inside loop",
		Suggestion: "Compile regex once outside loop and reuse",
	})
}
