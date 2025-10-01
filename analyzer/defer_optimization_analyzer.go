package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type DeferOptimizationAnalyzer struct{}

func NewDeferOptimizationAnalyzer() Analyzer {
	return &DeferOptimizationAnalyzer{}
}

func (d *DeferOptimizationAnalyzer) Name() string {
	return "Defer Optimization"
}

func (d *DeferOptimizationAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	issues := make([]*models.Issue, 0, 8)

	ast.Inspect(file, func(n ast.Node) bool {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			issues = append(issues, analyzeBlock(fn.Body, filename, fset, false)...)
		case *ast.FuncLit:
			issues = append(issues, analyzeBlock(fn.Body, filename, fset, false)...)
		}
		return true
	})

	return issues
}

func analyzeBlock(body *ast.BlockStmt, filename string, fset *token.FileSet, inLoop bool) []*models.Issue {
	if body == nil {
		return nil
	}

	issues := make([]*models.Issue, 0, 4)
	callCounts := make(map[string]int)

	for i, stmt := range body.List {
		switch s := stmt.(type) {
		case *ast.DeferStmt:
			issues = append(issues, analyzeDeferStmt(s, filename, fset, inLoop, i == len(body.List)-1, len(body.List), callCounts)...)
		case *ast.ForStmt:
			issues = append(issues, analyzeBlock(s.Body, filename, fset, true)...)
		case *ast.RangeStmt:
			issues = append(issues, analyzeBlock(s.Body, filename, fset, true)...)
		case *ast.IfStmt:
			issues = append(issues, analyzeBlock(s.Body, filename, fset, inLoop)...)
			if els, ok := s.Else.(*ast.BlockStmt); ok {
				issues = append(issues, analyzeBlock(els, filename, fset, inLoop)...)
			}
		case *ast.SwitchStmt:
			for _, stmt := range s.Body.List {
				if clause, ok := stmt.(*ast.CaseClause); ok {
					block := &ast.BlockStmt{List: clause.Body}
					issues = append(issues, analyzeBlock(block, filename, fset, inLoop)...)
				}
			}
		}
	}

	for _, count := range callCounts {
		if count > 1 {
			// report once at block start
			pos := fset.Position(body.Pos())
			issues = append(issues, &models.Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       models.IssueMultipleDefers,
				Severity:   models.SeverityLevelLow,
				Message:    "Same defer call appears multiple times",
				Suggestion: "Defer once and reuse the cleanup",
			})
			break
		}
	}

	return issues
}

func analyzeDeferStmt(
	stmt *ast.DeferStmt,
	filename string,
	fset *token.FileSet,
	inLoop bool,
	isLast bool,
	totalStmts int,
	callCounts map[string]int,
) []*models.Issue {
	issues := make([]*models.Issue, 0, 3)
	callName := deferCallName(stmt)
	if callName != "" {
		callCounts[callName]++
	}

	pos := fset.Position(stmt.Pos())
	if inLoop {
		issues = append(issues, &models.Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       models.IssueDeferInLoop,
			Severity:   models.SeverityLevelMedium,
			Message:    "Defer inside loop executes each iteration",
			Suggestion: "Move cleanup outside the loop",
		})
	}

	if isLast {
		issues = append(issues, &models.Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       models.IssueDeferAtEnd,
			Severity:   models.SeverityLevelLow,
			Message:    "Defer at end of block adds unnecessary overhead",
			Suggestion: "Call cleanup directly instead of deferring",
		})
	}

	if totalStmts <= 3 {
		if !requiresDeferForSafety(stmt) {
			issues = append(issues, &models.Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       models.IssueDeferInShortFunc,
				Severity:   models.SeverityLevelLow,
				Message:    "Short function literal uses defer",
				Suggestion: "Invoke cleanup directly when the block is very small",
			})
		}
	}

	return issues
}

func deferCallName(stmt *ast.DeferStmt) string {
	if stmt == nil || stmt.Call == nil {
		return ""
	}
	switch fun := stmt.Call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}

func requiresDeferForSafety(stmt *ast.DeferStmt) bool {
	if stmt == nil || stmt.Call == nil {
		return false
	}

	// Direct recover call is always intentional
	if name := deferCallName(stmt); name == funcRecover {
		return true
	}

	// Inline deferred closures that call recover should not be flagged
	if lit, ok := stmt.Call.Fun.(*ast.FuncLit); ok && lit.Body != nil {
		containsRecover := false
		ast.Inspect(lit.Body, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			if ident.Name == funcRecover {
				containsRecover = true
				return false
			}
			return true
		})
		if containsRecover {
			return true
		}
	}

	return false
}
