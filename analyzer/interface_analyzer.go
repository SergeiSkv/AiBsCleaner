package analyzer

import (
	"go/ast"
	"go/token"
)

type InterfaceAnalyzer struct{}

func NewInterfaceAnalyzer() *InterfaceAnalyzer {
	return &InterfaceAnalyzer{}
}

func (ia *InterfaceAnalyzer) Name() string {
	return "InterfaceAnalyzer"
}

func (ia *InterfaceAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.TypeAssertExpr:
			if node.Type == nil {
				if ia.isInLoop(n) {
					pos := fset.Position(node.Pos())
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "TYPE_ASSERTION_IN_LOOP",
						Severity:   SeverityMedium,
						Message:    "Type assertion in loop has performance overhead",
						Suggestion: "Cache type assertion result outside loop if possible",
					})
				}
			}
		case *ast.CallExpr:
			if ia.isEmptyInterfaceParam(node) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "EMPTY_INTERFACE_USAGE",
					Severity:   SeverityLow,
					Message:    "interface{} parameter causes allocation and type checking overhead",
					Suggestion: "Use concrete types or specific interfaces when possible",
				})
			}
		}
		return true
	})

	return issues
}

func (ia *InterfaceAnalyzer) isInLoop(node ast.Node) bool {
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

func (ia *InterfaceAnalyzer) isEmptyInterfaceParam(call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		if _, ok := arg.(*ast.InterfaceType); ok {
			return true
		}
	}
	return false
}