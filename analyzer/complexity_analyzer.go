package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
)

type ComplexityAnalyzer struct{}

func NewComplexityAnalyzer() *ComplexityAnalyzer {
	return &ComplexityAnalyzer{}
}

func (ca *ComplexityAnalyzer) Name() string {
	return "ComplexityAnalyzer"
}

func (ca *ComplexityAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			complexity := ca.calculateComplexity(node.Body)
			if complexity.nestedLoops >= HighComplexityThreshold {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "HIGH_COMPLEXITY_O3",
					Severity:   SeverityHigh,
					Message:    fmt.Sprintf("Function has O(n³) or higher complexity with %d nested loops", complexity.nestedLoops),
					Suggestion: "Consider breaking down the function or optimizing the algorithm",
				})
			} else if complexity.nestedLoops >= MediumComplexityThreshold {
				if complexity.hasExpensiveOps {
					pos := fset.Position(node.Pos())
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "HIGH_COMPLEXITY_O2_EXPENSIVE",
						Severity:   SeverityMedium,
						Message:    "O(n²) complexity with expensive operations inside loops",
						Suggestion: "Consider using more efficient data structures or algorithms",
					})
				}
			}

			if complexity.recursiveDepth > 0 && !complexity.hasTailRecursion {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "NON_TAIL_RECURSION",
					Severity:   SeverityMedium,
					Message:    "Non-tail recursive function can cause stack overflow",
					Suggestion: "Convert to tail recursion or iterative approach",
				})
			}
		}
		return true
	})

	return issues
}

type complexityInfo struct {
	nestedLoops      int
	recursiveDepth   int
	hasExpensiveOps  bool
	hasTailRecursion bool
}

func (ca *ComplexityAnalyzer) calculateComplexity(block *ast.BlockStmt) complexityInfo {
	if block == nil {
		return complexityInfo{}
	}

	info := complexityInfo{}
	ca.analyzeBlock(block, 0, &info)
	return info
}

func (ca *ComplexityAnalyzer) analyzeBlock(node ast.Node, currentDepth int, info *complexityInfo) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.ForStmt:
			newDepth := currentDepth + 1
			if newDepth > info.nestedLoops {
				info.nestedLoops = newDepth
			}

			if stmt.Body != nil {
				ca.analyzeBlock(stmt.Body, newDepth, info)
				ca.checkExpensiveOperations(stmt.Body, info)
			}
			return false

		case *ast.RangeStmt:
			newDepth := currentDepth + 1
			if newDepth > info.nestedLoops {
				info.nestedLoops = newDepth
			}

			if stmt.Body != nil {
				ca.analyzeBlock(stmt.Body, newDepth, info)
				ca.checkExpensiveOperations(stmt.Body, info)
			}
			return false

		case *ast.CallExpr:
			if ca.isRecursiveCall(stmt) {
				info.recursiveDepth++
				info.hasTailRecursion = ca.isTailRecursive(n)
			}

			if ca.isExpensiveOperation(stmt) {
				info.hasExpensiveOps = true
			}
		}
		return true
	})
}

func (ca *ComplexityAnalyzer) checkExpensiveOperations(block *ast.BlockStmt, info *complexityInfo) {
	ast.Inspect(block, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if ca.isExpensiveOperation(call) {
				info.hasExpensiveOps = true
			}
		}
		return true
	})
}

func (ca *ComplexityAnalyzer) isExpensiveOperation(call *ast.CallExpr) bool {
	if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		expensiveOps := map[string][]string{
			"sort":    {"Sort", "Slice", "Search"},
			"strings": {"Contains", "Index", "Replace"},
			"regexp":  {"Compile", "MustCompile", "Match"},
			"fmt":     {"Sprintf", "Printf"},
		}

		if ident, ok := selExpr.X.(*ast.Ident); ok {
			if ops, exists := expensiveOps[ident.Name]; exists {
				for _, op := range ops {
					if selExpr.Sel.Name == op {
						return true
					}
				}
			}
		}
	}

	if ident, ok := call.Fun.(*ast.Ident); ok {
		return ident.Name == "append" || ident.Name == "copy"
	}

	return false
}

func (ca *ComplexityAnalyzer) isRecursiveCall(call *ast.CallExpr) bool {
	// For proper recursion detection, we need function context
	// Without it, we can't accurately detect recursion, so return false to avoid false positives
	// TODO: Implement proper recursion detection with function context tracking
	return false
}

func (ca *ComplexityAnalyzer) isTailRecursive(node ast.Node) bool {
	// Check if the recursive call is the last statement
	parent := node
	for parent != nil {
		if _, ok := parent.(*ast.ReturnStmt); ok {
			return true
		}
		// Would need proper parent tracking for accurate detection
		break
	}
	return false
}
