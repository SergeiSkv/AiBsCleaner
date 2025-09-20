package analyzer

import (
	"go/ast"
	"go/token"
)

// HTTPClientAnalyzer checks for HTTP client best practices
type HTTPClientAnalyzer struct {
	clients map[string]*ClientInfo
}

type ClientInfo struct {
	Name       string
	HasTimeout bool
	Position   token.Position
}

func NewHTTPClientAnalyzer() *HTTPClientAnalyzer {
	return &HTTPClientAnalyzer{
		clients: make(map[string]*ClientInfo),
	}
}

func (hca *HTTPClientAnalyzer) Name() string {
	return "HTTPClientAnalyzer"
}

func (hca *HTTPClientAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Reset state
	hca.clients = make(map[string]*ClientInfo)

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			issues = append(issues, hca.analyzeClientCreation(node, filename, fset)...)
		case *ast.CallExpr:
			issues = append(issues, hca.analyzeHTTPCall(node, filename, fset)...)
		case *ast.CompositeLit:
			issues = append(issues, hca.analyzeClientLiteral(node, filename, fset)...)
		}
		return true
	})

	return issues
}

func (hca *HTTPClientAnalyzer) analyzeClientCreation(assign *ast.AssignStmt, filename string, fset *token.FileSet) []Issue { //nolint:gocyclo // HTTP client analysis inherently complex
	var issues []Issue

	for i, rhs := range assign.Rhs { //nolint:nestif // HTTP client analysis requires nested checks
		// Check for &http.Client{}
		if unary, ok := rhs.(*ast.UnaryExpr); ok {
			if comp, ok := unary.X.(*ast.CompositeLit); ok {
				if sel, ok := comp.Type.(*ast.SelectorExpr); ok {
					if ident, ok := sel.X.(*ast.Ident); ok {
						if ident.Name == "http" && sel.Sel.Name == "Client" {
							hasTimeout := false

							// Check if Timeout is set
							for _, elt := range comp.Elts {
								if kv, ok := elt.(*ast.KeyValueExpr); ok {
									if key, ok := kv.Key.(*ast.Ident); ok {
										if key.Name == "Timeout" {
											hasTimeout = true
										}
										if key.Name == "Transport" {
											// Check transport configuration
											issues = append(issues, hca.analyzeTransport(kv.Value, filename, fset)...)
										}
									}
								}
							}

							if !hasTimeout {
								pos := fset.Position(comp.Pos())
								issues = append(issues, Issue{
									File:       filename,
									Line:       pos.Line,
									Column:     pos.Column,
									Position:   pos,
									Type:       "HTTP_CLIENT_NO_TIMEOUT",
									Severity:   SeverityHigh,
									Message:    "HTTP client created without timeout",
									Suggestion: "Set Timeout field to prevent hanging requests",
								})
							}

							// Store client info
							if i < len(assign.Lhs) {
								if ident, ok := assign.Lhs[i].(*ast.Ident); ok {
									hca.clients[ident.Name] = &ClientInfo{
										Name:       ident.Name,
										HasTimeout: hasTimeout,
										Position:   fset.Position(comp.Pos()),
									}
								}
							}
						}
					}
				}
			}
		}

		// Check for http.DefaultClient usage
		if sel, ok := rhs.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				if ident.Name == "http" && sel.Sel.Name == "DefaultClient" {
					pos := fset.Position(sel.Pos())
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "HTTP_DEFAULT_CLIENT",
						Severity:   SeverityMedium,
						Message:    "Using http.DefaultClient without timeout",
						Suggestion: "Create custom client with timeout configuration",
					})
				}
			}
		}
	}

	return issues
}

