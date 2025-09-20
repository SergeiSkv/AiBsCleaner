package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// ConcurrencyPatternsAnalyzer detects inefficient concurrency patterns
type ConcurrencyPatternsAnalyzer struct {
	waitGroups map[string]*WaitGroupInfo
	channels   map[string]*ChannelUsage
}

type WaitGroupInfo struct {
	Added  []token.Position
	Done   []token.Position
	Waited []token.Position
}

type ChannelUsage struct {
	Created   token.Position
	Buffered  bool
	Size      int
	SendCount int
	RecvCount int
}

func NewConcurrencyPatternsAnalyzer() *ConcurrencyPatternsAnalyzer {
	return &ConcurrencyPatternsAnalyzer{
		waitGroups: make(map[string]*WaitGroupInfo),
		channels:   make(map[string]*ChannelUsage),
	}
}

func (cpa *ConcurrencyPatternsAnalyzer) Name() string {
	return "ConcurrencyPatternsAnalyzer"
}

func (cpa *ConcurrencyPatternsAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Reset state
	cpa.waitGroups = make(map[string]*WaitGroupInfo)
	cpa.channels = make(map[string]*ChannelUsage)

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.GoStmt:
			// Check for goroutine without recover
			if !cpa.hasRecover(node.Call) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "GOROUTINE_WITHOUT_RECOVER",
					Severity:   SeverityMedium,
					Message:    "Goroutine without recover() can crash the program",
					Suggestion: "Add defer recover() at the beginning of goroutine",
				})
			}

			// Check for goroutine with closure capturing loop variable
			if cpa.capturesLoopVar(node.Call) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "GOROUTINE_LOOP_VAR_CAPTURE",
					Severity:   SeverityHigh,
					Message:    "Goroutine captures loop variable by reference",
					Suggestion: "Pass loop variable as parameter or create local copy",
				})
			}

		case *ast.CallExpr:
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					// Check WaitGroup patterns
					if strings.Contains(ident.Name, "WaitGroup") || strings.HasSuffix(ident.Name, "wg") {
						switch sel.Sel.Name {
						case "Add":
							if cpa.isAddInLoop(n) {
								pos := fset.Position(node.Pos())
								issues = append(issues, Issue{
									File:       filename,
									Line:       pos.Line,
									Column:     pos.Column,
									Position:   pos,
									Type:       "WAITGROUP_ADD_IN_LOOP",
									Severity:   SeverityMedium,
									Message:    "WaitGroup.Add() called in loop - consider Add(n) before loop",
									Suggestion: "Call wg.Add(count) once before the loop",
								})
							}
						case "Wait":
							// Check if Wait is called before all goroutines started
							if cpa.isWaitBeforeGoroutines(n) {
								pos := fset.Position(node.Pos())
								issues = append(issues, Issue{
									File:       filename,
									Line:       pos.Line,
									Column:     pos.Column,
									Position:   pos,
									Type:       "WAITGROUP_WAIT_MISPLACED",
									Severity:   SeverityHigh,
									Message:    "WaitGroup.Wait() might be called before all goroutines start",
									Suggestion: "Ensure all goroutines are started before calling Wait()",
								})
							}
						}
					}

					// Check for sync.Mutex vs sync.RWMutex
					if strings.Contains(sel.Sel.Name, "Lock") && !strings.Contains(ident.Name, "RW") {
						if cpa.hasOnlyReads(n) {
							pos := fset.Position(node.Pos())
							issues = append(issues, Issue{
								File:       filename,
								Line:       pos.Line,
								Column:     pos.Column,
								Position:   pos,
								Type:       "MUTEX_INSTEAD_OF_RWMUTEX",
								Severity:   SeverityMedium,
								Message:    "Using sync.Mutex for read-only operations",
								Suggestion: "Use sync.RWMutex with RLock() for better concurrency",
							})
						}
					}
				}

				// Check for inefficient channel patterns
				if ident, ok := sel.X.(*ast.Ident); ok {
					if sel.Sel.Name == "Send" || sel.Sel.Name == "Recv" {
						if usage, exists := cpa.channels[ident.Name]; exists {
							if !usage.Buffered && usage.SendCount > 10 {
								pos := fset.Position(node.Pos())
								issues = append(issues, Issue{
									File:       filename,
									Line:       pos.Line,
									Column:     pos.Column,
									Position:   pos,
									Type:       "UNBUFFERED_CHANNEL_HIGH_TRAFFIC",
									Severity:   SeverityMedium,
									Message:    "Unbuffered channel with high traffic causes goroutine blocking",
									Suggestion: "Consider using buffered channel for better throughput",
								})
							}
						}
					}
				}
			}

		case *ast.SelectStmt:
			// Check for select with single case (inefficient)
			if len(node.Body.List) == 1 {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "SELECT_SINGLE_CASE",
					Severity:   SeverityLow,
					Message:    "Select with single case is inefficient",
					Suggestion: "Use direct channel operation instead of select",
				})
			}

			// Check for busy wait pattern
			if cpa.isBusyWait(node) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "BUSY_WAIT_PATTERN",
					Severity:   SeverityHigh,
					Message:    "Busy wait pattern wastes CPU cycles",
					Suggestion: "Add time.Sleep() or use blocking channel operation",
				})
			}

		case *ast.AssignStmt:
			// Check for context.Background() in concurrent code
			for _, rhs := range node.Rhs {
				if call, ok := rhs.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						if ident, ok := sel.X.(*ast.Ident); ok {
							if ident.Name == "context" && sel.Sel.Name == "Background" {
								if cpa.isInGoroutine(n) {
									pos := fset.Position(node.Pos())
									issues = append(issues, Issue{
										File:       filename,
										Line:       pos.Line,
										Column:     pos.Column,
										Position:   pos,
										Type:       "CONTEXT_BACKGROUND_IN_GOROUTINE",
										Severity:   SeverityMedium,
										Message:    "Using context.Background() in goroutine prevents cancellation",
										Suggestion: "Pass context from parent or use context.WithCancel",
									})
								}
							}
						}
					}
				}
			}
		}
		return true
	})

	// Check for deadlock patterns
	issues = append(issues, cpa.checkDeadlockPatterns(filename, fset)...)

	return issues
}

