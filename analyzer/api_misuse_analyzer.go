package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// APIMisuseAnalyzer detects common API misuse patterns
// Constants for common strings
const (
	backgroundFunc = "Background"
	recoverFunc    = "recover"
)

type APIMisuseAnalyzer struct {
	name        string
	loopDepth   int
	inDefer     bool
	inGoroutine bool
}

// NewAPIMisuseAnalyzer creates a new API misuse analyzer
func NewAPIMisuseAnalyzer() Analyzer {
	return &APIMisuseAnalyzer{
		name: "API Misuse",
	}
}

func (a *APIMisuseAnalyzer) Name() string {
	return a.name
}

func (a *APIMisuseAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
	issues := []*Issue{}

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Reset state for each analysis
	a.loopDepth = 0
	a.inDefer = false
	a.inGoroutine = false

	a.walkWithContext(
		astNode, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.DeferStmt:
				// Check for defer in loop
				if a.loopDepth > 0 {
					pos := fset.Position(node.Pos())
					issues = append(
						issues, &Issue{
							File:       pos.Filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueDeferInLoop,
							Severity:   SeverityLevelHigh,
							Message:    "defer in loop - accumulates until function returns",
							Suggestion: "Extract loop body to separate function or handle cleanup differently",
							Code:       getCodeSnippet(node, fset),
						},
					)
				}
			case *ast.CallExpr:
				issues = append(issues, a.checkCallExpr(node, fset)...)
			case *ast.FuncDecl:
				issues = append(issues, a.checkFuncDecl(node, fset)...)
			}
			return true
		},
	)

	return issues
}

func (a *APIMisuseAnalyzer) walkWithContext(node ast.Node, fn func(ast.Node) bool) {
	var walkNode func(ast.Node, int, bool, bool) bool
	walkNode = func(n ast.Node, loopDepth int, inDefer bool, inGoroutine bool) bool {
		// Update analyzer state
		oldLoopDepth := a.loopDepth
		oldInDefer := a.inDefer
		oldInGoroutine := a.inGoroutine

		a.loopDepth = loopDepth
		a.inDefer = inDefer
		a.inGoroutine = inGoroutine

		// Process current node
		if !fn(n) {
			a.loopDepth = oldLoopDepth
			a.inDefer = oldInDefer
			a.inGoroutine = oldInGoroutine
			return false
		}

		// Walk children with updated context
		handled := a.handleSpecialNodes(n, walkNode, loopDepth, inDefer, inGoroutine)
		if handled {
			a.loopDepth = oldLoopDepth
			a.inDefer = oldInDefer
			a.inGoroutine = oldInGoroutine
			return false
		}

		// Restore state
		a.loopDepth = oldLoopDepth
		a.inDefer = oldInDefer
		a.inGoroutine = oldInGoroutine
		return true
	}

	ast.Inspect(
		node, func(n ast.Node) bool {
			return walkNode(n, a.loopDepth, a.inDefer, a.inGoroutine)
		},
	)
}

func (a *APIMisuseAnalyzer) handleSpecialNodes(n ast.Node, walkNode func(ast.Node, int, bool, bool) bool, loopDepth int, inDefer, inGoroutine bool) bool {
	switch stmt := n.(type) {
	case *ast.ForStmt:
		a.walkForStmt(stmt, walkNode, loopDepth, inDefer, inGoroutine)
		return true
	case *ast.RangeStmt:
		a.walkRangeStmt(stmt, walkNode, loopDepth, inDefer, inGoroutine)
		return true
	case *ast.DeferStmt:
		if stmt.Call != nil {
			walkNode(stmt.Call, loopDepth, true, inGoroutine)
		}
		return true
	case *ast.GoStmt:
		if stmt.Call != nil {
			walkNode(stmt.Call, loopDepth, inDefer, true)
		}
		return true
	}
	return false
}

