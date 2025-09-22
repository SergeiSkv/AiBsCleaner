package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

type RegexAnalyzer struct{}

func NewRegexAnalyzer() *RegexAnalyzer {
	return &RegexAnalyzer{}
}

func (ra *RegexAnalyzer) Name() string {
	return "RegexAnalyzer"
}

func (ra *RegexAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if ra.isRegexpCompileInLoop(node) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "REGEX_COMPILE_IN_LOOP",
					Severity:   SeverityHigh,
					Message:    "Regular expression compiled inside loop",
					Suggestion: "Compile regex once outside the loop and reuse",
				})
			}

			if ra.isRegexpCompileWithoutMust(node) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "REGEX_WITHOUT_MUST",
					Severity:   SeverityLow,
					Message:    "regexp.Compile used without MustCompile for static pattern",
					Suggestion: "Use regexp.MustCompile for compile-time regex validation",
				})
			}
		}
		return true
	})

	return issues
}

func (ra *RegexAnalyzer) isRegexpCompileInLoop(call *ast.CallExpr) bool {
	if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			if ident.Name == "regexp" && strings.Contains(selExpr.Sel.Name, "Compile") {
				return ra.isInLoop(call)
			}
		}
	}
	return false
}

func (ra *RegexAnalyzer) isRegexpCompileWithoutMust(call *ast.CallExpr) bool {
	if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			if ident.Name == "regexp" && selExpr.Sel.Name == "Compile" {
				if len(call.Args) > 0 {
					if lit, ok := call.Args[0].(*ast.BasicLit); ok {
						return lit.Kind == token.STRING
					}
				}
			}
		}
	}
	return false
}

func (ra *RegexAnalyzer) isInLoop(node ast.Node) bool {
	parent := node
	depth := 0
	maxDepth := MaxSearchDepth

	for depth < maxDepth {
		switch parent.(type) {
		case *ast.ForStmt:
			return true
		case *ast.RangeStmt:
			return true
		}
		depth++
	}

	return false
}
