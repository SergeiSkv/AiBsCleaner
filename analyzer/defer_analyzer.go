package analyzer

import (
	"go/ast"
	"go/token"
)

type DeferAnalyzer struct{}

func NewDeferAnalyzer() *DeferAnalyzer {
	return &DeferAnalyzer{}
}

func (da *DeferAnalyzer) Name() string {
	return "DeferAnalyzer"
}

func (da *DeferAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ForStmt:
			if node.Body != nil && da.hasDeferInLoop(node.Body) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "DEFER_IN_LOOP",
					Severity:   SeverityHigh,
					Message:    "defer statement in loop will only execute when function returns, may cause resource leak",
					Suggestion: "Extract loop body to a separate function or handle cleanup manually",
				})
			}
		case *ast.RangeStmt:
			if node.Body != nil && da.hasDeferInLoop(node.Body) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "DEFER_IN_LOOP",
					Severity:   SeverityHigh,
					Message:    "defer statement in loop will only execute when function returns, may cause resource leak",
					Suggestion: "Extract loop body to a separate function or handle cleanup manually",
				})
			}
		}
		return true
	})

	return issues
}

func (da *DeferAnalyzer) hasDeferInLoop(block *ast.BlockStmt) bool {
	hasDefer := false

	ast.Inspect(block, func(n ast.Node) bool {
		if _, ok := n.(*ast.DeferStmt); ok {
			hasDefer = true
			return false
		}
		return true
	})

	return hasDefer
}
