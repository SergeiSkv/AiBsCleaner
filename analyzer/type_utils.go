package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

type typeTable struct {
	byPos map[token.Pos]string
}

func newTypeTable() *typeTable {
	return &typeTable{byPos: make(map[token.Pos]string, 32)}
}

func (tt *typeTable) record(pos token.Pos, typ string) {
	if pos == token.NoPos {
		return
	}
	types := tt.byPos
	types[pos] = typ
}

func (tt *typeTable) lookup(pos token.Pos) string {
	if pos == token.NoPos {
		return ""
	}
	return tt.byPos[pos]
}

func canonicalType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if pkg, ok := t.X.(*ast.Ident); ok {
			return pkg.Name + "." + t.Sel.Name
		}
	case *ast.StarExpr:
		inner := canonicalType(t.X)
		if inner != "" {
			return inner
		}
	}
	return ""
}

func isWaitGroupType(typ string) bool {
	return typ == "sync.WaitGroup" || strings.HasSuffix(typ, ".WaitGroup")
}

func isStringLiteral(expr ast.Expr) bool {
	lit, ok := expr.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING
}