func (hca *HTTPClientAnalyzer) analyzeHTTPCall(call *ast.CallExpr, filename string, fset *token.FileSet) []Issue { //nolint:gocyclo // HTTP call analysis inherently complex
	var issues []Issue

	// Check for direct http.Get, http.Post, etc. calls
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok { //nolint:nestif // HTTP call analysis requires nested checks
		if ident, ok := sel.X.(*ast.Ident); ok {
			if ident.Name == "http" {
				method := sel.Sel.Name
				directMethods := []string{"Get", "Post", "PostForm", "Head"}
				for _, m := range directMethods {
					if method == m {
						pos := fset.Position(call.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "HTTP_DIRECT_CALL",
							Severity:   SeverityMedium,
							Message:    "Using http." + method + "() uses DefaultClient without timeout",
							Suggestion: "Use custom client with timeout for HTTP requests",
						})
						break
					}
				}

				// Check for NewRequest without context
				if method == "NewRequest" {
					pos := fset.Position(call.Pos())
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "HTTP_NO_CONTEXT",
						Severity:   SeverityLow,
						Message:    "Using http.NewRequest without context",
						Suggestion: "Use http.NewRequestWithContext for cancellation support",
					})
				}
			}

			// Check for response body not being closed
			if sel.Sel.Name == "Do" || sel.Sel.Name == "Get" || sel.Sel.Name == "Post" {
				// Check if result is assigned
				parent := getParentAssignment(call)
				if parent != nil {
					// Check if Body.Close() is called
					if !isResponseClosed(parent) {
						pos := fset.Position(call.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "HTTP_BODY_NOT_CLOSED",
							Severity:   SeverityHigh,
							Message:    "HTTP response body may not be closed",
							Suggestion: "Always defer resp.Body.Close() after checking error",
						})
					}
				}
			}
		}
	}

	// Check for io.ReadAll on response body
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok { //nolint:nestif // Response body analysis requires nested checks
		if ident, ok := sel.X.(*ast.Ident); ok {
			if (ident.Name == "io" || ident.Name == "ioutil") && sel.Sel.Name == "ReadAll" {
				// Check if reading response body
				if len(call.Args) > 0 {
					if sel, ok := call.Args[0].(*ast.SelectorExpr); ok {
						if sel.Sel.Name == "Body" {
							pos := fset.Position(call.Pos())
							issues = append(issues, Issue{
								File:       filename,
								Line:       pos.Line,
								Column:     pos.Column,
								Position:   pos,
								Type:       "HTTP_READALL_BODY",
								Severity:   SeverityLow,
								Message:    "Reading entire response body into memory",
								Suggestion: "Consider streaming for large responses or use io.Copy with size limit",
							})
						}
					}
				}
			}
		}
	}

	return issues
}

func (hca *HTTPClientAnalyzer) analyzeClientLiteral(comp *ast.CompositeLit, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	// Check for http.Client{} literal without timeout
	if sel, ok := comp.Type.(*ast.SelectorExpr); ok { //nolint:nestif // Client literal analysis requires nested checks
		if ident, ok := sel.X.(*ast.Ident); ok {
			if ident.Name == "http" && sel.Sel.Name == "Client" {
				hasTimeout := false

				for _, elt := range comp.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if key, ok := kv.Key.(*ast.Ident); ok {
							if key.Name == "Timeout" {
								hasTimeout = true
								break
							}
						}
					}
				}

				if !hasTimeout {
					pos := fset.Position(comp.Pos())
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "HTTP_CLIENT_NO_TIMEOUT",
						Severity:   SeverityHigh,
						Message:    "HTTP client literal without timeout",
						Suggestion: "Add Timeout: 30 * time.Second or appropriate value",
					})
				}
			}
		}
	}

	return issues
}

func (hca *HTTPClientAnalyzer) analyzeTransport(expr ast.Expr, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	// Check for Transport configuration
	if comp, ok := expr.(*ast.CompositeLit); ok { //nolint:nestif // Transport analysis requires nested checks
		hasMaxConns := false

		for _, elt := range comp.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				if key, ok := kv.Key.(*ast.Ident); ok {
					switch key.Name {
					case "DialContext":
						// Has dial context
					case "IdleConnTimeout":
						// Has idle conn timeout
					case "MaxIdleConns", "MaxIdleConnsPerHost":
						hasMaxConns = true
					case "DisableKeepAlives":
						// Check if keep-alives are disabled
						if lit, ok := kv.Value.(*ast.Ident); ok {
							if lit.Name == "true" {
								pos := fset.Position(kv.Pos())
								issues = append(issues, Issue{
									File:       filename,
									Line:       pos.Line,
									Column:     pos.Column,
									Position:   pos,
									Type:       "HTTP_KEEPALIVE_DISABLED",
									Severity:   SeverityLow,
									Message:    "HTTP keep-alives disabled, may impact performance",
									Suggestion: "Consider enabling keep-alives for connection reuse",
								})
							}
						}
					}
				}
			}
		}

		if !hasMaxConns {
			pos := fset.Position(comp.Pos())
			issues = append(issues, Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       "HTTP_NO_CONNECTION_LIMIT",
				Severity:   SeverityMedium,
				Message:    "HTTP transport without connection limits",
				Suggestion: "Set MaxIdleConns and MaxIdleConnsPerHost to prevent connection exhaustion",
			})
		}
	}

	return issues
}

// Helper functions
func getParentAssignment(_ *ast.CallExpr) *ast.AssignStmt {
	// Simplified - would need actual parent tracking
	return nil
}

func isResponseClosed(_ *ast.AssignStmt) bool {
	// Simplified - would need to track defer statements
	return false
}
