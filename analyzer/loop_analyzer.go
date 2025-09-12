package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
)

type LoopAnalyzer struct{}

func NewLoopAnalyzer() *LoopAnalyzer {
	return &LoopAnalyzer{}
}

func (la *LoopAnalyzer) Name() string {
	return "LoopAnalyzer"
}

func (la *LoopAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.RangeStmt:
			if la.checkStringRangeLoop(node) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "INEFFICIENT_RANGE",
					Severity:   SeverityMedium,
					Message:    "Ranging over string converts it to []rune which allocates memory",
					Suggestion: "Consider using for i := 0; i < len(str); i++ if you don't need Unicode support",
				})
			}
		case *ast.ForStmt:
			if la.checkNestedLoopAllocation(node) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "NESTED_LOOP_ALLOCATION",
					Severity:   SeverityHigh,
					Message:    "Memory allocation inside nested loop detected",
					Suggestion: "Pre-allocate memory outside the loop or use object pooling",
				})
			}
		}
		return true
	})

	return issues
}

func (la *LoopAnalyzer) checkStringRangeLoop(stmt *ast.RangeStmt) bool {
	// Only flag literal strings, not variables (we don't have type info)
	if basicLit, ok := stmt.X.(*ast.BasicLit); ok {
		return basicLit.Kind == token.STRING
	}
	// Don't flag variables without type information to avoid false positives
	return false
}

func (la *LoopAnalyzer) checkNestedLoopAllocation(stmt *ast.ForStmt) bool {
	hasNestedLoop := false
	hasAllocation := false

	ast.Inspect(stmt.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ForStmt:
			hasNestedLoop = true
		case *ast.RangeStmt:
			hasNestedLoop = true
		case *ast.CompositeLit:
			hasAllocation = true
		case *ast.CallExpr:
			if ident, ok := node.Fun.(*ast.Ident); ok {
				if ident.Name == "make" || ident.Name == "append" {
					hasAllocation = true
				}
			}
		}
		return true
	})

	return hasNestedLoop && hasAllocation
}

func (la *LoopAnalyzer) getLineContent(fset *token.FileSet, pos token.Pos) string {
	position := fset.Position(pos)
	return fmt.Sprintf("Line %d", position.Line)
}
