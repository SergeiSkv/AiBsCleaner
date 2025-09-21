package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// ErrorHandlingAnalyzer checks for proper error handling patterns
type ErrorHandlingAnalyzer struct {
	ignoredErrors int
	wrappedErrors int
}

func NewErrorHandlingAnalyzer() *ErrorHandlingAnalyzer {
	return &ErrorHandlingAnalyzer{}
}

func (eha *ErrorHandlingAnalyzer) Name() string {
	return "ErrorHandlingAnalyzer"
}

func (eha *ErrorHandlingAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Reset counters
	eha.ignoredErrors = 0
	eha.wrappedErrors = 0

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			issues = append(issues, eha.analyzeErrorAssignment(node, filename, fset)...)
		case *ast.IfStmt:
			issues = append(issues, eha.analyzeErrorCheck(node, filename, fset)...)
		case *ast.CallExpr:
			issues = append(issues, eha.analyzeErrorWrapping(node, filename, fset)...)
		case *ast.FuncDecl:
			issues = append(issues, eha.analyzeErrorReturn(node, filename, fset)...)
		}
		return true
	})

	return issues
}

func (eha *ErrorHandlingAnalyzer) analyzeErrorAssignment(assign *ast.AssignStmt, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	// Check for ignored errors (using _) - but only for critical operations
	for _, lhs := range assign.Lhs { //nolint:nestif // Error analysis requires nested checks
		if ident, ok := lhs.(*ast.Ident); ok && ident.Name == "_" {
			// For multi-return functions like os.Open which return (file, error),
			// there's only one RHS expression but multiple LHS variables
			// Check all RHS expressions for critical operations
			for _, rhs := range assign.Rhs {
				if call, ok := rhs.(*ast.CallExpr); ok {
					if returnsError(call) && isCriticalOperation(call) {
						pos := fset.Position(assign.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "IGNORED_ERROR",
							Severity:   SeverityMedium,
							Message:    "Critical error return value is ignored",
							Suggestion: "Check and handle the error appropriately",
						})
						eha.ignoredErrors++
						break // Only report once per assignment
					}
				}
			}
		}
	}

	// Reduce false positives for UNCHECKED_ERROR - skip it for now
	// Library code often has different error handling patterns

	return issues
}