func (a *APIMisuseAnalyzer) walkForStmt(stmt *ast.ForStmt, walkNode func(ast.Node, int, bool, bool) bool, loopDepth int, inDefer, inGoroutine bool) {
	if stmt.Init != nil {
		walkNode(stmt.Init, loopDepth+1, inDefer, inGoroutine)
	}
	if stmt.Cond != nil {
		walkNode(stmt.Cond, loopDepth+1, inDefer, inGoroutine)
	}
	if stmt.Post != nil {
		walkNode(stmt.Post, loopDepth+1, inDefer, inGoroutine)
	}
	if stmt.Body != nil {
		walkNode(stmt.Body, loopDepth+1, inDefer, inGoroutine)
	}
}

func (a *APIMisuseAnalyzer) walkRangeStmt(stmt *ast.RangeStmt, walkNode func(ast.Node, int, bool, bool) bool, loopDepth int, inDefer, inGoroutine bool) {
	if stmt.Key != nil {
		walkNode(stmt.Key, loopDepth+1, inDefer, inGoroutine)
	}
	if stmt.Value != nil {
		walkNode(stmt.Value, loopDepth+1, inDefer, inGoroutine)
	}
	if stmt.X != nil {
		walkNode(stmt.X, loopDepth, inDefer, inGoroutine)
	}
	if stmt.Body != nil {
		walkNode(stmt.Body, loopDepth+1, inDefer, inGoroutine)
	}
}

func (a *APIMisuseAnalyzer) checkCallExpr(call *ast.CallExpr, fset *token.FileSet) []*Issue {
	var issues []*Issue

	// Get function name
	funcName := getFuncName(call)
	if funcName == "" {
		return issues
	}

	pos := fset.Position(call.Pos())

	// Check defer issues
	if issue := a.checkDeferIssues(call, funcName, pos, fset); issue != nil {
		issues = append(issues, issue)
	}

	// Check concurrency issues
	if issue := a.checkConcurrencyIssues(call, funcName, pos, fset); issue != nil {
		issues = append(issues, issue)
	}

	// Check time issues
	if issue := a.checkTimeIssues(call, funcName, pos, fset); issue != nil {
		issues = append(issues, issue)
	}

	// Check formatting issues
	if issue := a.checkFormattingIssues(call, funcName, pos, fset); issue != nil {
		issues = append(issues, issue)
	}

	// Check context issues
	if issue := a.checkContextIssues(call, funcName, pos, fset); issue != nil {
		issues = append(issues, issue)
	}

	// Check logging issues
	if issue := a.checkLoggingIssues(call, funcName, pos, fset); issue != nil {
		issues = append(issues, issue)
	}

	// Check recovery issues
	if issue := a.checkRecoveryIssues(call, funcName, pos, fset); issue != nil {
		issues = append(issues, issue)
	}

	// Check profiling issues
	if issue := a.checkProfilingIssues(call, funcName, pos, fset); issue != nil {
		issues = append(issues, issue)
	}

	// Check marshaling issues
	if issue := a.checkMarshalingIssues(call, funcName, pos, fset); issue != nil {
		issues = append(issues, issue)
	}

	// Check regex issues
	issues = append(issues, a.checkRegexIssues(call, funcName, pos, fset)...)

	return issues
}

func (a *APIMisuseAnalyzer) checkDeferIssues(call *ast.CallExpr, funcName string, pos token.Position, fset *token.FileSet) *Issue {
	if funcName == "defer" && a.loopDepth > 0 {
		return &Issue{
			File:       pos.Filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueDeferInLoop,
			Severity:   SeverityLevelHigh,
			Message:    "defer in loop - accumulates until function returns",
			Suggestion: "Extract loop body to separate function or handle cleanup differently",
			Code:       getCodeSnippet(call, fset),
		}
	}
	return nil
}

