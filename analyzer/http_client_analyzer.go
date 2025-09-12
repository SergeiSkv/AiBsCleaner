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

func NewHTTPClientAnalyzer() Analyzer {
	return &HTTPClientAnalyzer{
		clients: make(map[string]*ClientInfo),
	}
}

func (hca *HTTPClientAnalyzer) Name() string {
	return "HTTPClientAnalyzer"
}

func (hca *HTTPClientAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
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

	// Reset state
	hca.clients = make(map[string]*ClientInfo)

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				issues = append(issues, hca.analyzeClientCreation(node, filename, fset)...)
			case *ast.CallExpr:
				issues = append(issues, hca.analyzeHTTPCall(node, filename, fset)...)
			case *ast.CompositeLit:
				issues = append(issues, hca.analyzeClientLiteral(node, filename, fset)...)
			}
			return true
		},
	)

	return issues
}

func (hca *HTTPClientAnalyzer) analyzeClientCreation(
	assign *ast.AssignStmt, filename string, fset *token.FileSet,
) []*Issue {
	var issues []*Issue

	for i, rhs := range assign.Rhs {
		// Check for &http.Client{}
		issues = append(issues, hca.checkHTTPClientLiteral(rhs, i, assign, filename, fset)...)

		// Check for http.DefaultClient usage
		issues = append(issues, hca.checkDefaultClientUsage(rhs, filename, fset)...)
	}

	return issues
}

func (hca *HTTPClientAnalyzer) checkHTTPClientLiteral(
	rhs ast.Expr, index int, assign *ast.AssignStmt, filename string, fset *token.FileSet,
) []*Issue {
	var issues []*Issue

	unary, ok := rhs.(*ast.UnaryExpr)
	if !ok {
		return issues
	}

	comp, ok := unary.X.(*ast.CompositeLit)
	if !ok {
		return issues
	}

	sel, ok := comp.Type.(*ast.SelectorExpr)
	if !ok {
		return issues
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != pkgHTTP || sel.Sel.Name != methodClient {
		return issues
	}

	hasTimeout := false

	// Check if Timeout is set
	for _, elt := range comp.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		if key.Name == methodTimeout {
			hasTimeout = true
		}
		if key.Name == methodTransport {
			// Check transport configuration
			issues = append(issues, hca.analyzeTransport(kv.Value, filename, fset)...)
		}
	}

	if !hasTimeout {
		pos := fset.Position(comp.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueHTTPNoTimeout,
				Severity:   SeverityLevelHigh,
				Message:    "HTTP client created without timeout",
				Suggestion: "Set Timeout field to prevent hanging requests",
			},
		)
	}

	// Store client info
	if index < len(assign.Lhs) {
		if ident, ok := assign.Lhs[index].(*ast.Ident); ok {
			hca.clients[ident.Name] = &ClientInfo{
				Name:       ident.Name,
				HasTimeout: hasTimeout,
				Position:   fset.Position(comp.Pos()),
			}
		}
	}

	return issues
}

func (hca *HTTPClientAnalyzer) checkDefaultClientUsage(rhs ast.Expr, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	sel, ok := rhs.(*ast.SelectorExpr)
	if !ok {
		return issues
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != pkgHTTP || sel.Sel.Name != "DefaultClient" {
		return issues
	}

	pos := fset.Position(sel.Pos())
	issues = append(
		issues, &Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueHTTPNoTimeout,
			Severity:   SeverityLevelMedium,
			Message:    "Using http.DefaultClient without timeout",
			Suggestion: "Create custom client with timeout configuration",
		},
	)

	return issues
}

func (hca *HTTPClientAnalyzer) analyzeHTTPCall(
	call *ast.CallExpr, filename string, fset *token.FileSet,
) []*Issue {
	var issues []*Issue

	// Check for direct http package calls
	issues = append(issues, hca.checkDirectHTTPCalls(call, filename, fset)...)

	// Check for response body closing
	issues = append(issues, hca.checkResponseBodyClosed(call, filename, fset)...)

	// Check for io.ReadAll usage
	issues = append(issues, hca.checkIOReadAll(call, filename, fset)...)

	return issues
}

func (hca *HTTPClientAnalyzer) checkDirectHTTPCalls(call *ast.CallExpr, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return issues
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != pkgHTTP {
		return issues
	}

	method := sel.Sel.Name
	directMethods := []string{methodGet, methodPost, "PostForm", "Head"}
	for _, m := range directMethods {
		if method == m {
			pos := fset.Position(call.Pos())
			issues = append(
				issues, &Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       IssueHTTPNoTimeout,
					Severity:   SeverityLevelMedium,
					Message:    "Using http." + method + "() uses DefaultClient without timeout",
					Suggestion: "Use custom client with timeout for HTTP requests",
				},
			)
			break
		}
	}

	// Check for NewRequest without context
	if method == "NewRequest" {
		pos := fset.Position(call.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueHTTPNoTimeout,
				Severity:   SeverityLevelLow,
				Message:    "Using http.NewRequest without context",
				Suggestion: "Use http.NewRequestWithContext for cancellation support",
			},
		)
	}

	return issues
}

