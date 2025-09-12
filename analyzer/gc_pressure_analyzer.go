package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// GCPressureAnalyzer detects patterns that increase GC pressure
type GCPressureAnalyzer struct{}

func NewGCPressureAnalyzer() Analyzer {
	return &GCPressureAnalyzer{}
}

func (gpa *GCPressureAnalyzer) Name() string {
	return "GCPressureAnalyzer"
}

func (gpa *GCPressureAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
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
			case *ast.StructType:
				issues = append(issues, gpa.checkStructType(node, filename, fset)...)
			case *ast.CallExpr:
				issues = append(issues, gpa.checkCallExpr(node, n, filename, fset, ctx)...)
			case *ast.CompositeLit:
				issues = append(issues, gpa.checkCompositeLit(node, filename, fset)...)
			case *ast.FuncDecl:
				if node.Body != nil {
					issues = append(issues, gpa.checkEscapeProblems(node, filename, fset)...)
				}
			}
			return true
		},
	)

	return issues
}

func (gpa *GCPressureAnalyzer) checkStructType(node *ast.StructType, filename string, fset *token.FileSet) []*Issue {
	issues := []*Issue{}
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
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueHighGCPressure,
				Severity:   SeverityLevelMedium,
				Message:    "Struct with many pointers increases GC scanning time",
				Suggestion: "Consider using value types or arrays instead of pointers where possible",
			},
		)
	}
	return issues
}

func (gpa *GCPressureAnalyzer) checkCallExpr(
	node *ast.CallExpr, n ast.Node, filename string, fset *token.FileSet, ctx *AnalyzerWithContext,
) []*Issue {
	issues := []*Issue{}

	// Check for make calls
	issues = append(issues, gpa.checkMakeCall(node, n, filename, fset, ctx)...)

	// Check for string concatenation
	issues = append(issues, gpa.checkStringConcat(node, n, filename, fset, ctx)...)

	return issues
}

func (gpa *GCPressureAnalyzer) checkMakeCall(
	node *ast.CallExpr, n ast.Node, filename string, fset *token.FileSet, ctx *AnalyzerWithContext,
) []*Issue {
	issues := []*Issue{}

	ident, ok := node.Fun.(*ast.Ident)
	if !ok || ident.Name != funcMake || len(node.Args) < 1 {
		return issues
	}

	switch node.Args[0].(type) {
	case *ast.MapType:
		if len(node.Args) < 2 && ctx.IsNodeInLoop(n) {
			pos := fset.Position(node.Pos())
			issues = append(
				issues, &Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       IssueHighGCPressure,
					Severity:   SeverityLevelHigh,
					Message:    "Map created in loop without size hint causes repeated allocations",
					Suggestion: "Pre-allocate map with expected size outside loop or use sync.Pool",
				},
			)
		}
	case *ast.ArrayType:
		if len(node.Args) >= 3 {
			if lit, ok := node.Args[2].(*ast.BasicLit); ok {
				if strings.Contains(lit.Value, "000000") {
					pos := fset.Position(node.Pos())
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueHighGCPressure,
							Severity:   SeverityLevelHigh,
							Message:    "Very large slice allocation increases GC pressure",
							Suggestion: "Consider using sync.Pool or chunked processing",
						},
					)
				}
			}
		}
	}
	return issues
}

func (gpa *GCPressureAnalyzer) checkStringConcat(
	node *ast.CallExpr, n ast.Node, filename string, fset *token.FileSet, ctx *AnalyzerWithContext,
) []*Issue {
	issues := []*Issue{}

	binExpr, ok := node.Fun.(*ast.BinaryExpr)
	if !ok || binExpr.Op != token.ADD {
		return issues
	}

	if gpa.isStringConcat(binExpr) && ctx.IsNodeInLoop(n) {
		pos := fset.Position(node.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueHighGCPressure,
				Severity:   SeverityLevelHigh,
				Message:    "String concatenation in loop creates garbage for GC",
				Suggestion: "Use strings.Builder or bytes.Buffer",
			},
		)
	}
	return issues
}

func (gpa *GCPressureAnalyzer) checkCompositeLit(node *ast.CompositeLit, filename string, fset *token.FileSet) []*Issue {
	issues := []*Issue{}

	if len(node.Elts) > 1000 {
		pos := fset.Position(node.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueHighGCPressure,
				Severity:   SeverityLevelMedium,
				Message:    "Large composite literal creates GC pressure",
				Suggestion: "Consider lazy initialization or streaming processing",
			},
		)
	}
	return issues
}

func (gpa *GCPressureAnalyzer) isStringConcat(expr *ast.BinaryExpr) bool {
	// Check if it's string concatenation
	return expr.Op == token.ADD
}

func (gpa *GCPressureAnalyzer) checkEscapeProblems(fn *ast.FuncDecl, filename string, fset *token.FileSet) []*Issue {
	issues := []*Issue{}

	// Check for returning address of local variable
	ast.Inspect(
		fn.Body, func(n ast.Node) bool {
			ret, ok := n.(*ast.ReturnStmt)
			if !ok {
				return true
			}

			for _, result := range ret.Results {
				issue := gpa.checkReturnedAddressOfLocal(result, filename, fset)
				if issue != nil {
					issues = append(issues, issue)
				}
			}
			return true
		},
	)

	return issues
}

func (gpa *GCPressureAnalyzer) checkReturnedAddressOfLocal(result ast.Expr, filename string, fset *token.FileSet) *Issue {
	unary, ok := result.(*ast.UnaryExpr)
	if !ok || unary.Op != token.AND {
		return nil
	}

	ident, ok := unary.X.(*ast.Ident)
	if !ok {
		return nil
	}

	// Check if it's a local variable
	if ident.Obj == nil || ident.Obj.Kind != ast.Var {
		return nil
	}

	pos := fset.Position(unary.Pos())
	return &Issue{
		File:       filename,
		Line:       pos.Line,
		Column:     pos.Column,
		Position:   pos,
		Type:       IssueHighGCPressure,
		Severity:   SeverityLevelMedium,
		Message:    "Returning address of local variable causes heap allocation",
		Suggestion: "Return value instead of pointer if possible",
	}
}
