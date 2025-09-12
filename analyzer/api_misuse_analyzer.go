package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// APIMisuseAnalyzer detects common API misuse patterns
type APIMisuseAnalyzer struct {
	name string
}

// NewAPIMisuseAnalyzer creates a new API misuse analyzer
func NewAPIMisuseAnalyzer() Analyzer {
	return &APIMisuseAnalyzer{
		name: "API Misuse",
	}
}

func (a *APIMisuseAnalyzer) Name() string {
	return a.name
}

func (a *APIMisuseAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			issues = append(issues, a.checkCallExpr(node, fset)...)
		}
		return true
	})

	return issues
}

func (a *APIMisuseAnalyzer) checkCallExpr(call *ast.CallExpr, fset *token.FileSet) []Issue {
	var issues []Issue

	// Get function name
	funcName := getFuncName(call)
	if funcName == "" {
		return issues
	}

	// Check for pprof.StartCPUProfile with nil
	if strings.Contains(funcName, "StartCPUProfile") {
		if len(call.Args) > 0 {
			if ident, ok := call.Args[0].(*ast.Ident); ok && ident.Name == "nil" {
				pos := fset.Position(call.Pos())
				issues = append(issues, Issue{
					File:       pos.Filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "INVALID_API_USAGE",
					Severity:   SeverityHigh,
					Message:    "pprof.StartCPUProfile requires io.Writer, not nil",
					Suggestion: "Pass an io.Writer (e.g., os.Create(\"cpu.prof\")) instead of nil",
					Code:       getCodeSnippet(call, fset),
				})
			}
		}
	}

	// Check for context.Background() in loops
	if strings.Contains(funcName, "Background") && isInLoop(call) {
		pos := fset.Position(call.Pos())
		issues = append(issues, Issue{
			File:       pos.Filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       "CONTEXT_IN_LOOP",
			Severity:   SeverityMedium,
			Message:    "Creating context.Background() in loop",
			Suggestion: "Create context once outside the loop and reuse",
			Code:       getCodeSnippet(call, fset),
		})
	}

	// Check for time.Now() in tight loops
	if strings.Contains(funcName, "Now") && isInLoop(call) {
		pos := fset.Position(call.Pos())
		issues = append(issues, Issue{
			File:       pos.Filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       "TIME_NOW_IN_LOOP",
			Severity:   SeverityMedium,
			Message:    "Calling time.Now() in loop - expensive syscall",
			Suggestion: "Cache time outside loop or use time.Ticker for periodic timing",
			Code:       getCodeSnippet(call, fset),
		})
	}

	// Check for json.Marshal in loops
	if strings.Contains(funcName, "Marshal") && isInLoop(call) {
		pos := fset.Position(call.Pos())
		issues = append(issues, Issue{
			File:       pos.Filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       "JSON_MARSHAL_IN_LOOP",
			Severity:   SeverityHigh,
			Message:    "JSON marshaling in loop - creates garbage",
			Suggestion: "Pre-marshal data or use streaming encoder",
			Code:       getCodeSnippet(call, fset),
		})
	}

	// Check for regexp.Compile in loops
	if strings.Contains(funcName, "Compile") && isInLoop(call) {
		pos := fset.Position(call.Pos())
		issues = append(issues, Issue{
			File:       pos.Filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       "REGEX_COMPILE_IN_LOOP",
			Severity:   SeverityHigh,
			Message:    "Compiling regex in loop - very expensive",
			Suggestion: "Compile regex once outside loop, use compiled regex",
			Code:       getCodeSnippet(call, fset),
		})
	}

	// Check for database connections in loops
	if strings.Contains(funcName, "Open") && (strings.Contains(funcName, "sql") || strings.Contains(funcName, "db")) && isInLoop(call) {
		pos := fset.Position(call.Pos())
		issues = append(issues, Issue{
			File:       pos.Filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       "DB_CONNECTION_IN_LOOP",
			Severity:   SeverityHigh,
			Message:    "Opening database connection in loop",
			Suggestion: "Use connection pooling, open connection once",
			Code:       getCodeSnippet(call, fset),
		})
	}

	return issues
}

func getFuncName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		if pkg, ok := fun.X.(*ast.Ident); ok {
			return pkg.Name + "." + fun.Sel.Name
		}
		return fun.Sel.Name
	}
	return ""
}

func isInLoop(node ast.Node) bool {
	// This is a simplified check - in real implementation,
	// we'd need to walk up the AST to find parent loop nodes
	// For now, just return false - this would need proper parent tracking
	return false
}

func getCodeSnippet(node ast.Node, fset *token.FileSet) string {
	pos := fset.Position(node.Pos())
	end := fset.Position(node.End())

	// This is simplified - would need actual file content
	return "code snippet at " + pos.String() + "-" + end.String()
}