func (eha *ErrorHandlingAnalyzer) analyzeErrorCheck(ifStmt *ast.IfStmt, filename string, fset *token.FileSet) []Issue { //nolint:gocyclo // Error checking analysis inherently complex
	var issues []Issue

	// Check for error comparisons
	if binExpr, ok := ifStmt.Cond.(*ast.BinaryExpr); ok { //nolint:nestif // Error condition analysis requires deep nesting
		if binExpr.Op == token.NEQ || binExpr.Op == token.EQL {
			var errorVar *ast.Ident
			if ident, ok := binExpr.X.(*ast.Ident); ok && isErrorVariable(ident.Name) {
				errorVar = ident
			} else if ident, ok := binExpr.Y.(*ast.Ident); ok && isErrorVariable(ident.Name) {
				errorVar = ident
			}

			if errorVar != nil {
				// Check if error is properly handled in the if block
				if ifStmt.Body != nil && len(ifStmt.Body.List) == 0 {
					pos := fset.Position(ifStmt.Pos())
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "EMPTY_ERROR_HANDLER",
						Severity:   SeverityMedium,
						Message:    "Empty error handling block",
						Suggestion: "Add proper error handling or logging",
					})
				}

				// Check for panic in error handling
				if ifStmt.Body != nil {
					for _, stmt := range ifStmt.Body.List {
						if exprStmt, ok := stmt.(*ast.ExprStmt); ok {
							if call, ok := exprStmt.X.(*ast.CallExpr); ok {
								if ident, ok := call.Fun.(*ast.Ident); ok {
									if ident.Name == "panic" {
										pos := fset.Position(call.Pos())
										issues = append(issues, Issue{
											File:       filename,
											Line:       pos.Line,
											Column:     pos.Column,
											Position:   pos,
											Type:       "PANIC_ON_ERROR",
											Severity:   SeverityMedium,
											Message:    "Using panic for error handling",
											Suggestion: "Return error to caller instead of panicking",
										})
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return issues
}

func (eha *ErrorHandlingAnalyzer) analyzeErrorWrapping(call *ast.CallExpr, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	// Check for fmt.Errorf usage
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok { //nolint:nestif // Error wrapping analysis requires nested checks
		if ident, ok := sel.X.(*ast.Ident); ok {
			if ident.Name == "fmt" && sel.Sel.Name == "Errorf" {
				// Check if using %w verb for wrapping
				if len(call.Args) > 0 {
					if lit, ok := call.Args[0].(*ast.BasicLit); ok {
						if lit.Kind == token.STRING {
							format := lit.Value
							if !strings.Contains(format, "%w") && hasErrorInArgs(call.Args[1:]) {
								pos := fset.Position(call.Pos())
								issues = append(issues, Issue{
									File:       filename,
									Line:       pos.Line,
									Column:     pos.Column,
									Position:   pos,
									Type:       "ERROR_NOT_WRAPPED",
									Severity:   SeverityLow,
									Message:    "Error not wrapped with %w verb",
									Suggestion: "Use %w to wrap errors for better stack traces",
								})
							} else if strings.Contains(format, "%w") {
								eha.wrappedErrors++
							}
						}
					}
				}
			}

			// Check for errors.New vs fmt.Errorf
			if ident.Name == "errors" && sel.Sel.Name == "New" {
				// Check if the error message could be dynamic
				if len(call.Args) > 0 {
					if _, ok := call.Args[0].(*ast.BinaryExpr); ok {
						pos := fset.Position(call.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "ERRORS_NEW_WITH_FORMAT",
							Severity:   SeverityLow,
							Message:    "Using errors.New with string concatenation",
							Suggestion: "Use fmt.Errorf for formatted error messages",
						})
					}
				}
			}
		}
	}

	return issues
}

func (eha *ErrorHandlingAnalyzer) analyzeErrorReturn(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	// Skip optimizations - these patterns are too common in library code
	// Focus only on clear antipatterns

	return issues
}

// Helper functions
func returnsError(call *ast.CallExpr) bool {
	// Simplified check - in reality would need type information
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		methodName := sel.Sel.Name
		// Common methods that return errors
		errorMethods := []string{"Open", "Close", "Read", "Write", "Query", "Exec",
			"Get", "Post", "Do", "Dial", "Listen", "Accept", "Send", "Receive",
			"Marshal", "Unmarshal", "Decode", "Encode"}
		for _, method := range errorMethods {
			if strings.HasPrefix(methodName, method) {
				return true
			}
		}
	}
	return false
}

// isCriticalOperation checks if the operation is critical enough to require error checking
func isCriticalOperation(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		methodName := sel.Sel.Name
		// Only flag critical operations where ignoring errors is dangerous
		criticalMethods := []string{"Open", "Close", "Write", "Exec", "Dial", "Listen"}
		for _, method := range criticalMethods {
			if strings.HasPrefix(methodName, method) {
				return true
			}
		}
	}
	return false
}

func isErrorVariable(name string) bool {
	return name == "err" || strings.HasSuffix(name, "Err") || strings.HasSuffix(name, "Error")
}

func isErrorCheckedNearby(assign *ast.AssignStmt, errorVar string) bool {
	// Simplified - would need to analyze control flow
	return false
}

func isErrorCheckedBefore(ret *ast.ReturnStmt, errorVar string) bool {
	// Simplified - would need to analyze control flow
	return false
}

func hasErrorInArgs(args []ast.Expr) bool {
	for _, arg := range args {
		if ident, ok := arg.(*ast.Ident); ok {
			if isErrorVariable(ident.Name) {
				return true
			}
		}
	}
	return false
}
