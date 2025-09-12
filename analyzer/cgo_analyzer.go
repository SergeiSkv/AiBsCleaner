package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// CGOAnalyzer detects performance issues with CGO usage
type CGOAnalyzer struct{}

func NewCGOAnalyzer() Analyzer {
	return &CGOAnalyzer{}
}

func (ca *CGOAnalyzer) Name() string {
	return "CGOAnalyzer"
}

func (ca *CGOAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
	var issues []*Issue

	// Check if file uses CGO
	if !ca.usesCGO(node) {
		return issues
	}

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	cgoCallsInFunc := make(map[string]int)
	currentFunc := ""

	// Use context helper for proper loop detection
	ctx := NewAnalyzerWithContext(astNode)

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			if n == nil {
				return true
			}

			// Track current function
			if fn, ok := n.(*ast.FuncDecl); ok {
				if fn.Name != nil {
					currentFunc = fn.Name.Name
					cgoCallsInFunc[currentFunc] = 0
				}
			}

			// Check for CGO calls
			call, ok := n.(*ast.CallExpr)
			if !ok || !ca.isCGOCall(call) {
				return true
			}

			// Track calls per function
			if currentFunc != "" {
				cgoCallsInFunc[currentFunc]++
			}

			callIssues := ca.analyzeCGOCall(call, fset, ctx)
			issues = append(issues, callIssues...)

			return true
		},
	)

	// Check functions with too many CGO calls
	for funcName, count := range cgoCallsInFunc {
		if count > 5 {
			// Get filename from any valid position
			filename := ""
			if astNode.Pos().IsValid() {
				filename = fset.Position(astNode.Pos()).Filename
			}
			issues = append(
				issues, &Issue{
					File:       filename,
					Line:       1,
					Type:       IssueCGOCall,
					Severity:   SeverityLevelMedium,
					Message:    "Function '" + funcName + "' makes too many CGO calls",
					Suggestion: "Consider implementing entire function in C or Go",
				},
			)
		}
	}

	return issues
}

func (ca *CGOAnalyzer) usesCGO(node interface{}) bool {
	// Check for import "C" or CGO comments
	if file, ok := node.(*ast.File); ok {
		for _, imp := range file.Imports {
			if imp.Path != nil && imp.Path.Value == `"C"` {
				return true
			}
		}

		// Check for CGO preamble comments
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if strings.Contains(comment.Text, "#include") ||
					strings.Contains(comment.Text, "#cgo") {
					return true
				}
			}
		}
	}
	return false
}

func (ca *CGOAnalyzer) isCGOCall(call *ast.CallExpr) bool {
	// Check if it's a C.* function call
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			return ident.Name == "C"
		}
	}
	return false
}

func (ca *CGOAnalyzer) isSmallCGOOperation(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "C" {
			// List of typically small operations
			smallOps := []string{
				"strlen", "strcmp", "strcpy", "memcpy", "memset",
				"abs", "min", "max", "sqrt", "pow",
			}
			funcName := sel.Sel.Name
			for _, op := range smallOps {
				if funcName == op {
					return true
				}
			}
		}
	}
	return false
}

// Currently unused but kept for future enhancements
// 	// This would need more context to properly detect
// 	// For now, simplified check
// }

func (ca *CGOAnalyzer) analyzeCGOCall(call *ast.CallExpr, fset *token.FileSet, ctx *AnalyzerWithContext) []*Issue {
	var issues []*Issue
	pos := fset.Position(call.Pos())
	loopDepth := ctx.GetNodeLoopDepth(call)
	inLoop := loopDepth > 0

	// CGO call in hot path (loop)
	if inLoop {
		severity := SeverityLevelMedium
		if loopDepth > 1 {
			severity = SeverityLevelHigh
		}

		issues = append(
			issues, &Issue{
				File:       pos.Filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueCGOInLoop,
				Severity:   severity,
				Message:    "CGO call inside loop causes significant overhead (~200ns per call)",
				Suggestion: "Batch CGO operations or move outside the loop",
				WhyBad: `CGO calls have high overhead:
• Stack switching between Go and C (~50-200ns)
• Cannot be inlined by Go compiler  
• Blocks Go scheduler during C execution
• In nested loops: overhead multiplied by iterations
IMPACT: 100-1000x slower than pure Go calls`,
			},
		)
	}

	// Check for small CGO operations
	if ca.isSmallCGOOperation(call) {
		issues = append(
			issues, &Issue{
				File:       pos.Filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueCGOCall,
				Severity:   SeverityLevelMedium,
				Message:    "CGO overhead exceeds operation cost for small operations",
				Suggestion: "Use pure Go implementation for simple operations",
				WhyBad: `CGO overhead (~200ns) is too high for simple operations like:
• Basic arithmetic
• Simple string operations
• Small memory copies
BETTER: Implement in pure Go unless C provides significant algorithmic advantage`,
			},
		)
	}

	// Check for Go callbacks from C
	if ca.isGoCallback(call) {
		issues = append(
			issues, &Issue{
				File:       pos.Filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueCGOCall,
				Severity:   SeverityLevelHigh,
				Message:    "Go callback from C has double overhead (C->Go->C)",
				Suggestion: "Minimize callbacks or implement logic entirely in C",
				WhyBad: `Go callbacks from C have double overhead:
• C to Go transition (~200ns)
• Go to C transition when returning (~200ns)
• Cannot use goroutines in callbacks
• May cause deadlocks with Go scheduler`,
			},
		)
	}

	// Check string/slice conversions
	if ca.hasCGOConversion(call) {
		issues = append(
			issues, &Issue{
				File:       pos.Filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueCGOMemoryLeak,
				Severity:   SeverityLevelMedium,
				Message:    "C string/array conversion allocates and copies memory",
				Suggestion: "Reuse buffers or use unsafe.Pointer for zero-copy when safe",
				WhyBad: `CGO conversions cause allocations:
• C.CString() allocates and copies
• C.GoString() allocates and copies
• Slice conversions copy entire data
IMPACT: Memory allocation + copy overhead`,
			},
		)
	}

	return issues
}

func (ca *CGOAnalyzer) isGoCallback(call *ast.CallExpr) bool {
	// Check for export comments and callback patterns
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != "C" {
		return false
	}

	funcName := sel.Sel.Name
	// Check for common callback patterns
	if strings.Contains(funcName, "callback") || strings.Contains(funcName, "Callback") {
		return true
	}

	// Check if passing Go function to C or C.callback type conversion
	for _, arg := range call.Args {
		if ca.isCallbackArg(arg) {
			return true
		}
	}
	return false
}

func (ca *CGOAnalyzer) isCallbackArg(arg ast.Expr) bool {
	// Function literal
	if _, ok := arg.(*ast.FuncLit); ok {
		return true
	}

	// Type conversion like C.callback(C.goCallback)
	callExpr, ok := arg.(*ast.CallExpr)
	if !ok {
		return false
	}

	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	id, ok := selExpr.X.(*ast.Ident)
	if !ok || id.Name != "C" {
		return false
	}

	return strings.Contains(selExpr.Sel.Name, "callback") ||
		strings.Contains(selExpr.Sel.Name, "Callback") ||
		selExpr.Sel.Name == "callback"
}

func (ca *CGOAnalyzer) hasCGOConversion(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "C" {
			funcName := sel.Sel.Name
			conversions := []string{"CString", "GoString", "CBytes", "GoBytes"}
			for _, conv := range conversions {
				if funcName == conv {
					return true
				}
			}
		}
	}
	return false
}

// Currently unused but kept for future enhancements
// 	// Simplified - would need proper AST traversal to find parent function
// }
