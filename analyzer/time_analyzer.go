package analyzer

import (
	"go/ast"
	"go/token"
)

type TimeAnalyzer struct{}

func NewTimeAnalyzer() *TimeAnalyzer {
	return &TimeAnalyzer{}
}

func (ta *TimeAnalyzer) Name() string {
	return "TimeAnalyzer"
}

func (ta *TimeAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if ta.isTimeNowInLoop(node) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "TIME_NOW_IN_LOOP",
					Severity:   SeverityMedium,
					Message:    "time.Now() called repeatedly in loop",
					Suggestion: "Cache time.Now() result outside loop if precision allows",
				})
			}

			if ta.isTimeFormatInLoop(node) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "TIME_FORMAT_IN_LOOP",
					Severity:   SeverityMedium,
					Message:    "Time formatting in loop is expensive",
					Suggestion: "Consider caching formatted time or use more efficient format",
				})
			}
		}
		return true
	})

	return issues
}

func (ta *TimeAnalyzer) isTimeNowInLoop(call *ast.CallExpr) bool {
	if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			if ident.Name == "time" && selExpr.Sel.Name == "Now" {
				return ta.isInLoop(call)
			}
		}
	}
	return false
}

func (ta *TimeAnalyzer) isTimeFormatInLoop(call *ast.CallExpr) bool {
	if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		if selExpr.Sel.Name == "Format" {
			return ta.isInLoop(call)
		}
	}
	return false
}

func (ta *TimeAnalyzer) isInLoop(node ast.Node) bool {
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