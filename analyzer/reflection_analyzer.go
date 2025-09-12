package analyzer

import (
	"go/ast"
	"go/token"
)

type ReflectionAnalyzer struct{}

func NewReflectionAnalyzer() Analyzer {
	return &ReflectionAnalyzer{}
}

func (ra *ReflectionAnalyzer) Name() string {
	return "ReflectionAnalyzer"
}

func (ra *ReflectionAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
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

			if !ra.isReflectCall(callExpr) {
				return true
			}

			pos := fset.Position(callExpr.Pos())

			// Check if this reflection call is in a loop
			loopDepth := ctx.GetNodeLoopDepth(callExpr)
			inLoop := loopDepth > 0

			if inLoop {
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
						Type:       IssueReflection,
						Severity:   severity,
						Message:    "Reflection usage detected in loop - expensive operation",
						Suggestion: "Consider using type assertions, code generation, or move reflection out of loop",
						WhyBad: `Reflection in loops is extremely slow:
• Type introspection: ~100-1000ns per call
• Method lookup: ~1-10μs per call
• Value conversion overhead
• Disables compiler optimizations
BETTER: Type switches, interfaces, or pre-computed reflection data`,
					},
				)
			} else {
				// General reflection usage warning
				issues = append(
					issues, &Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       IssueReflection,
						Severity:   SeverityLevelLow,
						Message:    "Reflection usage detected - consider alternatives for better performance",
						Suggestion: "Consider using type assertions, interfaces, or code generation instead of reflection",
					},
				)
			}

			return true
		},
	)

	return issues
}

func (ra *ReflectionAnalyzer) isReflectCall(call *ast.CallExpr) bool {
	if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			if ident.Name == "reflect" {
				return true
			}
		}
	}
	return false
}

// This method is kept for compatibility but not used
// 	// This method is deprecated in favor of context-based detection
// }
