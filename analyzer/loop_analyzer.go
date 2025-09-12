package analyzer

import (
	"go/ast"
	"go/token"
)

type LoopAnalyzer struct{}

func NewLoopAnalyzer() Analyzer {
	return &LoopAnalyzer{}
}

func (la *LoopAnalyzer) Name() string {
	return "LoopAnalyzer"
}

func (la *LoopAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
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

	// Create context for loop detection
	ctx := NewAnalyzerWithContext(astNode)

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.DeferStmt:
				// Check if defer is inside a loop
				if ctx.IsNodeInLoop(node) {
					pos := fset.Position(node.Pos())
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueDeferInLoop,
							Severity:   SeverityLevelHigh,
							Message:    "defer statement inside loop can cause memory buildup",
							Suggestion: "Move defer outside the loop or avoid using defer in loops",
						},
					)
				}
			case *ast.RangeStmt:
				if la.checkStringRangeLoop(node) {
					pos := fset.Position(node.Pos())
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueStringConcatInLoop,
							Severity:   SeverityLevelMedium,
							Message:    "Ranging over string converts it to []rune which allocates memory",
							Suggestion: "Consider using for i := 0; i < len(str); i++ if you don't need Unicode support",
						},
					)
				}
				// Check for defer in range loop body
				issues = append(issues, la.checkLoopBody(node.Body, filename, fset, ctx)...)
			case *ast.ForStmt:
				if la.checkNestedLoopAllocation(node) {
					pos := fset.Position(node.Pos())
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueStringConcatInLoop,
							Severity:   SeverityLevelHigh,
							Message:    "Memory allocation inside nested loop detected",
							Suggestion: "Pre-allocate memory outside the loop or use object pooling",
						},
					)
				}
				// Check for defer in for loop body
				if node.Body != nil {
					issues = append(issues, la.checkLoopBody(node.Body, filename, fset, ctx)...)
				}
			}
			return true
		},
	)

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

	ast.Inspect(
		stmt.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.ForStmt:
				hasNestedLoop = true
			case *ast.RangeStmt:
				hasNestedLoop = true
			case *ast.CompositeLit:
				hasAllocation = true
			case *ast.CallExpr:
				if ident, ok := node.Fun.(*ast.Ident); ok {
					if ident.Name == funcMake || ident.Name == funcAppend {
						hasAllocation = true
					}
				}
			}
			return true
		},
	)

	return hasNestedLoop && hasAllocation
}

func (la *LoopAnalyzer) checkLoopBody(body *ast.BlockStmt, filename string, fset *token.FileSet, ctx *AnalyzerWithContext) []*Issue {
	var issues []*Issue
	// Additional checks can be added here for loop body
	return issues
}

// }
