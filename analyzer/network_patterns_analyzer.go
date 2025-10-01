package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type NetworkPatternsAnalyzer struct{}

func NewNetworkPatternsAnalyzer() Analyzer {
	return &NetworkPatternsAnalyzer{}
}

func (npa *NetworkPatternsAnalyzer) Name() string {
	return "NetworkPatternsAnalyzer"
}

func (npa *NetworkPatternsAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	visitor := &networkVisitor{
		fset:      fset,
		filename:  filename,
		loopDepth: 0,
		issues:    make([]*models.Issue, 0, 8),
	}

	ast.Walk(visitor, file)
	return visitor.issues
}

type networkVisitor struct {
	fset      *token.FileSet
	filename  string
	loopDepth int
	issues    []*models.Issue
}

func (v *networkVisitor) Visit(node ast.Node) ast.Visitor {
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

func (v *networkVisitor) inspectCall(call *ast.CallExpr) {
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

	// Flag os network dialing inside loops
	if pkgIdent.Name == "net" || pkgIdent.Name == "http" {
		if sel.Sel.Name == methodDial || sel.Sel.Name == "DialTimeout" || sel.Sel.Name == methodListen || sel.Sel.Name == methodGet || sel.Sel.Name == methodPost {
			pos := v.fset.Position(call.Pos())
			v.issues = append(v.issues, &models.Issue{
				File:       v.filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       models.IssueNetworkInLoop,
				Severity:   models.SeverityLevelMedium,
				Message:    "Network call inside loop can become N+1",
				Suggestion: "Move dial/request outside loop or batch requests",
			})
		}
	}
}