func (hca *HTTPClientAnalyzer) checkResponseBodyClosed(call *ast.CallExpr, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return issues
	}

	if sel.Sel.Name != "Do" && sel.Sel.Name != methodGet && sel.Sel.Name != methodPost {
		return issues
	}

	// Check if result is assigned
	parent := getParentAssignment(call)
	if parent == nil {
		return issues
	}

	// Check if Body.Close() is called
	if !isResponseClosed(parent) {
		pos := fset.Position(call.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueHTTPNoTimeout,
				Severity:   SeverityLevelHigh,
				Message:    "HTTP response body may not be closed",
				Suggestion: "Always defer resp.Body.Close() after checking error",
			},
		)
	}

	return issues
}

func (hca *HTTPClientAnalyzer) checkIOReadAll(call *ast.CallExpr, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return issues
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return issues
	}

	if (ident.Name != pkgIO && ident.Name != pkgIOutil) || sel.Sel.Name != methodReadAll {
		return issues
	}

	// Check if reading response body
	if len(call.Args) == 0 {
		return issues
	}

	sel, ok = call.Args[0].(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Body" {
		return issues
	}

	pos := fset.Position(call.Pos())
	issues = append(
		issues, &Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueHTTPNoTimeout,
			Severity:   SeverityLevelLow,
			Message:    "Reading entire response body into memory",
			Suggestion: "Consider streaming for large responses or use io.Copy with size limit",
		},
	)

	return issues
}

func (hca *HTTPClientAnalyzer) analyzeClientLiteral(comp *ast.CompositeLit, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	// Check for http.Client{} literal without timeout
	sel, ok := comp.Type.(*ast.SelectorExpr)
	if !ok {
		return issues
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != pkgHTTP || sel.Sel.Name != methodClient {
		return issues
	}

	if hca.hasTimeoutField(comp) {
		return issues
	}

	pos := fset.Position(comp.Pos())
	issues = append(
		issues, &Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueHTTPNoTimeout,
			Severity:   SeverityLevelHigh,
			Message:    "HTTP client literal without timeout",
			Suggestion: "Add Timeout: 30 * time.Second or appropriate value",
		},
	)

	return issues
}

func (hca *HTTPClientAnalyzer) analyzeTransport(expr ast.Expr, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	// Check for Transport configuration
	comp, ok := expr.(*ast.CompositeLit)
	if !ok {
		return issues
	}

	hasMaxConns := false

	for _, elt := range comp.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		switch key.Name {
		case "DialContext":
			// Has dial context
		case "IdleConnTimeout":
			// Has idle conn timeout
		case "MaxIdleConns", "MaxIdleConnsPerHost":
			hasMaxConns = true
		case "DisableKeepAlives":
			if issue := hca.checkKeepAlives(kv, filename, fset); issue != nil {
				issues = append(issues, issue)
			}
		}
	}

	if !hasMaxConns {
		pos := fset.Position(comp.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueHTTPNoTimeout,
				Severity:   SeverityLevelMedium,
				Message:    "HTTP transport without connection limits",
				Suggestion: "Set MaxIdleConns and MaxIdleConnsPerHost to prevent connection exhaustion",
			},
		)
	}

	return issues
}

// Helper functions
func (hca *HTTPClientAnalyzer) checkKeepAlives(kv *ast.KeyValueExpr, filename string, fset *token.FileSet) *Issue {
	lit, ok := kv.Value.(*ast.Ident)
	if !ok || lit.Name != "true" {
		return nil
	}

	pos := fset.Position(kv.Pos())
	return &Issue{
		File:       filename,
		Line:       pos.Line,
		Column:     pos.Column,
		Position:   pos,
		Type:       IssueHTTPNoTimeout,
		Severity:   SeverityLevelLow,
		Message:    "HTTP keep-alives disabled, may impact performance",
		Suggestion: "Consider enabling keep-alives for connection reuse",
	}
}

func (hca *HTTPClientAnalyzer) hasTimeoutField(comp *ast.CompositeLit) bool {
	for _, elt := range comp.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		if key.Name == methodTimeout {
			return true
		}
	}
	return false
}

func getParentAssignment(_ *ast.CallExpr) *ast.AssignStmt {
	// Simplified - would need actual parent tracking
	return nil
}

func isResponseClosed(_ *ast.AssignStmt) bool {
	// Simplified - would need to track defer statements
	return false
}
