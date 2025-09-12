package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

type DeferOptimizationAnalyzer struct{}

func NewDeferOptimizationAnalyzer() *DeferOptimizationAnalyzer {
	return &DeferOptimizationAnalyzer{}
}

func (doa *DeferOptimizationAnalyzer) Name() string {
	return "DeferOptimizationAnalyzer"
}

func (doa *DeferOptimizationAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			issues = append(issues, doa.analyzeFunction(node, filename, fset)...)
		case *ast.FuncLit:
			issues = append(issues, doa.analyzeFuncLit(node, filename, fset)...)
		}
		return true
	})

	return issues
}

func (doa *DeferOptimizationAnalyzer) analyzeFunction(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	if fn.Body == nil {
		return issues
	}

	deferStmts := doa.collectDeferStatements(fn.Body)

	// Check for unnecessary defer overhead
	for _, deferStmt := range deferStmts {
		// Check if defer is used for simple operations
		if doa.isSimpleOperation(deferStmt) {
			pos := fset.Position(deferStmt.Pos())
			issues = append(issues, Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       "UNNECESSARY_DEFER",
				Severity:   SeverityLow,
				Message:    "Unnecessary defer for simple operation adds overhead",
				Suggestion: "Call directly without defer for better performance",
			})
		}

		// Check if defer is at the end of function (pointless)
		if doa.isDeferAtEnd(deferStmt, fn.Body) {
			pos := fset.Position(deferStmt.Pos())
			issues = append(issues, Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       "DEFER_AT_END",
				Severity:   SeverityMedium,
				Message:    "Defer at the end of function is unnecessary",
				Suggestion: "Execute directly without defer",
			})
		}

		// Check for multiple defers that could be combined
		if doa.hasMultipleSimilarDefers(deferStmts) {
			pos := fset.Position(deferStmt.Pos())
			issues = append(issues, Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       "MULTIPLE_DEFERS",
				Severity:   SeverityLow,
				Message:    "Multiple defer statements add cumulative overhead",
				Suggestion: "Combine multiple defers into a single cleanup function",
			})
			break // Report once per function
		}

		// Check for defer in hot path
		if doa.isInHotPath(deferStmt, fn) {
			pos := fset.Position(deferStmt.Pos())
			issues = append(issues, Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       "DEFER_IN_HOT_PATH",
				Severity:   SeverityMedium,
				Message:    "Defer in performance-critical path adds overhead (~30ns)",
				Suggestion: "Consider manual cleanup for hot paths",
			})
		}

		// Check for defer with closure capturing large variables
		if doa.capturesLargeVariables(deferStmt) {
			pos := fset.Position(deferStmt.Pos())
			issues = append(issues, Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       "DEFER_LARGE_CAPTURE",
				Severity:   SeverityMedium,
				Message:    "Defer closure captures large variables increasing memory usage",
				Suggestion: "Pass values as parameters instead of capturing",
			})
		}

		// Check for unnecessary defer for mutex unlock
		if doa.isUnnecessaryMutexDefer(deferStmt, fn.Body) {
			pos := fset.Position(deferStmt.Pos())
			issues = append(issues, Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       "UNNECESSARY_MUTEX_DEFER",
				Severity:   SeverityLow,
				Message:    "Defer for mutex unlock in simple function adds unnecessary overhead",
				Suggestion: "Unlock directly before return if no complex control flow",
			})
		}
	}

	// Check for missing defer where it should be used
	issues = append(issues, doa.checkMissingDefer(fn, filename, fset)...)

	return issues
}

func (doa *DeferOptimizationAnalyzer) analyzeFuncLit(fn *ast.FuncLit, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	if fn.Body == nil {
		return issues
	}

	// Check for defer in short anonymous functions
	deferStmts := doa.collectDeferStatements(fn.Body)
	if len(deferStmts) > 0 && doa.isShortFunction(fn.Body) {
		pos := fset.Position(deferStmts[0].Pos())
		issues = append(issues, Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       "DEFER_IN_SHORT_FUNC",
			Severity:   SeverityLow,
			Message:    "Defer in short function adds unnecessary overhead",
			Suggestion: "Call cleanup directly in short functions",
		})
	}

	return issues
}

func (doa *DeferOptimizationAnalyzer) collectDeferStatements(block *ast.BlockStmt) []*ast.DeferStmt {
	var defers []*ast.DeferStmt

	ast.Inspect(block, func(n ast.Node) bool {
		if d, ok := n.(*ast.DeferStmt); ok {
			defers = append(defers, d)
		}
		return true
	})

	return defers
}

func (doa *DeferOptimizationAnalyzer) isSimpleOperation(deferStmt *ast.DeferStmt) bool {
	// Check if it's a simple function call without complex arguments
	call := deferStmt.Call
	// Simple operations that don't need defer
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		simpleOps := []string{"Close", "Unlock", "Done", "Release"}
		for _, op := range simpleOps {
			if sel.Sel.Name == op && len(call.Args) == 0 {
				return true
			}
		}
	}
	return false
}

func (doa *DeferOptimizationAnalyzer) isDeferAtEnd(deferStmt *ast.DeferStmt, body *ast.BlockStmt) bool {
	if len(body.List) < 2 {
		return false
	}

	// Check if defer is one of the last statements
	for i := len(body.List) - 2; i < len(body.List); i++ {
		if i >= 0 && body.List[i] == deferStmt {
			// Check if there are no meaningful statements after defer
			hasOnlyReturns := true
			for j := i + 1; j < len(body.List); j++ {
				if _, isReturn := body.List[j].(*ast.ReturnStmt); !isReturn {
					hasOnlyReturns = false
					break
				}
			}
			return hasOnlyReturns
		}
	}

	return false
}

