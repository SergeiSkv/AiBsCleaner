package analyzer

import (
	"go/ast"
	"go/token"
)

type SliceAnalyzer struct{}

func NewSliceAnalyzer() *SliceAnalyzer {
	return &SliceAnalyzer{}
}

func (sa *SliceAnalyzer) Name() string {
	return "SliceAnalyzer"
}

func (sa *SliceAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if ident, ok := node.Fun.(*ast.Ident); ok {
				switch ident.Name {
				case "append":
					if sa.isAppendInLoop(n) {
						pos := fset.Position(node.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "APPEND_IN_LOOP",
							Severity:   SeverityMedium,
							Message:    "Multiple append calls in loop may cause excessive allocations",
							Suggestion: "Pre-allocate slice with make([]T, 0, expectedSize) if size is known",
						})
					}
				case "make":
					if sa.isMakeWithoutCapacity(node) {
						pos := fset.Position(node.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "SLICE_WITHOUT_CAPACITY",
							Severity:   SeverityLow,
							Message:    "Slice created without capacity hint",
							Suggestion: "Specify capacity if known: make([]T, 0, capacity)",
						})
					}
				}
			}
		}
		return true
	})

	return issues
}

func (sa *SliceAnalyzer) isAppendInLoop(node ast.Node) bool {
	parent := node
	depth := 0
	maxDepth := MaxSearchDepth

	for depth < maxDepth {
		switch parent.(type) {
		case *ast.ForStmt:
			return true
		case *ast.RangeStmt:
			return true
		}
		depth++
	}

	return false
}

func (sa *SliceAnalyzer) isMakeWithoutCapacity(call *ast.CallExpr) bool {
	if len(call.Args) < 1 {
		return false
	}

	if _, ok := call.Args[0].(*ast.ArrayType); ok {
		return len(call.Args) < 3
	}

	return false
}
