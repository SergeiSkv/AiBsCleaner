package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type LoopAnalyzer struct{}

func NewLoopAnalyzer() Analyzer {
	return &LoopAnalyzer{}
}

func (la *LoopAnalyzer) Name() string {
	return "Loop Performance"
}

func (la *LoopAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	visitor := &loopVisitor{
		fset:     fset,
		filename: filename,
		issues:   make([]*models.Issue, 0, 8),
	}

	ast.Walk(visitor, file)
	return visitor.issues
}

type loopVisitor struct {
	fset      *token.FileSet
	filename  string
	loopDepth int
	issues    []*models.Issue
}

func (v *loopVisitor) Visit(node ast.Node) ast.Visitor {
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
	case *ast.DeferStmt:
		if v.loopDepth > 0 {
			v.addIssue(n.Pos())
		}
	}
	return v
}

func (v *loopVisitor) addIssue(pos token.Pos) {
	position := v.fset.Position(pos)
	v.issues = append(v.issues, &models.Issue{
		File:       v.filename,
		Line:       position.Line,
		Column:     position.Column,
		Position:   position,
		Type:       models.IssueDeferInLoop,
		Severity:   models.SeverityLevelMedium,
		Message:    "Defer inside loop runs every iteration",
		Suggestion: "Move the defer outside loop or replace with explicit cleanup",
	})
}
