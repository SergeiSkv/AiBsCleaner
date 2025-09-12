package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type HTTPClientAnalyzer struct{}

func NewHTTPClientAnalyzer() Analyzer {
	return &HTTPClientAnalyzer{}
}

func (hca *HTTPClientAnalyzer) Name() string {
	return "HTTPClientAnalyzer"
}

func (hca *HTTPClientAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	issues := make([]*models.Issue, 0, 8)

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if issue := detectDefaultClient(node, fset, filename); issue != nil {
				issues = append(issues, issue)
			}
		case *ast.CompositeLit:
			if issue := detectMissingTimeout(node, fset, filename); issue != nil {
				issues = append(issues, issue)
			}
		}
		return true
	})

	return issues
}

func detectDefaultClient(call *ast.CallExpr, fset *token.FileSet, filename string) *models.Issue {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok || pkgIdent.Name != pkgHTTP {
		return nil
	}

	switch sel.Sel.Name {
	case methodGet, methodPost, methodHead, methodPostForm:
		pos := fset.Position(call.Pos())
		return &models.Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       models.IssueHTTPNoTimeout,
			Severity:   models.SeverityLevelMedium,
			Message:    "http." + sel.Sel.Name + " uses the default client without timeout",
			Suggestion: "Use a custom http.Client with Timeout set",
		}
	}

	return nil
}

func detectMissingTimeout(comp *ast.CompositeLit, fset *token.FileSet, filename string) *models.Issue {
	if comp == nil {
		return nil
	}

	sel, ok := comp.Type.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok || pkgIdent.Name != pkgHTTP || sel.Sel.Name != methodClient {
		return nil
	}

	for _, elt := range comp.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if ident, ok := kv.Key.(*ast.Ident); ok && ident.Name == methodTimeout {
				return nil
			}
		}
	}

	pos := fset.Position(comp.Pos())
	return &models.Issue{
		File:       filename,
		Line:       pos.Line,
		Column:     pos.Column,
		Position:   pos,
		Type:       models.IssueHTTPNoTimeout,
		Severity:   models.SeverityLevelHigh,
		Message:    "http.Client literal missing Timeout",
		Suggestion: "Set Timeout when constructing http.Client",
	}
}
