package analyzer

import (
	"go/ast"
	"go/token"
)

type ReflectionAnalyzer struct{}

func NewReflectionAnalyzer() *ReflectionAnalyzer {
	return &ReflectionAnalyzer{}
}

func (ra *ReflectionAnalyzer) Name() string {
	return "ReflectionAnalyzer"
}

func (ra *ReflectionAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if ra.isReflectCall(node) {
				if ra.isInHotPath(n) {
					pos := fset.Position(node.Pos())
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "REFLECTION_IN_HOT_PATH",
						Severity:   SeverityHigh,
						Message:    "Reflection usage detected in potential hot path",
						Suggestion: "Consider using type assertions or code generation instead of reflection",
					})
				}
			}
		}
		return true
	})

	return issues
}

func (ra *ReflectionAnalyzer) isReflectCall(call *ast.CallExpr) bool {
	if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			if ident.Name == "reflect" {
				return true
			}
		}
	}
	return false
}

func (ra *ReflectionAnalyzer) isInHotPath(node ast.Node) bool {
	parent := node
	depth := 0
	maxDepth := MaxSearchDepth

	for depth < maxDepth {
		switch parent.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return true
		}
		depth++
	}

	return false
}