package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type ContextAnalyzer struct{}

func NewContextAnalyzer() Analyzer {
	return &ContextAnalyzer{}
}

func (ca *ContextAnalyzer) Name() string {
	return "ContextAnalyzer"
}

func (ca *ContextAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	ctx := newContextAnalysis(fset, filename)
	ast.Walk(&contextVisitor{ctx: ctx}, file)
	return ctx.issues
}

type contextVisitor struct {
	ctx *contextAnalysis
}

func (v *contextVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	switch n := node.(type) {
	case *ast.FuncDecl:
		v.ctx.inspectFunction(n)
		return nil
	case *ast.CallExpr:
		v.ctx.inspectCall(n)
	}

	return v
}

type contextAnalysis struct {
	fset     *token.FileSet
	filename string
	issues   []*models.Issue
}

func newContextAnalysis(fset *token.FileSet, filename string) *contextAnalysis {
	return &contextAnalysis{
		fset:     fset,
		filename: filename,
		issues:   make([]*models.Issue, 0, 8),
	}
}

func (ca *contextAnalysis) inspectFunction(fn *ast.FuncDecl) {
	if fn.Type == nil || fn.Type.Params == nil {
		return
	}

	params := fn.Type.Params.List
	for i, field := range params {
		if field.Type == nil {
			continue
		}
		if isContextType(field.Type) {
			if i > 0 {
				pos := ca.fset.Position(field.Pos())
				ca.addIssue(pos, models.IssueContextNotFirst, "context.Context should be the first parameter")
			}
			break
		}
	}
}

func (ca *contextAnalysis) inspectCall(call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok || pkgIdent.Name != pkgContext {
		return
	}

	if sel.Sel.Name != "WithValue" {
		return
	}
	if len(call.Args) < 2 {
		return
	}
	if _, ok := call.Args[1].(*ast.BasicLit); ok {
		pos := ca.fset.Position(call.Args[1].Pos())
		ca.addIssue(pos, models.IssueContextValue, "avoid using basic types as context keys")
	}
}

func (ca *contextAnalysis) addIssue(pos token.Position, issueType models.IssueType, msg string) {
	ca.issues = append(ca.issues, &models.Issue{
		File:     ca.filename,
		Line:     pos.Line,
		Column:   pos.Column,
		Position: pos,
		Type:     issueType,
		Severity: issueType.Severity(),
		Message:  msg,
	})
}

func isContextType(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return pkgIdent.Name == pkgContext && sel.Sel.Name == "Context"
}
