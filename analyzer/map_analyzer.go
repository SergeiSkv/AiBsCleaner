package analyzer

import (
	"go/ast"
	"go/token"
)

type MapAnalyzer struct{}

func NewMapAnalyzer() *MapAnalyzer {
	return &MapAnalyzer{}
}

func (ma *MapAnalyzer) Name() string {
	return "MapAnalyzer"
}

func (ma *MapAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if ident, ok := node.Fun.(*ast.Ident); ok && ident.Name == "make" {
				if ma.isMapWithoutSize(node) {
					pos := fset.Position(node.Pos())
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "MAP_WITHOUT_SIZE_HINT",
						Severity:   SeverityLow,
						Message:    "Map created without size hint may cause rehashing",
						Suggestion: "Provide size hint if known: make(map[K]V, size)",
					})
				}
			}
		case *ast.RangeStmt:
			if ma.isRangeOverMapKeys(node) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "INEFFICIENT_MAP_ITERATION",
					Severity:   SeverityLow,
					Message:    "Iterating over map keys when value is also needed",
					Suggestion: "Use for k, v := range map to get both key and value",
				})
			}
		}
		return true
	})

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
	if stmt.Value == nil && stmt.Key != nil {
		hasValueAccess := false
		ast.Inspect(stmt.Body, func(n ast.Node) bool {
			if indexExpr, ok := n.(*ast.IndexExpr); ok {
				if ident, ok := indexExpr.X.(*ast.Ident); ok {
					if ident.Name != "" {
						hasValueAccess = true
					}
				}
			}
			return true
		})
		return hasValueAccess
	}
	return false
}