func (doa *DeferOptimizationAnalyzer) hasMultipleSimilarDefers(defers []*ast.DeferStmt) bool {
	// If there are 3+ defers, it might be worth combining
	if len(defers) >= MaxNestedLoops {
		// Check if they're similar operations (e.g., multiple Close() calls)
		closeCount := 0
		unlockCount := 0

		for _, d := range defers {
			call := d.Call
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				switch sel.Sel.Name {
				case "Close":
					closeCount++
				case "Unlock", "RUnlock":
					unlockCount++
				}
			}
		}

		return closeCount >= MaxNestedLoops || unlockCount >= MaxNestedLoops
	}

	return false
}

func (doa *DeferOptimizationAnalyzer) isInHotPath(deferStmt *ast.DeferStmt, fn *ast.FuncDecl) bool {
	// Check if function name indicates hot path
	hotPathIndicators := []string{"Process", "Handle", "Serve", "Parse", "Compute", "Calculate"}
	for _, indicator := range hotPathIndicators {
		if strings.Contains(fn.Name.Name, indicator) {
			return true
		}
	}

	// Check if defer is inside a loop (already handled by DeferAnalyzer)
	// but we check for performance-critical functions here

	return false
}

func (doa *DeferOptimizationAnalyzer) capturesLargeVariables(deferStmt *ast.DeferStmt) bool {
	// Check if defer uses a closure that captures variables
	call := deferStmt.Call
	if _, ok := call.Fun.(*ast.FuncLit); ok {
		// It's a closure, likely captures variables
		return true
	}
	return false
}

func (doa *DeferOptimizationAnalyzer) isUnnecessaryMutexDefer(deferStmt *ast.DeferStmt, body *ast.BlockStmt) bool {
	// Check if it's a mutex unlock
	call := deferStmt.Call
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if sel.Sel.Name == "Unlock" || sel.Sel.Name == "RUnlock" {
			// Check if function has complex control flow
			hasComplexFlow := false
			ast.Inspect(body, func(n ast.Node) bool {
				switch n.(type) {
				case *ast.IfStmt, *ast.SwitchStmt, *ast.ForStmt, *ast.RangeStmt:
					hasComplexFlow = true
					return false
				}
				return true
			})

			return !hasComplexFlow
		}
	}
	return false
}

func (doa *DeferOptimizationAnalyzer) checkMissingDefer(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	if fn.Body == nil {
		return issues
	}

	// Track opened resources and their cleanup
	openedResources := make(map[string]bool, 10)
	hasDefer := make(map[string]bool, 10)

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			// Check for resource opening
			for i, expr := range node.Rhs {
				if call, ok := expr.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						// Check for common resource opening patterns
						if doa.isResourceOpening(sel) && i < len(node.Lhs) {
							if ident, ok := node.Lhs[i].(*ast.Ident); ok {
								openedResources[ident.Name] = true
							}
						}
					}
				}
			}
		case *ast.DeferStmt:
			// Check if defer closes a resource
			call := node.Call
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					hasDefer[ident.Name] = true
				}
			}
		case *ast.CallExpr:
			// Check for Lock without defer Unlock
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "Lock" || sel.Sel.Name == "RLock" {
					if ident, ok := sel.X.(*ast.Ident); ok {
						// Check if there's ANY unlock (defer or direct) for this mutex
						if !hasDefer[ident.Name] && !doa.hasDirectUnlock(fn.Body, ident.Name) {
							pos := fset.Position(node.Pos())
							issues = append(issues, Issue{
								File:       filename,
								Line:       pos.Line,
								Column:     pos.Column,
								Position:   pos,
								Type:       "MISSING_DEFER_UNLOCK",
								Severity:   SeverityHigh,
								Message:    "Mutex locked without defer unlock",
								Suggestion: "Add defer mu.Unlock() immediately after Lock()",
							})
						}
					}
				}
			}
		}
		return true
	})

	// Check for opened resources without defer
	for resource := range openedResources {
		if !hasDefer[resource] {
			issues = append(issues, Issue{
				File:       filename,
				Line:       1,
				Column:     1,
				Position:   token.Position{Filename: filename},
				Type:       "MISSING_DEFER_CLOSE",
				Severity:   SeverityMedium,
				Message:    fmt.Sprintf("Resource opened without defer close: %s", resource),
				Suggestion: fmt.Sprintf("Add defer %s.Close() after opening", resource),
			})
		}
	}

	return issues
}

func (doa *DeferOptimizationAnalyzer) isShortFunction(body *ast.BlockStmt) bool {
	// Function with less than 5 statements is considered short
	return len(body.List) < MaxFunctionParams
}

func (doa *DeferOptimizationAnalyzer) isResourceOpening(sel *ast.SelectorExpr) bool {
	if ident, ok := sel.X.(*ast.Ident); ok {
		openingPatterns := map[string][]string{
			"os":   {"Open", "Create", "OpenFile"},
			"sql":  {"Open", "Begin"},
			"http": {"Get", "Post"},
			"net":  {"Dial", "Listen"},
		}

		if methods, exists := openingPatterns[ident.Name]; exists {
			for _, method := range methods {
				if sel.Sel.Name == method {
					return true
				}
			}
		}
	}
	return false
}

func (doa *DeferOptimizationAnalyzer) hasDirectUnlock(body *ast.BlockStmt, mutexName string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					if ident.Name == mutexName && (sel.Sel.Name == "Unlock" || sel.Sel.Name == "RUnlock") {
						found = true
						return false
					}
				}
			}
		}
		return true
	})
	return found
}
