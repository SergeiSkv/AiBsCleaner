package analyzer

import (
	"go/ast"
)

// LoopContext provides proper loop detection for analyzers
type LoopContext struct {
}

// IsInLoop checks if a node is inside a loop by traversing the AST
func IsInLoop(root, target ast.Node) bool {
	var inLoop bool
	var targetFound bool

	ast.Inspect(root, func(n ast.Node) bool {
		if n == target {
			targetFound = true
			return false
		}

		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			// Check if target is inside this loop
			ast.Inspect(n, func(inner ast.Node) bool {
				if inner == target {
					inLoop = true
					targetFound = true
					return false
				}
				return true
			})
			if targetFound {
				return false
			}
		}

		return !targetFound
	})

	return inLoop
}

// GetLoopDepth returns the nesting depth of loops for a node
func GetLoopDepth(root, target ast.Node) int {
	depth := 0
	var measure func(ast.Node, int) bool

	measure = func(n ast.Node, currentDepth int) bool {
		if n == target {
			depth = currentDepth
			return false
		}

		switch node := n.(type) {
		case *ast.ForStmt:
			if node.Body != nil {
				ast.Inspect(node.Body, func(inner ast.Node) bool {
					return measure(inner, currentDepth+1)
				})
			}
			return false

		case *ast.RangeStmt:
			if node.Body != nil {
				ast.Inspect(node.Body, func(inner ast.Node) bool {
					return measure(inner, currentDepth+1)
				})
			}
			return false
		}

		return true
	}

	ast.Inspect(root, func(n ast.Node) bool {
		return measure(n, 0)
	})

	return depth
}

// AnalyzerWithContext provides context-aware analysis
type AnalyzerWithContext struct {
	root ast.Node
}

func NewAnalyzerWithContext(root ast.Node) *AnalyzerWithContext {
	return &AnalyzerWithContext{root: root}
}

func (a *AnalyzerWithContext) IsNodeInLoop(node ast.Node) bool {
	return IsInLoop(a.root, node)
}

func (a *AnalyzerWithContext) GetNodeLoopDepth(node ast.Node) int {
	return GetLoopDepth(a.root, node)
}
