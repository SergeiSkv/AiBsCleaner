package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type DependencyAnalyzer struct{}

func NewDependencyAnalyzer(_ string) *DependencyAnalyzer {
	return &DependencyAnalyzer{}
}

func (a *DependencyAnalyzer) Name() string {
	return "Dependency Security"
}

func (a *DependencyAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok || file == nil {
		return []*models.Issue{}
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	issues := make([]*models.Issue, 0, len(file.Imports))

	for _, imp := range file.Imports {
		if imp.Path == nil {
			continue
		}
		path := trimQuote(imp.Path.Value)
		if issue := checkImportPath(path, fset.Position(imp.Pos()), filename); issue != nil {
			issues = append(issues, issue)
		}
	}

	return issues
}

func checkImportPath(path string, pos token.Position, filename string) *models.Issue {
	switch path {
	case "unsafe":
		return &models.Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       models.IssueDependencyUnsafe,
			Severity:   models.SeverityLevelMedium,
			Message:    "Importing unsafe bypasses type safety; audit necessity",
			Suggestion: "Limit unsafe usage to well-reviewed sections only",
		}
	case "C":
		return &models.Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       models.IssueDependencyCGO,
			Severity:   models.SeverityLevelLow,
			Message:    "cgo import increases build complexity and can reduce portability",
			Suggestion: "Evaluate whether pure Go alternatives exist",
		}
	}

	if isCaseMismatchLogrus(path) {
		return &models.Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       models.IssueDependencyVersionConflict,
			Severity:   models.SeverityLevelLow,
			Message:    "Use github.com/sirupsen/logrus (lowercase) to avoid module duplication",
			Suggestion: "Replace import with the canonical lowercase path",
		}
	}

	return nil
}

func trimQuote(s string) string {
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"') {
		return s[1 : len(s)-1]
	}
	return s
}

func isCaseMismatchLogrus(path string) bool {
	return path == "github.com/Sirupsen/logrus"
}
