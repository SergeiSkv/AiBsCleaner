package analyzer

import (
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type TestCoverageAnalyzer struct{}

func NewTestCoverageAnalyzer() Analyzer {
	return &TestCoverageAnalyzer{}
}

func (tca *TestCoverageAnalyzer) Name() string {
	return "Test Coverage Analysis"
}

func (tca *TestCoverageAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	return nil
}
