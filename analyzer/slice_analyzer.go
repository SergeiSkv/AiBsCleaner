package analyzer

import (
	"go/ast"
	"go/token"
)

type SliceAnalyzer struct{}

func NewSliceAnalyzer() Analyzer {
	return &SliceAnalyzer{}
}

func (sa *SliceAnalyzer) Name() string {
	return "SliceAnalyzer"
}

func (sa *SliceAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
	issues := make([]*Issue, 0, 10) // Pre-allocate for common case

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

	// Pre-size map for visited nodes to avoid rehashing
	visited := make(map[ast.Node]bool, 100)

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			// Skip if already visited
			if visited[n] {
				return false
			}
			visited[n] = true

			callExpr, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			ident, ok := callExpr.Fun.(*ast.Ident)
			if !ok {
				return true
			}

			switch ident.Name {
			case "append":
				// Check if this append call is in a loop
				loopDepth := ctx.GetNodeLoopDepth(callExpr)
				if loopDepth > 0 {
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
							Type:       IssueSliceCapacity,
							Severity:   severity,
							Message:    "Multiple append calls in loop may cause excessive memory allocations",
							Suggestion: "Pre-allocate slice with make([]T, 0, expectedSize) if size is known",
							WhyBad: `append() in loops causes repeated allocations:
• Slice capacity doubles each time: 0→1→2→4→8→16...
• Previous data must be copied to new backing array
• O(n) copy operations for n appends = O(n²) total
• GC pressure from temporary slices
IMPACT: Can cause 10-100x slowdown vs pre-allocated slice
BETTER: make([]T, 0, knownCapacity) before loop`,
						},
					)
				}
			case funcMake:
				// Check for slice creation without capacity
				if sa.isMakeWithoutCapacity(callExpr) {
					pos := fset.Position(callExpr.Pos())
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueSliceCapacity,
							Severity:   SeverityLevelLow,
							Message:    "Slice created without capacity hint may require reallocations",
							Suggestion: "Specify capacity if known: make([]T, 0, capacity)",
							WhyBad:     "Without capacity hint, slice may need multiple reallocations as it grows",
						},
					)
				}
			}
			return true
		},
	)

	return issues
}

// This method is kept for compatibility but not used
// 	// This method is deprecated in favor of context-based detection
// }

func (sa *SliceAnalyzer) isMakeWithoutCapacity(call *ast.CallExpr) bool {
	if len(call.Args) < 1 {
		return false
	}

	if _, ok := call.Args[0].(*ast.ArrayType); ok {
		return len(call.Args) < 3
	}

	return false
}
