package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

type RegexAnalyzer struct{}

func NewRegexAnalyzer() Analyzer {
	return &RegexAnalyzer{}
}

func (ra *RegexAnalyzer) Name() string {
	return "RegexAnalyzer"
}

func (ra *RegexAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
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

	// Use context helper for proper loop detection
	ctx := NewAnalyzerWithContext(astNode)

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			callExpr, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Check if this is a regex compilation call
			if !ra.isRegexpCompile(callExpr) {
				return true
			}

			// Check if this call is in a loop
			loopDepth := ctx.GetNodeLoopDepth(callExpr)
			inLoop := loopDepth > 0

			if inLoop {
				pos := fset.Position(callExpr.Pos())
				severity := SeverityLevelMedium
				if loopDepth > 1 {
					severity = SeverityLevelHigh
				}

				issues = append(
					issues, &Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       IssueRegexCompileInLoop,
						Severity:   severity,
						Message:    "Regular expression compiled inside loop - expensive operation",
						Suggestion: "Compile regex once outside the loop and reuse the compiled pattern",
						WhyBad: `Regex compilation in loops is expensive:
• Pattern parsing: ~1-10μs per compile
• DFA construction overhead
• Memory allocation for state machines
• No caching benefits
IMPACT: 100-1000x slower than reusing compiled regex
BETTER: var re = regexp.MustCompile(pattern) outside loop`,
					},
				)
			}

			// Also check for static patterns that should use MustCompile
			if !inLoop && ra.isStaticPattern(callExpr) && ra.isCompileNotMustCompile(callExpr) {
				pos := fset.Position(callExpr.Pos())
				issues = append(
					issues, &Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       IssueRegexCompileInFunc,
						Severity:   SeverityLevelLow,
						Message:    "regexp.Compile used instead of MustCompile for static pattern",
						Suggestion: "Use regexp.MustCompile for compile-time validation of static patterns",
						WhyBad:     "Static patterns should use MustCompile for early error detection and cleaner code",
					},
				)
			}

			return true
		},
	)

	return issues
}

// Check if this is any regexp compile call
func (ra *RegexAnalyzer) isRegexpCompile(call *ast.CallExpr) bool {
	if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			if ident.Name == pkgRegexp {
				return strings.Contains(selExpr.Sel.Name, "Compile")
			}
		}
	}
	return false
}

// Check if this uses Compile instead of MustCompile
func (ra *RegexAnalyzer) isCompileNotMustCompile(call *ast.CallExpr) bool {
	if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			if ident.Name == pkgRegexp && selExpr.Sel.Name == "Compile" {
				return true
			}
		}
	}
	return false
}

// Check if this is a static pattern (string literal)
func (ra *RegexAnalyzer) isStaticPattern(call *ast.CallExpr) bool {
	if len(call.Args) > 0 {
		if lit, ok := call.Args[0].(*ast.BasicLit); ok {
			return lit.Kind == token.STRING
		}
	}
	return false
}
