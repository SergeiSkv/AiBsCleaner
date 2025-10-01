package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type SerializationAnalyzer struct{}

func NewSerializationAnalyzer() Analyzer {
	return &SerializationAnalyzer{}
}

func (sa *SerializationAnalyzer) Name() string {
	return "Serialization Performance"
}

func (sa *SerializationAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	visitor := &serializationVisitor{
		fset:      fset,
		filename:  filename,
		loopDepth: 0,
		issues:    make([]*models.Issue, 0, 8),
	}

	ast.Walk(visitor, file)
	return visitor.issues
}

type serializationVisitor struct {
	fset      *token.FileSet
	filename  string
	loopDepth int
	issues    []*models.Issue
}

func (v *serializationVisitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.ForStmt:
		v.loopDepth++
		if n.Body != nil {
			ast.Walk(v, n.Body)
		}
		v.loopDepth--
		return nil
	case *ast.RangeStmt:
		v.loopDepth++
		if n.Body != nil {
			ast.Walk(v, n.Body)
		}
		v.loopDepth--
		return nil
	case *ast.CallExpr:
		v.inspectCall(n)
	}
	return v
}

func (v *serializationVisitor) inspectCall(call *ast.CallExpr) {
	if v.loopDepth == 0 {
		return
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}

	if pkgIdent.Name == pkgJSON {
		if sel.Sel.Name == methodMarshal || sel.Sel.Name == methodUnmarshal {
			pos := v.fset.Position(call.Pos())
			v.issues = append(v.issues, &models.Issue{
				File:       v.filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       models.IssueSerializationInLoop,
				Severity:   models.SeverityLevelMedium,
				Message:    "json.Marshal/json.Unmarshal inside loop",
				Suggestion: "Move serialization outside loop or reuse encoder",
			})
		}
	}
}