func (a *APIMisuseAnalyzer) checkConcurrencyIssues(call *ast.CallExpr, funcName string, pos token.Position, fset *token.FileSet) *Issue {
	if strings.Contains(funcName, "Add") && a.inGoroutine {
		return &Issue{
			File:       pos.Filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueWaitgroupAddInGoroutine,
			Severity:   SeverityLevelHigh,
			Message:    "WaitGroup.Add called inside goroutine - race condition",
			Suggestion: "Call Add before starting the goroutine",
			Code:       getCodeSnippet(call, fset),
		}
	}
	return nil
}

func (a *APIMisuseAnalyzer) checkTimeIssues(call *ast.CallExpr, funcName string, pos token.Position, fset *token.FileSet) *Issue {
	if strings.Contains(funcName, "Sleep") && a.loopDepth > 0 {
		return &Issue{
			File:       pos.Filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueSleepInLoop,
			Severity:   SeverityLevelMedium,
			Message:    "time.Sleep in loop - blocks execution",
			Suggestion: "Use time.Ticker or channels for periodic operations",
			Code:       getCodeSnippet(call, fset),
		}
	}
	if strings.Contains(funcName, "Now") && a.loopDepth > 0 {
		return &Issue{
			File:       pos.Filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueTimeNowInLoop,
			Severity:   SeverityLevelMedium,
			Message:    "Calling time.Now() in loop - expensive syscall",
			Suggestion: "Cache time outside loop or use time.Ticker for periodic timing",
			Code:       getCodeSnippet(call, fset),
		}
	}
	return nil
}

func (a *APIMisuseAnalyzer) checkFormattingIssues(call *ast.CallExpr, funcName string, pos token.Position, fset *token.FileSet) *Issue {
	if funcName == "fmt.Sprintf" && len(call.Args) == 2 {
		if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Value == `"%s%s"` {
			return &Issue{
				File:       pos.Filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueSprintfConcatenation,
				Severity:   SeverityLevelLow,
				Message:    "Using fmt.Sprintf for simple string concatenation",
				Suggestion: "Use + operator or strings.Builder for better performance",
				Code:       getCodeSnippet(call, fset),
			}
		}
	}
	return nil
}

func (a *APIMisuseAnalyzer) checkContextIssues(call *ast.CallExpr, funcName string, pos token.Position, fset *token.FileSet) *Issue {
	if funcName == "context.Background" || funcName == backgroundFunc {
		return &Issue{
			File:       pos.Filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueContextBackgroundMisuse,
			Severity:   SeverityLevelMedium,
			Message:    "Using context.Background() - may be inappropriate for request handlers",
			Suggestion: "Use request context or context with timeout/cancel",
			Code:       getCodeSnippet(call, fset),
		}
	}
	return nil
}

func (a *APIMisuseAnalyzer) checkLoggingIssues(call *ast.CallExpr, funcName string, pos token.Position, fset *token.FileSet) *Issue {
	if a.loopDepth > 0 && (strings.Contains(funcName, "log.") || strings.Contains(funcName, "Printf") || strings.Contains(
		funcName, "Print",
	)) {
		return &Issue{
			File:       pos.Filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueLogInHotPath,
			Severity:   SeverityLevelMedium,
			Message:    "Logging in hot path (loop) - performance impact",
			Suggestion: "Use conditional logging or batch logging",
			Code:       getCodeSnippet(call, fset),
		}
	}
	return nil
}

func (a *APIMisuseAnalyzer) checkRecoveryIssues(call *ast.CallExpr, funcName string, pos token.Position, fset *token.FileSet) *Issue {
	if funcName == recoverFunc && !a.inDefer {
		return &Issue{
			File:       pos.Filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueRecoverWithoutDefer,
			Severity:   SeverityLevelHigh,
			Message:    "recover() called outside defer - won't work",
			Suggestion: "recover() must be called from within a deferred function",
			Code:       getCodeSnippet(call, fset),
		}
	}
	return nil
}