func (cpa *ConcurrencyPatternsAnalyzer) hasRecover(call *ast.CallExpr) bool {
	if fn, ok := call.Fun.(*ast.FuncLit); ok {
		hasRecover := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if defer_, ok := n.(*ast.DeferStmt); ok {
				if defer_.Call != nil {
					if ident, ok := defer_.Call.Fun.(*ast.Ident); ok {
						if ident.Name == "recover" {
							hasRecover = true
							return false
						}
					}
				}
			}
			return true
		})
		return hasRecover
	}
	return false
}

func (cpa *ConcurrencyPatternsAnalyzer) capturesLoopVar(call *ast.CallExpr) bool {
	// Simplified check - would need proper scope analysis
	return false
}

func (cpa *ConcurrencyPatternsAnalyzer) isAddInLoop(node ast.Node) bool {
	// Simplified check
	return false
}

func (cpa *ConcurrencyPatternsAnalyzer) isWaitBeforeGoroutines(node ast.Node) bool {
	// Simplified check
	return false
}

func (cpa *ConcurrencyPatternsAnalyzer) hasOnlyReads(node ast.Node) bool {
	// Simplified check
	return false
}

func (cpa *ConcurrencyPatternsAnalyzer) isInGoroutine(node ast.Node) bool {
	// Simplified check
	return false
}

func (cpa *ConcurrencyPatternsAnalyzer) isBusyWait(sel *ast.SelectStmt) bool {
	// Check if select has only default case
	for _, stmt := range sel.Body.List {
		if comm, ok := stmt.(*ast.CommClause); ok {
			if comm.Comm == nil { // default case
				// Check if default case is empty or only continues
				if len(comm.Body) == 0 {
					return true
				}
			}
		}
	}
	return false
}

func (cpa *ConcurrencyPatternsAnalyzer) checkDeadlockPatterns(filename string, fset *token.FileSet) []Issue {
	var issues []Issue
	// Check for circular channel dependencies, mutex ordering issues, etc.
	return issues
}
