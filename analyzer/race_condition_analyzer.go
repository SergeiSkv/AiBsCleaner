package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type RaceConditionAnalyzer struct{}

func NewRaceConditionAnalyzer() Analyzer {
	return &RaceConditionAnalyzer{}
}

func (rca *RaceConditionAnalyzer) Name() string {
	return "RaceConditionAnalyzer"
}

func (rca *RaceConditionAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	info, _ := LoadTypes(fset, file, filename)
	if info == nil {
		return nil
	}

	funcDecls := make(map[*types.Func]*ast.FuncDecl, len(file.Decls))
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}
		if obj, ok := info.Defs[fn.Name].(*types.Func); ok {
			funcDecls[obj] = fn
		}
	}

	visitor := &raceVisitor{
		fset:      fset,
		filename:  filename,
		info:      info,
		issues:    make([]*models.Issue, 0, 4),
		funcDecls: funcDecls,
	}

	ast.Walk(visitor, file)
	return visitor.issues
}

type raceVisitor struct {
	fset      *token.FileSet
	filename  string
	info      *types.Info
	issues    []*models.Issue
	funcDecls map[*types.Func]*ast.FuncDecl
}

func (v *raceVisitor) Visit(node ast.Node) ast.Visitor {
	goStmt, ok := node.(*ast.GoStmt)
	if !ok {
		return v
	}

	v.inspectGo(goStmt)
	return nil
}

func (v *raceVisitor) inspectGo(goStmt *ast.GoStmt) {
	writes := make(map[types.Object]token.Pos)

	ast.Inspect(goStmt.Call, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range stmt.Lhs {
				v.recordWrite(lhs, writes)
			}
		case *ast.IncDecStmt:
			v.recordWrite(stmt.X, writes)
		}
		return true
	})

	v.inspectFunctionCall(goStmt.Call.Fun, writes)

	for _, pos := range writes {
		v.issues = append(v.issues, &models.Issue{
			File:       v.filename,
			Line:       v.fset.Position(pos).Line,
			Column:     v.fset.Position(pos).Column,
			Position:   v.fset.Position(pos),
			Type:       models.IssueRaceCondition,
			Severity:   models.SeverityLevelHigh,
			Message:    "Write to package-level variable inside goroutine",
			Suggestion: "Guard the variable with synchronization or avoid shared state",
		})
	}
}

func (v *raceVisitor) inspectFunctionCall(fun ast.Expr, writes map[types.Object]token.Pos) {
	switch callee := fun.(type) {
	case *ast.Ident:
		if obj, ok := v.info.Uses[callee].(*types.Func); ok {
			v.inspectFuncBody(obj, writes)
		}
	case *ast.SelectorExpr:
		if v.info != nil && v.info.Selections != nil {
			if sel := v.info.Selections[callee]; sel != nil {
				if fn, ok := sel.Obj().(*types.Func); ok {
					v.inspectFuncBody(fn, writes)
					return
				}
			}
		}
		if ident := callee.Sel; ident != nil {
			if obj, ok := v.info.Uses[ident].(*types.Func); ok {
				v.inspectFuncBody(obj, writes)
			}
		}
	}
}

func (v *raceVisitor) inspectFuncBody(fn *types.Func, writes map[types.Object]token.Pos) {
	decl, ok := v.funcDecls[fn]
	if !ok || decl.Body == nil {
		return
	}

	ast.Inspect(decl.Body, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range stmt.Lhs {
				v.recordWrite(lhs, writes)
			}
		case *ast.IncDecStmt:
			v.recordWrite(stmt.X, writes)
		}
		return true
	})
}

func (v *raceVisitor) recordWrite(expr ast.Expr, writes map[types.Object]token.Pos) {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return
	}

	obj := v.info.Defs[ident]
	if obj == nil {
		obj = v.info.Uses[ident]
	}
	if obj == nil {
		return
	}

	if pkgVar(obj) {
		if _, exists := writes[obj]; !exists {
			writes[obj] = ident.Pos()
		}
	}
}

func pkgVar(obj types.Object) bool {
	_, ok := obj.(*types.Var)
	if !ok {
		return false
	}
	parent := obj.Parent()
	if parent == nil {
		return false
	}
	pkg := obj.Pkg()
	if pkg == nil {
		return false
	}
	return parent == pkg.Scope()
}
