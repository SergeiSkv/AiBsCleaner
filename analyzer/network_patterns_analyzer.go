package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

const (
	pkgSQLLocal = "sql"
	pkgNetLocal = "net"
)

// NetworkPatternsAnalyzer detects inefficient network patterns
type NetworkPatternsAnalyzer struct{}

func NewNetworkPatternsAnalyzer() Analyzer {
	return &NetworkPatternsAnalyzer{}
}

func (npa *NetworkPatternsAnalyzer) Name() string {
	return "NetworkPatternsAnalyzer"
}

func (npa *NetworkPatternsAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
	issues := []*Issue{}

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Get filename from the first position we encounter
	filename := ""
	if astNode.Pos().IsValid() {
		filename = fset.Position(astNode.Pos()).Filename
	}

	// Use context helper for proper loop detection
	ctx := NewAnalyzerWithContext(astNode)

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				if node.Body != nil {
					issues = append(issues, npa.analyzeFunction(node, filename, fset, ctx)...)
				}

			case *ast.CallExpr:
				issues = append(issues, npa.analyzeNetworkCall(node, filename, fset, ctx)...)
			}
			return true
		},
	)

	return issues
}

func (npa *NetworkPatternsAnalyzer) analyzeFunction(
	fn *ast.FuncDecl, filename string, fset *token.FileSet, ctx *AnalyzerWithContext,
) []*Issue {
	issues := []*Issue{}

	// Check for N+1 API calls
	apiCallCount := 0
	var loopFound bool

	ast.Inspect(
		fn.Body, func(n ast.Node) bool {
			// Check if we're in a loop
			switch n.(type) {
			case *ast.RangeStmt, *ast.ForStmt:
				loopFound = true
			}

			// Count API calls
			if call, ok := n.(*ast.CallExpr); ok && loopFound {
				if npa.isNetworkCall(call) {
					apiCallCount++
				}
			}
			return true
		},
	)

	if apiCallCount > 1 && loopFound {
		pos := fset.Position(fn.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueNetworkInLoop,
				Severity:   SeverityLevelHigh,
				Message:    "Multiple API calls in loop - N+1 problem",
				Suggestion: "Batch API calls or use bulk endpoints",
			},
		)
	}

	// Check for unbatched operations
	issues = append(issues, npa.checkUnbatchedOperations(fn, filename, fset, ctx)...)

	// Check for missing connection pooling
	issues = append(issues, npa.checkConnectionPooling(fn, filename, fset, ctx)...)

	return issues
}

func (npa *NetworkPatternsAnalyzer) analyzeNetworkCall(
	call *ast.CallExpr, filename string, fset *token.FileSet, ctx *AnalyzerWithContext,
) []*Issue {
	issues := []*Issue{}

	// Check for synchronous calls that could be concurrent
	if npa.isSyncNetworkCall(call) {
		// Check if multiple similar calls exist in the same scope
		if npa.hasMultipleSimilarCalls(call) {
			pos := fset.Position(call.Pos())
			issues = append(
				issues, &Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       IssueNetworkInLoop,
					Severity:   SeverityLevelMedium,
					Message:    "Sequential network calls could be concurrent",
					Suggestion: "Use goroutines with sync.WaitGroup for parallel requests",
				},
			)
		}
	}

	// Check for inefficient serialization
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return issues
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return issues
	}

	// Check for JSON marshaling in hot paths
	if ident.Name == "json" && (sel.Sel.Name == "Marshal" || sel.Sel.Name == "Unmarshal") {
		if ctx.IsNodeInLoop(call) {
			pos := fset.Position(call.Pos())
			issues = append(
				issues, &Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       IssueNetworkInLoop,
					Severity:   SeverityLevelHigh,
					Message:    "JSON marshaling/unmarshaling in loop is expensive",
					Suggestion: "Move serialization outside loop or use streaming JSON",
				},
			)
		}
	}

	// Check for gob encoding (slower than alternatives)
	if ident.Name == "gob" {
		pos := fset.Position(call.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueNetworkInLoop,
				Severity:   SeverityLevelLow,
				Message:    "Gob encoding is slower than protobuf or msgpack",
				Suggestion: "Consider protobuf or msgpack for better performance",
			},
		)
	}

	// Check for missing compression
	if npa.isLargeDataTransfer(call) {
		pos := fset.Position(call.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueNetworkInLoop,
				Severity:   SeverityLevelMedium,
				Message:    "Large data transfer without compression",
				Suggestion: "Enable gzip compression for large payloads",
			},
		)
	}

	return issues
}

