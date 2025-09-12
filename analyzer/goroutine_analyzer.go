package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type GoroutineAnalyzer struct{}

func NewGoroutineAnalyzer() Analyzer {
	return &GoroutineAnalyzer{}
}

func (ga *GoroutineAnalyzer) Name() string {
	return "GoroutineAnalyzer"
}

func (ga *GoroutineAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	visitor := &goroutineVisitor{
		fset:     fset,
		filename: filename,
		issues:   make([]*models.Issue, 0, 8),
	}

	ast.Walk(visitor, file)
	return visitor.issues
}

type goroutineVisitor struct {
	fset      *token.FileSet
	filename  string
	loopDepth int
	issues    []*models.Issue
}

func (v *goroutineVisitor) Visit(node ast.Node) ast.Visitor {
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
	case *ast.GoStmt:
		v.inspectGo(n)
	}
	return v
}

func (v *goroutineVisitor) inspectGo(stmt *ast.GoStmt) {
	if stmt == nil {
		return
	}

	if v.loopDepth > 0 {
		pos := v.fset.Position(stmt.Pos())
		v.issues = append(v.issues, &models.Issue{
			File:       v.filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       models.IssueGoroutinePerRequest,
			Severity:   models.SeverityLevelMedium,
			Message:    "Goroutine launched per loop iteration; consider worker pool",
			Suggestion: "Use a semaphore/worker pool to bound goroutine count",
		})
	}

	if capturesRangeVar(stmt) {
		pos := v.fset.Position(stmt.Pos())
		v.issues = append(v.issues, &models.Issue{
			File:       v.filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       models.IssueGoroutineCapturesLoop,
			Severity:   models.SeverityLevelHigh,
			Message:    "Goroutine closes over loop variable",
			Suggestion: "Pass the loop variable as argument to the goroutine",
		})
	}
}

func capturesRangeVar(goStmt *ast.GoStmt) bool {
	if goStmt.Call == nil {
		return false
	}

	seen := false
	ast.Inspect(goStmt.Call, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Obj != nil && ident.Obj.Kind == ast.Var {
			if _, ok := ident.Obj.Decl.(*ast.RangeStmt); ok {
				seen = true
				return false
			}
		}
		return true
	})
	return seen
}
