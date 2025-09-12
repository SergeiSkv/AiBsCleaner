package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

type DeferOptimizationAnalyzer struct{}

func NewDeferOptimizationAnalyzer() Analyzer {
	return &DeferOptimizationAnalyzer{}
}

func (doa *DeferOptimizationAnalyzer) Name() string {
	return "DeferOptimizationAnalyzer"
}

func (doa *DeferOptimizationAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
	var issues []*Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Get filename from the first position we encounter
	filename := ""
	if astNode.Pos().IsValid() {
		filename = fset.Position(astNode.Pos()).Filename
	}

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				issues = append(issues, doa.analyzeFunction(node, filename, fset)...)
			case *ast.FuncLit:
				issues = append(issues, doa.analyzeFuncLit(node, filename, fset)...)
			}
			return true
		},
	)

	return issues
}

func (doa *DeferOptimizationAnalyzer) analyzeFunction(fn *ast.FuncDecl, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	if fn.Body == nil {
		return issues
	}

	deferStmts := doa.collectDeferStatements(fn.Body)

	// Check for unnecessary defer overhead
	for _, deferStmt := range deferStmts {
		issues = append(issues, doa.checkDeferIssues(deferStmt, fn, filename, fset)...)
	}

	// Check for multiple defers that could be combined
	if doa.hasMultipleSimilarDefers(deferStmts) && len(deferStmts) > 0 {
		pos := fset.Position(deferStmts[0].Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueDeferOverhead,
				Severity:   SeverityLevelLow,
				Message:    "Multiple defer statements add cumulative overhead",
				Suggestion: "Combine multiple defers into a single cleanup function",
			},
		)
	}

	// Check for missing defer where it should be used
	issues = append(issues, doa.checkMissingDefer(fn, filename, fset)...)

	return issues
}

func (doa *DeferOptimizationAnalyzer) checkDeferIssues(
	deferStmt *ast.DeferStmt,
	fn *ast.FuncDecl, filename string, fset *token.FileSet,
) []*Issue {
	var issues []*Issue
	pos := fset.Position(deferStmt.Pos())

	// Check if defer is used for simple operations
	if doa.isSimpleOperation(deferStmt) {
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueDeferOverhead,
				Severity:   SeverityLevelLow,
				Message:    "Unnecessary defer for simple operation adds overhead",
				Suggestion: "Call directly without defer for better performance",
			},
		)
	}

	// Check if defer is at the end of function
	if doa.isDeferAtEnd(deferStmt, fn.Body) {
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueDeferOverhead,
				Severity:   SeverityLevelMedium,
				Message:    "Defer at the end of function is unnecessary",
				Suggestion: "Execute directly without defer",
			},
		)
	}

	// Check for defer in hot path
	if doa.isInHotPath(deferStmt, fn) {
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueDeferOverhead,
				Severity:   SeverityLevelMedium,
				Message:    "Defer in performance-critical path adds overhead (~30ns)",
				Suggestion: "Consider manual cleanup for hot paths",
			},
		)
	}

	// Check for defer with closure capturing large variables
	if doa.capturesLargeVariables(deferStmt) {
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueDeferOverhead,
				Severity:   SeverityLevelMedium,
				Message:    "Defer closure captures large variables increasing memory usage",
				Suggestion: "Pass values as parameters instead of capturing",
			},
		)
	}

	// Check for unnecessary defer for mutex unlock
	if doa.isUnnecessaryMutexDefer(deferStmt, fn.Body) {
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueDeferOverhead,
				Severity:   SeverityLevelLow,
				Message:    "Defer for mutex unlock in simple function adds unnecessary overhead",
				Suggestion: "Unlock directly before return if no complex control flow",
			},
		)
	}

	return issues
}

func (doa *DeferOptimizationAnalyzer) analyzeFuncLit(fn *ast.FuncLit, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	if fn.Body == nil {
		return issues
	}

	// Check for defer in short anonymous functions
	deferStmts := doa.collectDeferStatements(fn.Body)
	if len(deferStmts) > 0 && doa.isShortFunction(fn.Body) {
		pos := fset.Position(deferStmts[0].Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueDeferOverhead,
				Severity:   SeverityLevelLow,
				Message:    "Defer in short function adds unnecessary overhead",
				Suggestion: "Call cleanup directly in short functions",
			},
		)
	}

	return issues
}

func (doa *DeferOptimizationAnalyzer) collectDeferStatements(block *ast.BlockStmt) []*ast.DeferStmt {
	var defers []*ast.DeferStmt

	ast.Inspect(
		block, func(n ast.Node) bool {
			if d, ok := n.(*ast.DeferStmt); ok {
				defers = append(defers, d)
			}
			return true
		},
	)

	return defers
}

