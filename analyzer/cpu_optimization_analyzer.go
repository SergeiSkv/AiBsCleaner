package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// CPUOptimizationAnalyzer detects CPU performance anti-patterns
type CPUOptimizationAnalyzer struct {
	hotPaths map[string]bool
}

func NewCPUOptimizationAnalyzer() *CPUOptimizationAnalyzer {
	return &CPUOptimizationAnalyzer{
		hotPaths: make(map[string]bool),
	}
}

func (coa *CPUOptimizationAnalyzer) Name() string {
	return "CPUOptimizationAnalyzer"
}

func (coa *CPUOptimizationAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Identify hot paths based on function names
	ast.Inspect(astNode, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			name := fn.Name.Name
			// Common hot path indicators
			hotPathKeywords := []string{"Process", "Handle", "Parse", "Serialize", "Compute", "Calculate", "Render", "Execute"}
			for _, keyword := range hotPathKeywords {
				if strings.Contains(name, keyword) {
					coa.hotPaths[name] = true
					break
				}
			}
		}
		return true
	})

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			if node.Body != nil {
				issues = append(issues, coa.analyzeFunctionCPU(node, filename, fset)...)
			}

		case *ast.IfStmt:
			// Check for branch misprediction patterns
			if coa.isUnpredictableBranch(node) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "UNPREDICTABLE_BRANCH",
					Severity:   SeverityLow,
					Message:    "Branch with random condition causes CPU misprediction",
					Suggestion: "Consider branch-free alternatives or reorder conditions by likelihood",
				})
			}

		case *ast.RangeStmt:
			// Check for interface boxing in loops
			if coa.hasInterfaceBoxing(node.Body) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "INTERFACE_BOXING_IN_LOOP",
					Severity:   SeverityHigh,
					Message:    "Interface boxing in loop causes allocations and CPU overhead",
					Suggestion: "Use concrete types in performance-critical loops",
				})
			}

		case *ast.CallExpr:
			// Check for repeated len() calls
			if ident, ok := node.Fun.(*ast.Ident); ok && ident.Name == "len" {
				if coa.isInLoop(n) {
					pos := fset.Position(node.Pos())
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "REPEATED_LEN_CALL",
						Severity:   SeverityLow,
						Message:    "Repeated len() call in loop condition",
						Suggestion: "Cache length in variable before loop",
					})
				}
			}

			// Check for expensive operations in hot path
			if coa.isInHotPath(n) {
				if coa.isExpensiveOperation(node) {
					pos := fset.Position(node.Pos())
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "EXPENSIVE_OP_IN_HOT_PATH",
						Severity:   SeverityHigh,
						Message:    "Expensive operation in hot path degrades performance",
						Suggestion: "Move operation outside hot path or cache results",
					})
				}
			}

		case *ast.BinaryExpr:
			// Check for inefficient modulo operations
			if node.Op == token.REM {
				if lit, ok := node.Y.(*ast.BasicLit); ok {
					if coa.isPowerOfTwo(lit.Value) {
						pos := fset.Position(node.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "MODULO_POWER_OF_TWO",
							Severity:   SeverityLow,
							Message:    "Modulo with power of 2 can be optimized to bitwise AND",
							Suggestion: "Use x & (n-1) instead of x % n for power of 2",
						})
					}
				}
			}

		case *ast.StructType:
			// Check for false sharing potential
			issues = append(issues, coa.checkFalseSharing(node, filename, fset)...)
		}
		return true
	})

	return issues
}

func (coa *CPUOptimizationAnalyzer) analyzeFunctionCPU(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	// Check for bounds checking that can be eliminated
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if index, ok := n.(*ast.IndexExpr); ok {
			// Check if index is constant and could use unsafe
			if coa.isConstantIndex(index.Index) && coa.isInHotPath(fn) {
				pos := fset.Position(index.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "BOUNDS_CHECK_IN_HOT_PATH",
					Severity:   SeverityLow,
					Message:    "Bounds checking in hot path adds CPU overhead",
					Suggestion: "Consider unsafe access if bounds are guaranteed",
				})
			}
		}

		// Check for map access in hot path (hash computation overhead)
		if _, ok := n.(*ast.MapType); ok {
			if coa.isInHotPath(fn) {
				pos := fset.Position(n.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "MAP_IN_HOT_PATH",
					Severity:   SeverityMedium,
					Message:    "Map access in hot path has hash computation overhead",
					Suggestion: "Consider array or slice with direct indexing for small fixed sets",
				})
			}
		}

		// Check for unnecessary type assertions
		if assert, ok := n.(*ast.TypeAssertExpr); ok {
			if assert.Type != nil && coa.isInLoop(n) {
				pos := fset.Position(assert.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "TYPE_ASSERTION_IN_LOOP",
					Severity:   SeverityMedium,
					Message:    "Type assertion in loop adds CPU overhead",
					Suggestion: "Assert type once before loop or use concrete types",
				})
			}
		}

		return true
	})

	// Check for function inlining prevention
	if coa.preventsInlining(fn) {
		pos := fset.Position(fn.Pos())
		issues = append(issues, Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       "PREVENTS_INLINING",
			Severity:   SeverityMedium,
			Message:    "Function too complex to inline, adds call overhead",
			Suggestion: "Simplify function or split into smaller functions for inlining",
		})
	}

	return issues
}

