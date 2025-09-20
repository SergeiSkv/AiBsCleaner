package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// GCPressureAnalyzer detects patterns that increase GC pressure
type GCPressureAnalyzer struct{}

func NewGCPressureAnalyzer() *GCPressureAnalyzer {
	return &GCPressureAnalyzer{}
}

func (gpa *GCPressureAnalyzer) Name() string {
	return "GCPressureAnalyzer"
}

func (gpa *GCPressureAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.StructType:
			// Check for pointer-heavy structs
			pointerCount := 0
			totalFields := 0

			for _, field := range node.Fields.List {
				totalFields += len(field.Names)
				if _, isPointer := field.Type.(*ast.StarExpr); isPointer {
					pointerCount += len(field.Names)
				}
			}

			if totalFields > 5 && float64(pointerCount)/float64(totalFields) > 0.7 {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "POINTER_HEAVY_STRUCT",
					Severity:   SeverityMedium,
					Message:    "Struct with many pointers increases GC scanning time",
					Suggestion: "Consider using value types or arrays instead of pointers where possible",
				})
			}

		case *ast.CallExpr:
			// Check for large allocations without size hints
			if ident, ok := node.Fun.(*ast.Ident); ok && ident.Name == "make" {
				if len(node.Args) >= 1 {
					switch node.Args[0].(type) {
					case *ast.MapType:
						if len(node.Args) < 2 {
							// Map without size hint in potential hot path
							if gpa.isInLoop(n) {
								pos := fset.Position(node.Pos())
								issues = append(issues, Issue{
									File:       filename,
									Line:       pos.Line,
									Column:     pos.Column,
									Position:   pos,
									Type:       "MAP_IN_LOOP_NO_SIZE",
									Severity:   SeverityHigh,
									Message:    "Map created in loop without size hint causes repeated allocations",
									Suggestion: "Pre-allocate map with expected size outside loop or use sync.Pool",
								})
							}
						}
					case *ast.ArrayType:
						if len(node.Args) >= 3 {
							// Check for huge slice allocations
							if lit, ok := node.Args[2].(*ast.BasicLit); ok {
								if strings.Contains(lit.Value, "000000") {
									pos := fset.Position(node.Pos())
									issues = append(issues, Issue{
										File:       filename,
										Line:       pos.Line,
										Column:     pos.Column,
										Position:   pos,
										Type:       "HUGE_SLICE_ALLOCATION",
										Severity:   SeverityHigh,
										Message:    "Very large slice allocation increases GC pressure",
										Suggestion: "Consider using sync.Pool or chunked processing",
									})
								}
							}
						}
					}
				}
			}

			// Check for string concatenation in loops (creates garbage)
			if binExpr, ok := node.Fun.(*ast.BinaryExpr); ok {
				if binExpr.Op == token.ADD {
					if gpa.isStringConcat(binExpr) && gpa.isInLoop(n) {
						pos := fset.Position(node.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "STRING_CONCAT_IN_LOOP",
							Severity:   SeverityHigh,
							Message:    "String concatenation in loop creates garbage for GC",
							Suggestion: "Use strings.Builder or bytes.Buffer",
						})
					}
				}
			}

		case *ast.CompositeLit:
			// Check for large literal allocations
			if len(node.Elts) > 1000 {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "LARGE_LITERAL_ALLOCATION",
					Severity:   SeverityMedium,
					Message:    "Large composite literal creates GC pressure",
					Suggestion: "Consider lazy initialization or streaming processing",
				})
			}

		case *ast.FuncDecl:
			// Check for escape analysis problems
			if node.Body != nil {
				issues = append(issues, gpa.checkEscapeProblems(node, filename, fset)...)
			}
		}
		return true
	})

	return issues
}

func (gpa *GCPressureAnalyzer) isInLoop(node ast.Node) bool {
	// Simplified check - in real implementation would need proper context
	return false
}

func (gpa *GCPressureAnalyzer) isStringConcat(expr *ast.BinaryExpr) bool {
	// Check if it's string concatenation
	return expr.Op == token.ADD
}

func (gpa *GCPressureAnalyzer) checkEscapeProblems(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	// Check for returning address of local variable
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok {
			for _, result := range ret.Results {
				if unary, ok := result.(*ast.UnaryExpr); ok {
					if unary.Op == token.AND {
						// Taking address of something
						if ident, ok := unary.X.(*ast.Ident); ok {
							// Check if it's a local variable
							if ident.Obj != nil && ident.Obj.Kind == ast.Var {
								pos := fset.Position(unary.Pos())
								issues = append(issues, Issue{
									File:       filename,
									Line:       pos.Line,
									Column:     pos.Column,
									Position:   pos,
									Type:       "ESCAPE_TO_HEAP",
									Severity:   SeverityMedium,
									Message:    "Returning address of local variable causes heap allocation",
									Suggestion: "Return value instead of pointer if possible",
								})
							}
						}
					}
				}
			}
		}
		return true
	})

	return issues
}
