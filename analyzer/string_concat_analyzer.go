package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

type StringConcatAnalyzer struct{}

func NewStringConcatAnalyzer() *StringConcatAnalyzer {
	return &StringConcatAnalyzer{}
}

func (sca *StringConcatAnalyzer) Name() string {
	return "StringConcatAnalyzer"
}

func (sca *StringConcatAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ForStmt:
			if node.Body != nil && sca.hasStringConcatenation(node.Body) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "STRING_CONCAT_IN_LOOP",
					Severity:   SeverityHigh,
					Message:    "String concatenation in loop creates new string on each iteration",
					Suggestion: "Use strings.Builder or bytes.Buffer for efficient string concatenation",
				})
			}
		case *ast.RangeStmt:
			if node.Body != nil && sca.hasStringConcatenation(node.Body) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "STRING_CONCAT_IN_LOOP",
					Severity:   SeverityHigh,
					Message:    "String concatenation in loop creates new string on each iteration",
					Suggestion: "Use strings.Builder or bytes.Buffer for efficient string concatenation",
				})
			}
		}
		return true
	})

	return issues
}

func (sca *StringConcatAnalyzer) hasStringConcatenation(block *ast.BlockStmt) bool {
	hasConcatenation := false

	ast.Inspect(block, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			// Check for += operations (compound assignment)
			if node.Tok == token.ADD_ASSIGN {
				// Check if the left side looks like a string variable
				for _, lhs := range node.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok {
						// Common string variable names
						name := strings.ToLower(ident.Name)
						if name == "result" || name == "output" || name == "buf" ||
							strings.Contains(name, "str") || strings.Contains(name, "msg") ||
							strings.Contains(name, "text") || strings.Contains(name, "path") ||
							strings.Contains(name, "url") || strings.Contains(name, "content") {
							hasConcatenation = true
						}
					}
				}
			}
			// Also check for regular + operations
			for _, expr := range node.Rhs {
				if binExpr, ok := expr.(*ast.BinaryExpr); ok {
					if binExpr.Op == token.ADD {
						if sca.isStringType(binExpr.X) || sca.isStringType(binExpr.Y) {
							hasConcatenation = true
						}
					}
				}
			}
		}
		return true
	})

	return hasConcatenation
}

func (sca *StringConcatAnalyzer) isStringType(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Kind == token.STRING
	case *ast.Ident:
		// Only consider identifiers with names that suggest strings
		// This is a heuristic since we don't have full type information
		name := e.Name
		return strings.Contains(strings.ToLower(name), "str") ||
			strings.Contains(strings.ToLower(name), "msg") ||
			strings.Contains(strings.ToLower(name), "text") ||
			strings.Contains(strings.ToLower(name), "message") ||
			strings.Contains(strings.ToLower(name), "path") ||
			strings.Contains(strings.ToLower(name), "url")
	case *ast.BinaryExpr:
		return e.Op == token.ADD && (sca.isStringType(e.X) || sca.isStringType(e.Y))
	}
	return false
}
