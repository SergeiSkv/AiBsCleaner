package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type InterfaceAnalyzer struct{}

func NewInterfaceAnalyzer() Analyzer {
	return &InterfaceAnalyzer{}
}

func (ia *InterfaceAnalyzer) Name() string {
	return "Interface Allocation"
}

func (ia *InterfaceAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	visitor := &interfaceVisitor{
		fset:      fset,
		filename:  filename,
		loopDepth: 0,
		issues:    make([]*models.Issue, 0, 4),
	}

	ast.Walk(visitor, file)
	return visitor.issues
}

type interfaceVisitor struct {
	fset      *token.FileSet
	filename  string
	loopDepth int
	issues    []*models.Issue
}

func (v *interfaceVisitor) Visit(node ast.Node) ast.Visitor {
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
	case *ast.TypeAssertExpr:
		v.checkTypeAssert(n)
	case *ast.CompositeLit:
		v.checkInterfaceLiteral(n)
	}
	return v
}

func (v *interfaceVisitor) checkTypeAssert(assert *ast.TypeAssertExpr) {
	if v.loopDepth == 0 {
		return
	}
	pos := v.fset.Position(assert.Pos())
	v.issues = append(v.issues, &models.Issue{
		File:       v.filename,
		Line:       pos.Line,
		Column:     pos.Column,
		Position:   pos,
		Type:       models.IssueInterfaceAllocation,
		Severity:   models.SeverityLevelLow,
		Message:    "Type assertion in loop causes interface allocations",
		Suggestion: "Cache type assertion result outside loop",
	})
}

func (v *interfaceVisitor) checkInterfaceLiteral(lit *ast.CompositeLit) {
	_, ok := lit.Type.(*ast.InterfaceType)
	if !ok {
		return
	}
	pos := v.fset.Position(lit.Pos())
	v.issues = append(v.issues, &models.Issue{
		File:       v.filename,
		Line:       pos.Line,
		Column:     pos.Column,
		Position:   pos,
		Type:       models.IssueInterfaceAllocation,
		Severity:   models.SeverityLevelLow,
		Message:    "anonymous interface literal may allocate per use",
		Suggestion: "Define interface type once and reuse",
	})
}