func (a *APIMisuseAnalyzer) checkProfilingIssues(call *ast.CallExpr, funcName string, pos token.Position, fset *token.FileSet) *Issue {
	if strings.Contains(funcName, "StartCPUProfile") {
		if len(call.Args) > 0 {
			if ident, ok := call.Args[0].(*ast.Ident); ok && ident.Name == "nil" {
				return &Issue{
					File:       pos.Filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       IssuePprofNilWriter,
					Severity:   SeverityLevelHigh,
					Message:    "pprof.StartCPUProfile requires io.Writer, not nil",
					Suggestion: "Pass an io.Writer (e.g., os.Create(\"cpu.prof\")) instead of nil",
					Code:       getCodeSnippet(call, fset),
				}
			}
		}
	}
	return nil
}

func (a *APIMisuseAnalyzer) checkMarshalingIssues(call *ast.CallExpr, funcName string, pos token.Position, fset *token.FileSet) *Issue {
	if strings.Contains(funcName, "Marshal") && a.loopDepth > 0 {
		return &Issue{
			File:       pos.Filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueJSONMarshalInLoop,
			Severity:   SeverityLevelHigh,
			Message:    "JSON marshaling in loop - creates garbage",
			Suggestion: "Pre-marshal data or use streaming encoder",
			Code:       getCodeSnippet(call, fset),
		}
	}
	return nil
}

func (a *APIMisuseAnalyzer) checkRegexIssues(call *ast.CallExpr, funcName string, pos token.Position, fset *token.FileSet) []*Issue {
	var issues []*Issue
	if (strings.Contains(funcName, "Compile") || strings.Contains(funcName, "MustCompile")) && strings.Contains(funcName, "regexp") {
		if a.loopDepth > 0 {
			issues = append(
				issues, &Issue{
					File:       pos.Filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       IssueRegexCompileInLoop,
					Severity:   SeverityLevelHigh,
					Message:    "Compiling regex in loop - very expensive",
					Suggestion: "Compile regex once outside loop, use compiled regex",
					Code:       getCodeSnippet(call, fset),
				},
			)
		} else {
			issues = append(
				issues, &Issue{
					File:       pos.Filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       IssueRegexCompileInFunc,
					Severity:   SeverityLevelMedium,
					Message:    "Compiling regex in function - recompiled on each call",
					Suggestion: "Compile regex once at package level",
					Code:       getCodeSnippet(call, fset),
				},
			)
		}
	}
	return issues
}

func (a *APIMisuseAnalyzer) checkFuncDecl(fn *ast.FuncDecl, fset *token.FileSet) []*Issue {
	// Check for mutex passed by value
	if fn.Type == nil || fn.Type.Params == nil {
		return nil
	}

	issues := make([]*Issue, 0, len(fn.Type.Params.List))

	for _, param := range fn.Type.Params.List {
		sel, ok := param.Type.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			continue
		}

		if pkg.Name != "sync" || (sel.Sel.Name != "Mutex" && sel.Sel.Name != "RWMutex") {
			continue
		}

		pos := fset.Position(param.Pos())
		issues = append(
			issues, &Issue{
				File:       pos.Filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueMutexByValue,
				Severity:   SeverityLevelHigh,
				Message:    "Mutex passed by value - won't protect shared state",
				Suggestion: "Pass mutex by pointer (*sync.Mutex)",
				Code:       getCodeSnippet(param, fset),
			},
		)
	}

	return issues
}

func getFuncName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		if pkg, ok := fun.X.(*ast.Ident); ok {
			return pkg.Name + "." + fun.Sel.Name
		}
		return fun.Sel.Name
	}
	return ""
}

func getCodeSnippet(node ast.Node, fset *token.FileSet) string {
	pos := fset.Position(node.Pos())
	end := fset.Position(node.End())

	// This is simplified - would need actual file content
	return "code snippet at " + pos.String() + "-" + end.String()
}
