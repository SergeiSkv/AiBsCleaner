package analyzer

import (
	"go/ast"
	"go/token"
)

type GoroutineAnalyzer struct{}

func NewGoroutineAnalyzer() *GoroutineAnalyzer {
	return &GoroutineAnalyzer{}
}

func (ga *GoroutineAnalyzer) Name() string {
	return "GoroutineAnalyzer"
}

func (ga *GoroutineAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.GoStmt:
			if ga.isGoroutineInLoop(n) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "UNBOUNDED_GOROUTINE",
					Severity:   SeverityHigh,
					Message:    "Creating goroutines in loop without limit can cause resource exhaustion",
					Suggestion: "Use worker pool pattern or semaphore to limit concurrent goroutines",
				})
			}

			if ga.capturesLoopVariable(node) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "GOROUTINE_LOOP_VARIABLE",
					Severity:   SeverityHigh,
					Message:    "Goroutine may capture loop variable by reference",
					Suggestion: "Pass loop variable as parameter or create local copy",
				})
			}
		case *ast.CallExpr:
			if ga.isUnbufferedChannel(node) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "UNBUFFERED_CHANNEL",
					Severity:   SeverityMedium,
					Message:    "Unbuffered channel may cause goroutine blocking",
					Suggestion: "Consider using buffered channel: make(chan T, buffer)",
				})
			}
		}
		return true
	})

	return issues
}

func (ga *GoroutineAnalyzer) isGoroutineInLoop(stmt ast.Node) bool {
	parent := stmt
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

func (ga *GoroutineAnalyzer) capturesLoopVariable(stmt *ast.GoStmt) bool {
	inLoop := ga.isGoroutineInLoop(stmt)
	if !inLoop {
		return false
	}

	hasLoopVarReference := false
	ast.Inspect(stmt.Call, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if ident.Name == "i" || ident.Name == "j" || ident.Name == "k" {
				hasLoopVarReference = true
			}
		}
		return true
	})

	return hasLoopVarReference
}

func (ga *GoroutineAnalyzer) isUnbufferedChannel(call *ast.CallExpr) bool {
	if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "make" {
		if len(call.Args) >= 1 {
			if _, ok := call.Args[0].(*ast.ChanType); ok {
				return len(call.Args) == 1
			}
		}
	}
	return false
}