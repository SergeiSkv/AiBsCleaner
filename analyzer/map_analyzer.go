package analyzer

import (
	"go/ast"
	"go/token"
)

type MapAnalyzer struct{}

func NewMapAnalyzer() Analyzer {
	return &MapAnalyzer{}
}

func (ma *MapAnalyzer) Name() string {
	return "MapAnalyzer"
}

func (ma *MapAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
	issues := make([]*Issue, 0, 5) // Pre-allocate for common case

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Get filename from the first position we encounter
	filename := ""
	if astNode.Pos().IsValid() {
		filename = fset.Position(astNode.Pos()).Filename
	}

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				if ident, ok := node.Fun.(*ast.Ident); ok && ident.Name == funcMake {
					if ma.isMapWithoutSize(node) {
						pos := fset.Position(node.Pos())
						issues = append(
							issues, &Issue{
								File:       filename,
								Line:       pos.Line,
								Column:     pos.Column,
								Position:   pos,
								Type:       IssueMapCapacity,
								Severity:   SeverityLevelLow,
								Message:    "Map created without size hint may cause rehashing",
								Suggestion: "Provide size hint if known: make(map[K]V, size)",
							},
						)
					}
				}
			case *ast.RangeStmt:
				if ma.isRangeOverMapKeys(node) {
					pos := fset.Position(node.Pos())
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueMapCapacity,
							Severity:   SeverityLevelLow,
							Message:    "Iterating over map keys when value is also needed",
							Suggestion: "Use for k, v := range map to get both key and value",
						},
					)
				}
			}
			return true
		},
	)

	return issues
}

func (ma *MapAnalyzer) isMapWithoutSize(call *ast.CallExpr) bool {
	if len(call.Args) < 1 {
		return false
	}

	if mapType, ok := call.Args[0].(*ast.MapType); ok {
		return mapType != nil && len(call.Args) == 1
	}

	return false
}

func (ma *MapAnalyzer) isRangeOverMapKeys(stmt *ast.RangeStmt) bool {
	if stmt.Value != nil || stmt.Key == nil {
		return false
	}

	hasValueAccess := false
	ast.Inspect(
		stmt.Body, func(n ast.Node) bool {
			indexExpr, ok := n.(*ast.IndexExpr)
			if !ok {
				return true
			}

			ident, ok := indexExpr.X.(*ast.Ident)
			if ok && ident.Name != "" {
				hasValueAccess = true
			}
			return true
		},
	)
	return hasValueAccess
}
