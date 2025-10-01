package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type HTTPReuseAnalyzer struct{}

func NewHTTPReuseAnalyzer() Analyzer {
	return &HTTPReuseAnalyzer{}
}

func (ha *HTTPReuseAnalyzer) Name() string {
	return "HTTP Connection Reuse"
}

func (ha *HTTPReuseAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	visitor := &httpReuseVisitor{
		fset:     fset,
		filename: filename,
		issues:   make([]*models.Issue, 0, 8),
	}

	ast.Walk(visitor, file)
	return visitor.issues
}

type httpReuseVisitor struct {
	fset      *token.FileSet
	filename  string
	loopDepth int
	issues    []*models.Issue
}

func (v *httpReuseVisitor) Visit(node ast.Node) ast.Visitor {
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

func (v *httpReuseVisitor) inspectCall(call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}

	if pkgIdent.Name == pkgHTTP {
		switch sel.Sel.Name {
		case methodGet, methodPost, methodHead, methodPostForm:
			pos := v.fset.Position(call.Pos())
			v.issues = append(v.issues, &models.Issue{
				File:       v.filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       models.IssueHTTPNoConnectionReuse,
				Severity:   models.SeverityLevelMedium,
				Message:    "Direct http." + sel.Sel.Name + " call creates new transport",
				Suggestion: "Reuse a shared http.Client for connection pooling",
			})
		}
	}

	if v.loopDepth > 0 {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if sel.Sel.Name == "RoundTrip" {
				pos := v.fset.Position(call.Pos())
				v.issues = append(v.issues, &models.Issue{
					File:       v.filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       models.IssueHTTPNoConnectionReuse,
					Severity:   models.SeverityLevelLow,
					Message:    "Custom RoundTrip in loop; ensure transport is reused",
					Suggestion: "Cache transports rather than recreating per iteration",
				})
			}
		}
	}
}
