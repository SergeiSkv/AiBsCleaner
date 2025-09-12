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

func NewConcurrencyPatternsAnalyzer() Analyzer {
	return &ConcurrencyPatternsAnalyzer{
		waitGroups: make(map[string]*WaitGroupInfo),
		channels:   make(map[string]*ChannelUsage),
	}
}

func (cpa *ConcurrencyPatternsAnalyzer) Name() string {
	return "ConcurrencyPatternsAnalyzer"
}

func (cpa *ConcurrencyPatternsAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
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

	// Reset state
	cpa.waitGroups = make(map[string]*WaitGroupInfo)
	cpa.channels = make(map[string]*ChannelUsage)

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.GoStmt:
				issues = append(issues, cpa.analyzeGoStmt(node, filename, fset)...)
			case *ast.CallExpr:
				issues = append(issues, cpa.analyzeCallExpr(node, n, filename, fset)...)
			case *ast.SelectStmt:
				issues = append(issues, cpa.analyzeSelectStmt(node, filename, fset)...)
			case *ast.AssignStmt:
				issues = append(issues, cpa.analyzeAssignStmt(node, n, filename, fset)...)
			}
			return true
		},
	)

	// Check for deadlock patterns
	issues = append(issues, cpa.checkDeadlockPatterns(filename, fset)...)

	return issues
}

func (cpa *ConcurrencyPatternsAnalyzer) analyzeCallExpr(node *ast.CallExpr, n ast.Node, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	sel, ok := node.Fun.(*ast.SelectorExpr)
	if !ok {
		return issues
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return issues
	}

	// Check WaitGroup patterns
	issues = append(issues, cpa.checkWaitGroupPatterns(node, n, sel, ident, filename, fset)...)

	// Check Mutex patterns
	issues = append(issues, cpa.checkMutexPatterns(n, sel, ident, filename, fset)...)

	// Check Channel patterns
	issues = append(issues, cpa.checkChannelPatterns(node, sel, ident, filename, fset)...)

	return issues
}

func (cpa *ConcurrencyPatternsAnalyzer) checkWaitGroupPatterns(
	node *ast.CallExpr, n ast.Node, sel *ast.SelectorExpr, ident *ast.Ident,
	filename string, fset *token.FileSet,
) []*Issue {
	var issues []*Issue

	if !strings.Contains(ident.Name, "WaitGroup") && !strings.HasSuffix(ident.Name, "wg") {
		return issues
	}

	switch sel.Sel.Name {
	case "Add":
		if cpa.isAddInLoop(n) {
			pos := fset.Position(node.Pos())
			issues = append(
				issues, &Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       IssueWaitGroupAddInLoop,
					Severity:   SeverityLevelMedium,
					Message:    "WaitGroup.Add() called in loop - consider Add(n) before loop",
					Suggestion: "Call wg.Add(count) once before the loop",
				},
			)
		}
	case "Wait":
		if cpa.isWaitBeforeGoroutines(n) {
			pos := fset.Position(node.Pos())
			issues = append(
				issues, &Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       IssueWaitGroupWaitBeforeStart,
					Severity:   SeverityLevelHigh,
					Message:    "WaitGroup.Wait() might be called before all goroutines start",
					Suggestion: "Ensure all goroutines are started before calling Wait()",
				},
			)
		}
	}

	return issues
}

func (cpa *ConcurrencyPatternsAnalyzer) checkMutexPatterns(
	n ast.Node, sel *ast.SelectorExpr, ident *ast.Ident, filename string, fset *token.FileSet,
) []*Issue {
	var issues []*Issue

	if strings.Contains(sel.Sel.Name, "Lock") && !strings.Contains(ident.Name, "RW") {
		if cpa.hasOnlyReads(n) {
			pos := fset.Position(sel.Pos())
			issues = append(
				issues, &Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       IssueMutexForReadOnly,
					Severity:   SeverityLevelMedium,
					Message:    "Using sync.Mutex for read-only operations",
					Suggestion: "Use sync.RWMutex with RLock() for better concurrency",
				},
			)
		}
	}

	return issues
}