func (doa *DeferOptimizationAnalyzer) isSimpleOperation(deferStmt *ast.DeferStmt) bool {
	// Check if it's a simple function call without complex arguments
	call := deferStmt.Call
	// Simple operations that don't need defer
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		simpleOps := []string{methodClose, methodUnlock, "Done", "Release"}
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
				case methodClose:
					closeCount++
				case methodUnlock, methodRUnlock:
					unlockCount++
				}
			}
		}

		return closeCount >= MaxNestedLoops || unlockCount >= MaxNestedLoops
	}

	return false
}

func (doa *DeferOptimizationAnalyzer) isInHotPath(_ *ast.DeferStmt, fn *ast.FuncDecl) bool {
	// Check if function name indicates hot path
	hotPathIndicators := []string{"Process", "Handle", "Serve", "Parse", "Compute", "Calculate"}
	for _, indicator := range hotPathIndicators {
		if strings.Contains(fn.Name.Name, indicator) {
			return true
		}
	}

	// Check if defer is inside a loop (already handled by DeferAnalyzer)

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
		if sel.Sel.Name == methodUnlock || sel.Sel.Name == methodRUnlock {
			// Check if function has complex control flow
			hasComplexFlow := false
			ast.Inspect(
				body, func(n ast.Node) bool {
					switch n.(type) {
					case *ast.IfStmt, *ast.SwitchStmt, *ast.ForStmt, *ast.RangeStmt:
						hasComplexFlow = true
						return false
					}
					return true
				},
			)

			return !hasComplexFlow
		}
	}
	return false
}

func (doa *DeferOptimizationAnalyzer) checkMissingDefer(fn *ast.FuncDecl, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	if fn.Body == nil {
		return issues
	}

	// Track opened resources and their cleanup
	openedResources := make(map[string]bool, 10)
	hasDefer := make(map[string]bool, 10)

	ast.Inspect(
		fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				doa.checkAssignStmt(node, openedResources)
			case *ast.DeferStmt:
				doa.checkDeferStmt(node, hasDefer)
			case *ast.CallExpr:
				lockIssue := doa.checkLockCall(node, hasDefer, fn.Body, filename, fset)
				if lockIssue != nil {
					issues = append(issues, lockIssue)
				}
			}
			return true
		},
	)

	// Check for opened resources without defer
	resourceIssues := doa.checkUnclosedResources(openedResources, hasDefer, filename)
	issues = append(issues, resourceIssues...)

	return issues
}

func (doa *DeferOptimizationAnalyzer) checkAssignStmt(node *ast.AssignStmt, openedResources map[string]bool) {
	for i, expr := range node.Rhs {
		call, ok := expr.(*ast.CallExpr)
		if !ok {
			continue
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !doa.isResourceOpening(sel) || i >= len(node.Lhs) {
			continue
		}

		if ident, ok := node.Lhs[i].(*ast.Ident); ok {
			openedResources[ident.Name] = true
		}
	}
}

func (doa *DeferOptimizationAnalyzer) checkDeferStmt(node *ast.DeferStmt, hasDefer map[string]bool) {
	sel, ok := node.Call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	if ident, ok := sel.X.(*ast.Ident); ok {
		hasDefer[ident.Name] = true
	}
}

func (doa *DeferOptimizationAnalyzer) checkLockCall(
	node *ast.CallExpr, hasDefer map[string]bool, body *ast.BlockStmt, filename string, fset *token.FileSet,
) *Issue {
	sel, ok := node.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	if sel.Sel.Name != methodLock && sel.Sel.Name != methodRLock {
		return nil
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil
	}

	if hasDefer[ident.Name] || doa.hasDirectUnlock(body, ident.Name) {
		return nil
	}

	pos := fset.Position(node.Pos())
	return &Issue{
		File:       filename,
		Line:       pos.Line,
		Column:     pos.Column,
		Position:   pos,
		Type:       IssueDeferOverhead,
		Severity:   SeverityLevelHigh,
		Message:    "Mutex locked without defer unlock",
		Suggestion: "Add defer mu.Unlock() immediately after Lock()",
	}
}

func (doa *DeferOptimizationAnalyzer) checkUnclosedResources(openedResources, hasDefer map[string]bool, filename string) []*Issue {
	var issues []*Issue

	for resource := range openedResources {
		if !hasDefer[resource] {
			issues = append(
				issues, &Issue{
					File:       filename,
					Line:       1,
					Column:     1,
					Position:   token.Position{Filename: filename},
					Type:       IssueDeferOverhead,
					Severity:   SeverityLevelMedium,
					Message:    fmt.Sprintf("Resource opened without defer close: %s", resource),
					Suggestion: fmt.Sprintf("Add defer %s.Close() after opening", resource),
				},
			)
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
	ast.Inspect(
		body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}

			if ident.Name == mutexName && (sel.Sel.Name == methodUnlock || sel.Sel.Name == methodRUnlock) {
				found = true
				return false
			}
			return true
		},
	)
	return found
}
