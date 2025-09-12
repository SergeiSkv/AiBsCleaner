package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type CPUOptimizationAnalyzer struct{}

func NewCPUOptimizationAnalyzer() Analyzer {
	return &CPUOptimizationAnalyzer{}
}

func (coa *CPUOptimizationAnalyzer) Name() string {
	return "CPUOptimizationAnalyzer"
}

func (coa *CPUOptimizationAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	issues := make([]*models.Issue, 0, 8)

	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			if fn.Body != nil {
				issues = append(issues, checkLenInLoop(fn.Body, fset)...)
			}
		}
		return true
	})

	return issues
}

func checkLenInLoop(body *ast.BlockStmt, fset *token.FileSet) []*models.Issue {
	issues := make([]*models.Issue, 0, 2)

	ast.Inspect(body, func(n ast.Node) bool {
		if forStmt, ok := n.(*ast.ForStmt); ok {
			if forStmt.Cond != nil && containsLenCall(forStmt.Cond) {
				pos := fset.Position(forStmt.Cond.Pos())
				issues = append(issues, &models.Issue{
					File:       pos.Filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       models.IssueCPUIntensive,
					Severity:   models.SeverityLevelLow,
					Message:    "len() computed in loop condition; consider hoisting",
					Suggestion: "Cache len(collection) in a variable before the loop",
				})
			}
		}
		return true
	})

	return issues
}

func containsLenCall(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "len" {
			found = true
			return false
		}
		return true
	})
	return found
}