func (coa *CPUOptimizationAnalyzer) checkFalseSharing(st *ast.StructType, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	if st.Fields == nil {
		return issues
	}

	// Check for frequently accessed fields that might cause false sharing
	hotFields := []string{}
	for _, field := range st.Fields.List {
		for _, name := range field.Names {
			// Common hot field patterns
			if strings.Contains(strings.ToLower(name.Name), "counter") ||
				strings.Contains(strings.ToLower(name.Name), "flag") ||
				strings.Contains(strings.ToLower(name.Name), "atomic") {
				hotFields = append(hotFields, name.Name)
			}
		}
	}

	if len(hotFields) > 1 {
		pos := fset.Position(st.Pos())
		issues = append(issues, Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       "POTENTIAL_FALSE_SHARING",
			Severity:   SeverityMedium,
			Message:    "Multiple hot fields in struct may cause false sharing",
			Suggestion: "Add padding between hot fields or use cache line alignment",
		})
	}

	return issues
}

func (coa *CPUOptimizationAnalyzer) isUnpredictableBranch(ifStmt *ast.IfStmt) bool {
	// Check for conditions with random/unpredictable values
	if call, ok := ifStmt.Cond.(*ast.CallExpr); ok {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				// Check for random number generators
				if ident.Name == "rand" || ident.Name == "math" {
					return true
				}
			}
		}
	}
	return false
}

func (coa *CPUOptimizationAnalyzer) hasInterfaceBoxing(block *ast.BlockStmt) bool {
	hasBoxing := false
	ast.Inspect(block, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			for _, lhs := range assign.Lhs {
				// Check if assigning to interface{}
				if _, ok := lhs.(*ast.InterfaceType); ok {
					hasBoxing = true
					return false
				}
			}
		}
		return true
	})
	return hasBoxing
}

func (coa *CPUOptimizationAnalyzer) isInLoop(node ast.Node) bool {
	// Simplified - would need proper context tracking
	return false
}

func (coa *CPUOptimizationAnalyzer) isInHotPath(node ast.Node) bool {
	// Check if we're in a function marked as hot path
	return len(coa.hotPaths) > 0
}

func (coa *CPUOptimizationAnalyzer) isExpensiveOperation(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		expensiveOps := map[string][]string{
			"runtime":  {"GC", "Gosched", "GOMAXPROCS"},
			"reflect":  {"TypeOf", "ValueOf", "DeepEqual"},
			"fmt":      {"Sprintf", "Printf", "Fprintf"},
			"encoding": {"Marshal", "Unmarshal"},
			"json":     {"Marshal", "Unmarshal", "Encode", "Decode"},
			"regexp":   {"Compile", "MustCompile", "Match"},
			"sync":     {"Once"},
		}

		if ident, ok := sel.X.(*ast.Ident); ok {
			if ops, exists := expensiveOps[ident.Name]; exists {
				for _, op := range ops {
					if sel.Sel.Name == op {
						return true
					}
				}
			}
		}
	}
	return false
}

func (coa *CPUOptimizationAnalyzer) isPowerOfTwo(value string) bool {
	// Check if the literal is a power of 2
	powerOfTwoValues := []string{"2", "4", "8", "16", "32", "64", "128", "256", "512", "1024"}
	for _, v := range powerOfTwoValues {
		if value == v {
			return true
		}
	}
	return false
}

func (coa *CPUOptimizationAnalyzer) isConstantIndex(expr ast.Expr) bool {
	_, ok := expr.(*ast.BasicLit)
	return ok
}

func (coa *CPUOptimizationAnalyzer) preventsInlining(fn *ast.FuncDecl) bool {
	// Functions with defer, recover, or too many statements prevent inlining
	hasDefer := false
	stmtCount := 0

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.DeferStmt:
			hasDefer = true
			return false
		case ast.Stmt:
			stmtCount++
		}
		return true
	})

	// Go compiler inlining budget is ~80 nodes
	return hasDefer || stmtCount > 40
}
