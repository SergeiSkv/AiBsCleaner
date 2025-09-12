package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
)

// HTTPReuseAnalyzer detects HTTP client reuse issues and inefficiencies
type HTTPReuseAnalyzer struct {
	filename string
	fset     *token.FileSet
	ctx      *AnalyzerWithContext
	issues   []*Issue
}

func NewHTTPReuseAnalyzer() Analyzer {
	return &HTTPReuseAnalyzer{}
}

func (ha *HTTPReuseAnalyzer) Name() string {
	return "HTTPReuseAnalyzer"
}

func (ha *HTTPReuseAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
	astNode, ok := node.(ast.Node)
	if !ok {
		return nil
	}

	// Get filename from the first position we encounter
	filename := ""
	if astNode.Pos().IsValid() {
		filename = fset.Position(astNode.Pos()).Filename
	}

	ha.filename = filename
	ha.fset = fset
	ha.ctx = NewAnalyzerWithContext(astNode)
	ha.issues = make([]*Issue, 0)

	state := &httpAnalysisState{
		httpClientsInFunc: make(map[string]int),
		transportCount:    0,
		currentFunc:       "",
	}

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			ha.inspectNode(n, state)
			return true
		},
	)

	// Add summary issues
	ha.addSummaryIssues(state)
	return ha.issues
}

type httpAnalysisState struct {
	httpClientsInFunc map[string]int
	transportCount    int
	currentFunc       string
}

func (ha *HTTPReuseAnalyzer) inspectNode(n ast.Node, state *httpAnalysisState) {
	switch node := n.(type) {
	case *ast.FuncDecl:
		ha.handleFuncDecl(node, state)
	case *ast.UnaryExpr:
		ha.handleUnaryExpr(node, state)
	case *ast.CallExpr:
		ha.handleCallExpr(node)
	}
}

func (ha *HTTPReuseAnalyzer) handleFuncDecl(node *ast.FuncDecl, state *httpAnalysisState) {
	if node.Name != nil {
		state.currentFunc = node.Name.Name
		state.httpClientsInFunc[state.currentFunc] = 0
	}
}

func (ha *HTTPReuseAnalyzer) handleUnaryExpr(node *ast.UnaryExpr, state *httpAnalysisState) {
	if node.Op != token.AND {
		return
	}

	comp, ok := node.X.(*ast.CompositeLit)
	if !ok {
		return
	}

	// Check for &http.Client{}
	if ha.isHTTPClientCreation(comp) {
		ha.checkHTTPClient(node, comp, state)
	}

	// Check for &http.Transport{}
	if ha.isTransportCreation(comp) {
		ha.checkHTTPTransport(node, state)
	}
}

func (ha *HTTPReuseAnalyzer) checkHTTPClient(node *ast.UnaryExpr, comp *ast.CompositeLit, state *httpAnalysisState) {
	pos := ha.fset.Position(node.Pos())
	loopDepth := ha.ctx.GetNodeLoopDepth(node)
	inLoop := loopDepth > 0

	// Track client creation
	if state.currentFunc != "" {
		state.httpClientsInFunc[state.currentFunc]++
	}

	if inLoop {
		ha.addClientInLoopIssue(pos)
	}

	if !ha.hasTimeout(comp) {
		ha.addMissingTimeoutIssue(pos)
	}
}

func (ha *HTTPReuseAnalyzer) checkHTTPTransport(node *ast.UnaryExpr, state *httpAnalysisState) {
	state.transportCount++
	pos := ha.fset.Position(node.Pos())
	loopDepth := ha.ctx.GetNodeLoopDepth(node)

	if loopDepth > 0 {
		ha.addTransportInLoopIssue(pos)
	}
}

func (ha *HTTPReuseAnalyzer) handleCallExpr(node *ast.CallExpr) {
	pos := ha.fset.Position(node.Pos())
	loopDepth := ha.ctx.GetNodeLoopDepth(node)
	inLoop := loopDepth > 0

	// Check various HTTP call patterns
	if ha.isDefaultHTTPCall(node) && inLoop {
		ha.addDefaultClientInLoopIssue(pos)
	}

	// Response close detection would need proper data flow analysis
	// Removed for now as it was always returning false

	if ha.isInefficientRequest(node) {
		ha.addInefficientRequestIssue(pos)
	}

	if ha.isMissingPoolConfig(node) {
		ha.addMissingPoolConfigIssue(pos)
	}

	if ha.hasDisabledKeepAlive(node) {
		ha.addDisabledKeepAliveIssue(pos)
	}

	if ha.isReadingFullResponse(node) {
		ha.addReadingFullResponseIssue(pos)
	}
}