func (cpa *ConcurrencyPatternsAnalyzer) checkChannelPatterns(
	node *ast.CallExpr, sel *ast.SelectorExpr, ident *ast.Ident,
	filename string, fset *token.FileSet,
) []*Issue {
	var issues []*Issue

	if sel.Sel.Name != "Send" && sel.Sel.Name != "Recv" {
		return issues
	}

	usage, exists := cpa.channels[ident.Name]
	if !exists {
		return issues
	}

	if !usage.Buffered && usage.SendCount > 10 {
		pos := fset.Position(node.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueUnbufferedChannel,
				Severity:   SeverityLevelMedium,
				Message:    "Unbuffered channel with high traffic causes goroutine blocking",
				Suggestion: "Consider using buffered channel for better throughput",
			},
		)
	}

	return issues
}

func (cpa *ConcurrencyPatternsAnalyzer) analyzeSelectStmt(node *ast.SelectStmt, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	// Check for select with single case
	if len(node.Body.List) == 1 {
		pos := fset.Position(node.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueSelectWithSingleCase,
				Severity:   SeverityLevelLow,
				Message:    "Select with single case is inefficient",
				Suggestion: "Use direct channel operation instead of select",
			},
		)
	}

	// Check for busy wait pattern
	if cpa.isBusyWait(node) {
		pos := fset.Position(node.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueBusyWait,
				Severity:   SeverityLevelHigh,
				Message:    "Busy wait pattern wastes CPU cycles",
				Suggestion: "Add time.Sleep() or use blocking channel operation",
			},
		)
	}

	return issues
}

func (cpa *ConcurrencyPatternsAnalyzer) analyzeAssignStmt(node *ast.AssignStmt, n ast.Node, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	for _, rhs := range node.Rhs {
		call, ok := rhs.(*ast.CallExpr)
		if !ok {
			continue
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			continue
		}

		if ident.Name == pkgContext && sel.Sel.Name == backgroundFunc {
			if cpa.isInGoroutine(n) {
				pos := fset.Position(node.Pos())
				issues = append(
					issues, &Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       IssueContextBackgroundInGoroutine,
						Severity:   SeverityLevelMedium,
						Message:    "Using context.Background() in goroutine prevents cancellation",
						Suggestion: "Pass context from parent or use context.WithCancel",
					},
				)
			}
		}
	}

	return issues
}

func (cpa *ConcurrencyPatternsAnalyzer) hasRecover(call *ast.CallExpr) bool {
	fn, ok := call.Fun.(*ast.FuncLit)
	if !ok {
		return false
	}

	hasRecover := false
	ast.Inspect(
		fn.Body, func(n ast.Node) bool {
			defer_, ok := n.(*ast.DeferStmt)
			if !ok || defer_.Call == nil {
				return true
			}

			ident, ok := defer_.Call.Fun.(*ast.Ident)
			if !ok || ident.Name != "recover" {
				return true
			}

			hasRecover = true
			return false
		},
	)
	return hasRecover
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

func (cpa *ConcurrencyPatternsAnalyzer) checkDeadlockPatterns(filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue
	// Check for circular channel dependencies, mutex ordering issues, etc.
	return issues
}

func (cpa *ConcurrencyPatternsAnalyzer) analyzeGoStmt(node *ast.GoStmt, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	// Check for goroutine without recover
	if !cpa.hasRecover(node.Call) {
		pos := fset.Position(node.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueGoroutineNoRecover,
				Severity:   SeverityLevelMedium,
				Message:    "Goroutine without recover() can crash the program",
				Suggestion: "Add defer recover() at the beginning of goroutine",
			},
		)
	}

	// Check for goroutine with closure capturing loop variable
	if cpa.capturesLoopVar(node.Call) {
		pos := fset.Position(node.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueGoroutineCapturesLoop,
				Severity:   SeverityLevelHigh,
				Message:    "Goroutine captures loop variable by reference",
				Suggestion: "Pass loop variable as parameter or create local copy",
			},
		)
	}

	return issues
}
