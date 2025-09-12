package analyzer

import (
	"go/ast"
	"go/token"
)

type TimeAnalyzer struct{}

func NewTimeAnalyzer() Analyzer {
	return &TimeAnalyzer{}
}

func (ta *TimeAnalyzer) Name() string {
	return "TimeAnalyzer"
}

func (ta *TimeAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
	astNode, ok := node.(ast.Node)
	if !ok {
		return []*Issue{}
	}

	// Get filename from the first position we encounter
	filename := ""
	if astNode.Pos().IsValid() {
		filename = fset.Position(astNode.Pos()).Filename
	}

	var issues []*Issue

	// Use AnalyzerWithContext for proper loop detection
	ctx := NewAnalyzerWithContext(astNode)

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			if !ctx.IsNodeInLoop(call) {
				return true
			}

			if ta.isTimeNow(call) {
				pos := fset.Position(call.Pos())
				issues = append(
					issues, &Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       IssueTimeNowInLoop,
						Severity:   SeverityLevelMedium,
						Message:    "time.Now() called repeatedly in loop",
						Suggestion: "Cache time.Now() result outside loop if precision allows",
					},
				)
			}

			if ta.isTimeFormat(call) {
				pos := fset.Position(call.Pos())
				issues = append(
					issues, &Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       IssueTimeAfterLeak,
						Severity:   SeverityLevelMedium,
						Message:    "Time formatting in loop is expensive",
						Suggestion: "Consider caching formatted time or use more efficient format",
					},
				)
			}

			return true
		},
	)

	return issues
}

func (ta *TimeAnalyzer) isTimeNow(call *ast.CallExpr) bool {
	if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			return ident.Name == "time" && selExpr.Sel.Name == "Now"
		}
	}
	return false
}

func (ta *TimeAnalyzer) isTimeFormat(call *ast.CallExpr) bool {
	if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		return selExpr.Sel.Name == "Format"
	}
	return false
}