// Issue creation methods
func (ha *HTTPReuseAnalyzer) addClientInLoopIssue(pos token.Position) {
	ha.issues = append(
		ha.issues, &Issue{
			File:       ha.filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueHTTPNoConnectionReuse,
			Severity:   SeverityLevelHigh,
			Message:    "Creating http.Client in loop - causes connection exhaustion",
			Suggestion: "Create http.Client once and reuse across requests",
			WhyBad: `Creating http.Client in loops causes:
• New connection pool for each client
• TCP connection exhaustion
• No connection reuse benefits
• File descriptor leaks
IMPACT: 10-100x slower, system resource exhaustion
BETTER: Use single shared http.Client or http.DefaultClient`,
		},
	)
}

func (ha *HTTPReuseAnalyzer) addMissingTimeoutIssue(pos token.Position) {
	ha.issues = append(
		ha.issues, &Issue{
			File:       ha.filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueHTTPNoConnectionReuse,
			Severity:   SeverityLevelHigh,
			Message:    "HTTP client without timeout can hang forever",
			Suggestion: "Set Client.Timeout or use context with timeout",
			WhyBad: `Missing timeout causes:
• Goroutine leaks on slow servers
• Resource exhaustion
• Application hangs
ALWAYS: Set reasonable timeout (e.g., 30s)`,
		},
	)
}

func (ha *HTTPReuseAnalyzer) addTransportInLoopIssue(pos token.Position) {
	ha.issues = append(
		ha.issues, &Issue{
			File:       ha.filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueHTTPNoConnectionReuse,
			Severity:   SeverityLevelHigh,
			Message:    "Creating http.Transport in loop - prevents connection pooling",
			Suggestion: "Create Transport once with proper settings and reuse",
			WhyBad: `http.Transport manages connection pooling:
• Each Transport has its own connection pool
• Creating new = losing all pooled connections
• No HTTP/2 connection reuse
IMPACT: Every request creates new TCP connection`,
		},
	)
}

func (ha *HTTPReuseAnalyzer) addDefaultClientInLoopIssue(pos token.Position) {
	ha.issues = append(
		ha.issues, &Issue{
			File:       ha.filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueHTTPNoConnectionReuse,
			Severity:   SeverityLevelLow,
			Message:    "Using http.Get/Post in loop - consider custom client for tuning",
			Suggestion: "For high-throughput, configure custom client with appropriate limits",
			WhyBad: `http.DefaultClient limitations:
• No timeout configured (can hang forever)
• Default connection limits may be insufficient
• Cannot configure per-service settings`,
		},
	)
}

func (ha *HTTPReuseAnalyzer) addInefficientRequestIssue(pos token.Position) {
	ha.issues = append(
		ha.issues, &Issue{
			File:       ha.filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueHTTPNoConnectionReuse,
			Severity:   SeverityLevelLow,
			Message:    "Creating request with string concatenation",
			Suggestion: "Use url.Values or proper URL building",
		},
	)
}

func (ha *HTTPReuseAnalyzer) addMissingPoolConfigIssue(pos token.Position) {
	ha.issues = append(
		ha.issues, &Issue{
			File:       ha.filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueHTTPNoConnectionReuse,
			Severity:   SeverityLevelMedium,
			Message:    "Using default connection pool settings",
			Suggestion: "Configure MaxIdleConns, MaxConnsPerHost for your load",
			WhyBad: `Default pool settings may be insufficient:
• MaxIdleConns: 100 (total)
• MaxIdleConnsPerHost: 2 (too low for API clients)
• May cause unnecessary connection churn
TUNE: Based on your concurrent request patterns`,
		},
	)
}

