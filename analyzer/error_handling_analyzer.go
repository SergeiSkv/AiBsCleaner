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

	// Check for ignored errors (using _)
	for i, lhs := range assign.Lhs { //nolint:nestif // Error analysis requires nested checks
		if ident, ok := lhs.(*ast.Ident); ok && ident.Name == "_" {
			// Check if corresponding RHS returns an error
			if i < len(assign.Rhs) {
				if call, ok := assign.Rhs[i].(*ast.CallExpr); ok {
					if returnsError(call) {
						pos := fset.Position(assign.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "IGNORED_ERROR",
							Severity:   SeverityHigh,
							Message:    "Error return value is ignored",
							Suggestion: "Check and handle the error appropriately",
						})
						eha.ignoredErrors++
					}
				}
			}
		}
	}

	// Check for error variables not being checked
	hasErrorVar := false
	errorVarName := ""
	for _, lhs := range assign.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok {
			if ident.Name == "err" || strings.HasSuffix(ident.Name, "Err") || strings.HasSuffix(ident.Name, "Error") {
				hasErrorVar = true
				errorVarName = ident.Name
				break
			}
		}
	}

	if hasErrorVar {
		// Check if error is checked in the next statement
		// This is simplified - in real implementation we'd need to track control flow
		if !isErrorCheckedNearby(assign, errorVarName) {
			pos := fset.Position(assign.Pos())
			issues = append(issues, Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       "UNCHECKED_ERROR",
				Severity:   SeverityMedium,
				Message:    "Error variable '" + errorVarName + "' may not be checked",
				Suggestion: "Add error checking immediately after assignment",
			})
		}
	}

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

func (eha *ErrorHandlingAnalyzer) analyzeErrorReturn(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue { //nolint:gocyclo // Error return analysis inherently complex
	var issues []Issue

	// Check if function returns error
	returnsErr := false
	if fn.Type.Results != nil {
		for _, result := range fn.Type.Results.List {
			if ident, ok := result.Type.(*ast.Ident); ok && ident.Name == "error" {
				returnsErr = true
				break
			}
		}
	}

	if returnsErr { //nolint:nestif // Return error analysis requires deep nesting
		// Check for inconsistent error returns
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if ret, ok := n.(*ast.ReturnStmt); ok {
				// Check return values - currently just tracking them
				// TODO: Add logic to check for inconsistent error returns
				for range ret.Results {
					// Placeholder for future implementation
				}
			}
			return true
		})

		// Check for missing nil checks before returning error
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if ret, ok := n.(*ast.ReturnStmt); ok {
				if len(ret.Results) > 0 {
					// Check if returning an error variable without checking it's not nil
					for _, result := range ret.Results {
						if ident, ok := result.(*ast.Ident); ok {
							if isErrorVariable(ident.Name) && !isErrorCheckedBefore(ret, ident.Name) {
								pos := fset.Position(ret.Pos())
								issues = append(issues, Issue{
									File:       filename,
									Line:       pos.Line,
									Column:     pos.Column,
									Position:   pos,
									Type:       "UNCHECKED_ERROR_RETURN",
									Severity:   SeverityLow,
									Message:    "Returning error without nil check",
									Suggestion: "Check if error is nil before returning",
								})
							}
						}
					}
				}
			}
			return true
		})

		// Check for error shadowing
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if assign, ok := n.(*ast.AssignStmt); ok {
				if assign.Tok == token.DEFINE { // := operator
					for _, lhs := range assign.Lhs {
						if ident, ok := lhs.(*ast.Ident); ok {
							if ident.Name == "err" {
								// Check if err is already defined in outer scope
								pos := fset.Position(assign.Pos())
								issues = append(issues, Issue{
									File:       filename,
									Line:       pos.Line,
									Column:     pos.Column,
									Position:   pos,
									Type:       "ERROR_SHADOWING",
									Severity:   SeverityMedium,
									Message:    "Potential error variable shadowing",
									Suggestion: "Use consistent error variable or check for shadowing",
								})
							}
						}
					}
				}
			}
			return true
		})
	}

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
