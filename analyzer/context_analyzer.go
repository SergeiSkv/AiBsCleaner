package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// ContextAnalyzer checks for context.Context misuse
type ContextAnalyzer struct {
	contextParams map[string]bool
}

func NewContextAnalyzer() *ContextAnalyzer {
	return &ContextAnalyzer{
		contextParams: make(map[string]bool),
	}
}

func (ca *ContextAnalyzer) Name() string {
	return "ContextAnalyzer"
}

func (ca *ContextAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Check all function declarations
	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			issues = append(issues, ca.analyzeFuncContext(node, filename, fset)...)
		case *ast.CallExpr:
			issues = append(issues, ca.analyzeContextCall(node, filename, fset)...)
		case *ast.GoStmt:
			issues = append(issues, ca.analyzeGoroutineContext(node, filename, fset)...)
		}
		return true
	})

	return issues
}

func (ca *ContextAnalyzer) analyzeFuncContext(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	// Check context parameter position
	issues = append(issues, ca.checkContextPosition(fn, filename, fset)...)

	// Check for context.Background() or context.TODO() usage
	issues = append(issues, ca.checkContextBackgroundUsage(fn, filename, fset)...)

	return issues
}

func (ca *ContextAnalyzer) checkContextPosition(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
		return issues
	}

	hasContext := false
	contextPosition := -1

	for i, param := range fn.Type.Params.List {
		if ca.isContextType(param.Type) {
			hasContext = true
			contextPosition = i
			break
		}
	}

	// Context should be first parameter if present
	if hasContext && contextPosition > 0 {
		pos := fset.Position(fn.Type.Params.List[contextPosition].Pos())
		issues = append(issues, Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       "CONTEXT_POSITION",
			Severity:   SeverityMedium,
			Message:    "Context should be the first parameter",
			Suggestion: "Move context.Context to be the first parameter",
		})
	}

	// Store function parameters with context for later checks
	if hasContext && fn.Name != nil {
		ca.contextParams[fn.Name.Name] = true
	}

	return issues
}

func (ca *ContextAnalyzer) checkContextBackgroundUsage(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	if fn.Body == nil || fn.Name == nil {
		return issues
	}

	// Check for context.Background() or context.TODO() in production code
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok { //nolint:nestif // AST traversal requires deep nesting
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					if ident.Name == "context" {
						if sel.Sel.Name == "Background" || sel.Sel.Name == "TODO" {
							// Check if not in main or test function
							if fn.Name.Name != "main" && !strings.HasPrefix(fn.Name.Name, "Test") {
								pos := fset.Position(call.Pos())
								issues = append(issues, Issue{
									File:       filename,
									Line:       pos.Line,
									Column:     pos.Column,
									Position:   pos,
									Type:       "CONTEXT_BACKGROUND_MISUSE",
									Severity:   SeverityMedium,
									Message:    "Avoid using context.Background() or context.TODO() in production code",
									Suggestion: "Use context from parent function or request",
								})
							}
						}
					}
				}
			}
		}
		return true
	})

	return issues
}

func (ca *ContextAnalyzer) analyzeContextCall(call *ast.CallExpr, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	// Check for context.WithValue with non-comparable keys
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok { //nolint:nestif // AST analysis requires nested checks
		if ident, ok := sel.X.(*ast.Ident); ok {
			if ident.Name == "context" && sel.Sel.Name == "WithValue" {
				if len(call.Args) >= 2 {
					// Check if key is a string literal (bad practice)
					if lit, ok := call.Args[1].(*ast.BasicLit); ok {
						if lit.Kind == token.STRING {
							pos := fset.Position(lit.Pos())
							issues = append(issues, Issue{
								File:       filename,
								Line:       pos.Line,
								Column:     pos.Column,
								Position:   pos,
								Type:       "CONTEXT_STRING_KEY",
								Severity:   SeverityLow,
								Message:    "Using string literal as context key can cause collisions",
								Suggestion: "Define a custom type for context keys",
							})
						}
					}
				}
			}

			// Check for missing cancel function call
			// TODO: This would need more complex analysis to track if cancel is called
			// For now, we can't easily detect if the result is ignored without parent tracking
		}
	}

	return issues
}

func (ca *ContextAnalyzer) analyzeGoroutineContext(goStmt *ast.GoStmt, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	// Check if goroutine uses parent context
	hasContext := false
	ast.Inspect(goStmt.Call, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if ident.Name == "ctx" || strings.Contains(ident.Name, "context") {
				hasContext = true
				return false
			}
		}
		return true
	})

	if !hasContext {
		// Check if the function being called expects context
		if call, ok := goStmt.Call.Fun.(*ast.Ident); ok {
			if ca.contextParams[call.Name] {
				pos := fset.Position(goStmt.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "GOROUTINE_NO_CONTEXT",
					Severity:   SeverityMedium,
					Message:    "Goroutine started without passing context",
					Suggestion: "Pass parent context to goroutine for proper cancellation",
				})
			}
		}
	}

	return issues
}

func (ca *ContextAnalyzer) isContextType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name == "context" && t.Sel.Name == "Context"
		}
	case *ast.InterfaceType:
		// Check for embedded context.Context
		return false
	}
	return false
}
