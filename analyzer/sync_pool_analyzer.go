package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type SyncPoolAnalyzer struct{}

func NewSyncPoolAnalyzer() Analyzer {
	return &SyncPoolAnalyzer{}
}

func (spa *SyncPoolAnalyzer) Name() string {
	return "SyncPoolAnalyzer"
}

func (spa *SyncPoolAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
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

	visitor := &poolVisitor{
		fset:     fset,
		filename: filename,
		info:     info,
		issues:   make([]*models.Issue, 0, 4),
	}

	ast.Walk(visitor, file)
	return visitor.issues
}

type poolVisitor struct {
	fset     *token.FileSet
	filename string
	info     *types.Info
	issues   []*models.Issue
}

func (v *poolVisitor) Visit(node ast.Node) ast.Visitor {
	fn, ok := node.(*ast.FuncDecl)
	if !ok {
		return v
	}

	v.inspectFunction(fn)
	return nil
}

func (v *poolVisitor) inspectFunction(fn *ast.FuncDecl) {
	gets := make(map[string]int)
	puts := make(map[string]int)

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}

		if !isSyncPoolReceiver(v.info.TypeOf(sel.X)) {
			return true
		}

		switch sel.Sel.Name {
		case "Get":
			gets[ident.Name]++
		case "Put":
			puts[ident.Name]++
		}
		return true
	})

	for name, count := range gets {
		if count > puts[name] {
			pos := v.fset.Position(fn.Pos())
			v.issues = append(v.issues, &models.Issue{
				File:       v.filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       models.IssueSyncPoolOpportunity,
				Severity:   models.SeverityLevelMedium,
				Message:    "sync.Pool.Get without corresponding Put",
				Suggestion: "Ensure objects taken from " + name + " are returned with defer",
			})
		}
	}
}

func isSyncPoolReceiver(t types.Type) bool {
	if t == nil {
		return false
	}
	if pointer, ok := t.(*types.Pointer); ok {
		t = pointer.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	return obj.Pkg().Path() == pkgSync && obj.Name() == "Pool"
}