func (npa *NetworkPatternsAnalyzer) checkUnbatchedOperations(
	fn *ast.FuncDecl, filename string, fset *token.FileSet, _ *AnalyzerWithContext,
) []*Issue {
	issues := []*Issue{}

	// Look for patterns like multiple individual inserts/updates
	var dbCalls []ast.Node
	ast.Inspect(
		fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if npa.isDatabaseCall(call) {
					dbCalls = append(dbCalls, call)
				}
			}
			return true
		},
	)

	if len(dbCalls) > 3 {
		pos := fset.Position(fn.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueNetworkInLoop,
				Severity:   SeverityLevelHigh,
				Message:    "Multiple individual database operations could be batched",
				Suggestion: "Use bulk insert/update operations or transactions",
			},
		)
	}

	return issues
}

func (npa *NetworkPatternsAnalyzer) checkConnectionPooling(
	fn *ast.FuncDecl, filename string, fset *token.FileSet, ctx *AnalyzerWithContext,
) []*Issue {
	issues := []*Issue{}

	// Check for creating connections in loops or functions
	ast.Inspect(
		fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if npa.isConnectionCreation(call) {
					// Check if it's in a frequently called function or loop
					if ctx.IsNodeInLoop(call) || npa.isFrequentlyCalledFunction(fn) {
						pos := fset.Position(call.Pos())
						issues = append(
							issues, &Issue{
								File:       filename,
								Line:       pos.Line,
								Column:     pos.Column,
								Position:   pos,
								Type:       IssueNetworkInLoop,
								Severity:   SeverityLevelHigh,
								Message:    "Creating new connection per request - use connection pool",
								Suggestion: "Use connection pooling to reuse connections",
							},
						)
					}
				}
			}
			return true
		},
	)

	return issues
}

// Helper functions
func (npa *NetworkPatternsAnalyzer) isNetworkCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	if ident, ok := sel.X.(*ast.Ident); ok {
		networkPackages := []string{"http", "grpc", "rpc", "net"}
		for _, pkg := range networkPackages {
			if strings.Contains(ident.Name, pkg) {
				return true
			}
		}
	}

	// Check for common network method patterns
	networkMethods := []string{methodGet, methodPost, "Put", "Delete", "Fetch", "Request", "Call"}
	for _, method := range networkMethods {
		if strings.Contains(sel.Sel.Name, method) {
			return true
		}
	}
	return false
}

func (npa *NetworkPatternsAnalyzer) isSyncNetworkCall(call *ast.CallExpr) bool {
	// Check if it's a synchronous HTTP call
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			if ident.Name == "http" && (sel.Sel.Name == methodGet || sel.Sel.Name == methodPost) {
				return true
			}
		}
	}
	return false
}

func (npa *NetworkPatternsAnalyzer) hasMultipleSimilarCalls(call *ast.CallExpr) bool {
	// Simplified - would need to analyze the entire function scope
	return false
}

func (npa *NetworkPatternsAnalyzer) isLargeDataTransfer(call *ast.CallExpr) bool {
	// Check for methods that typically transfer large data
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		largeDataMethods := []string{"SendFile", "Upload", "Download", "Transfer"}
		for _, method := range largeDataMethods {
			if strings.Contains(sel.Sel.Name, method) {
				return true
			}
		}
	}
	return false
}

func (npa *NetworkPatternsAnalyzer) isDatabaseCall(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		dbMethods := []string{"Exec", "Query", "Insert", "Update", "Delete", "Save"}
		for _, method := range dbMethods {
			if strings.Contains(sel.Sel.Name, method) {
				return true
			}
		}
	}
	return false
}

func (npa *NetworkPatternsAnalyzer) isConnectionCreation(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			// Check for connection creation patterns
			if (ident.Name == pkgSQLLocal && sel.Sel.Name == "Open") ||
				(ident.Name == pkgNetLocal && sel.Sel.Name == methodDial) ||
				(ident.Name == "grpc" && sel.Sel.Name == methodDial) {
				return true
			}
		}
	}
	return false
}

func (npa *NetworkPatternsAnalyzer) isFrequentlyCalledFunction(fn *ast.FuncDecl) bool {
	// Check if a function name indicates it's frequently called
	name := strings.ToLower(fn.Name.Name)
	frequentPatterns := []string{"handle", "process", "serve", "request", "api", "endpoint"}
	for _, pattern := range frequentPatterns {
		if strings.Contains(name, pattern) {
			return true
		}
	}
	return false
}