func (ha *HTTPReuseAnalyzer) addDisabledKeepAliveIssue(pos token.Position) {
	ha.issues = append(
		ha.issues, &Issue{
			File:       ha.filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueHTTPNoConnectionReuse,
			Severity:   SeverityLevelMedium,
			Message:    "HTTP KeepAlive disabled - prevents connection reuse",
			Suggestion: "Enable KeepAlive unless you have specific reasons",
			WhyBad: `Disabling KeepAlive:
• New TCP connection for every request
• No HTTP/2 benefits
• 3-way handshake overhead on each request
• TLS handshake overhead on each HTTPS request`,
		},
	)
}

func (ha *HTTPReuseAnalyzer) addReadingFullResponseIssue(pos token.Position) {
	ha.issues = append(
		ha.issues, &Issue{
			File:       ha.filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueHTTPNoConnectionReuse,
			Severity:   SeverityLevelMedium,
			Message:    "Reading full response into memory with ioutil.ReadAll",
			Suggestion: "Use io.Copy or json.Decoder for streaming large responses",
			WhyBad: `ioutil.ReadAll problems:
• Loads entire response into memory
• Can cause OOM on large responses
• No streaming processing possible
BETTER: Stream with io.Copy or use json.Decoder directly`,
		},
	)
}

func (ha *HTTPReuseAnalyzer) addSummaryIssues(state *httpAnalysisState) {
	// Check for multiple HTTP clients in same function
	for funcName, count := range state.httpClientsInFunc {
		if count > 1 {
			ha.issues = append(
				ha.issues, &Issue{
					File:       ha.filename,
					Line:       1,
					Type:       IssueHTTPNoConnectionReuse,
					Severity:   SeverityLevelMedium,
					Message:    fmt.Sprintf("Function '%s' creates %d HTTP clients", funcName, count),
					Suggestion: "Reuse single HTTP client throughout function",
				},
			)
		}
	}

	// Warn about multiple transports
	if state.transportCount > 1 {
		ha.issues = append(
			ha.issues, &Issue{
				File:       ha.filename,
				Line:       1,
				Type:       IssueHTTPNoConnectionReuse,
				Severity:   SeverityLevelMedium,
				Message:    fmt.Sprintf("File creates %d http.Transport instances", state.transportCount),
				Suggestion: "Usually one Transport per service is sufficient",
			},
		)
	}
}

// Helper methods
func (ha *HTTPReuseAnalyzer) hasTimeout(comp *ast.CompositeLit) bool {
	for _, elt := range comp.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if ident, ok := kv.Key.(*ast.Ident); ok {
				if ident.Name == "Timeout" {
					return true
				}
			}
		}
	}
	return false
}

func (ha *HTTPReuseAnalyzer) isHTTPClientCreation(lit *ast.CompositeLit) bool {
	if lit.Type == nil {
		return false
	}

	if sel, ok := lit.Type.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			return ident.Name == pkgHTTP && sel.Sel.Name == "Client"
		}
	}
	return false
}

func (ha *HTTPReuseAnalyzer) isTransportCreation(lit *ast.CompositeLit) bool {
	if lit.Type == nil {
		return false
	}

	if sel, ok := lit.Type.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			return ident.Name == pkgHTTP && sel.Sel.Name == "Transport"
		}
	}
	return false
}

func (ha *HTTPReuseAnalyzer) isDefaultHTTPCall(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == pkgHTTP {
			method := sel.Sel.Name
			return method == methodGet || method == methodPost || method == "Head" || method == "PostForm"
		}
	}
	return false
}

func (ha *HTTPReuseAnalyzer) isInefficientRequest(node *ast.CallExpr) bool {
	// Check for URL building with string concatenation
	if _, ok := node.Fun.(*ast.Ident); ok {
		for _, arg := range node.Args {
			if binExpr, ok := arg.(*ast.BinaryExpr); ok {
				if binExpr.Op == token.ADD {
					// String concatenation for URLs
					return true
				}
			}
		}
	}
	return false
}

func (ha *HTTPReuseAnalyzer) isMissingPoolConfig(node *ast.CallExpr) bool {
	// Would need to analyze Transport configuration
	return false
}

func (ha *HTTPReuseAnalyzer) hasDisabledKeepAlive(node *ast.CallExpr) bool {
	// Check for DisableKeepAlives = true in Transport
	return false
}

func (ha *HTTPReuseAnalyzer) isReadingFullResponse(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == pkgIOutil {
			return sel.Sel.Name == "ReadAll"
		}
	}
	return false
}
