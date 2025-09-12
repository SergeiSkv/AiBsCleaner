package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

func TestRaceConditionAnalyzer_FlagsGlobalWrite(t *testing.T) {
	code := `package main
var counter int
func inc() { counter++ }
func run() { go inc() }`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	analyzer := NewRaceConditionAnalyzer()
	issues := analyzer.Analyze(file, fset)
	if len(issues) == 0 {
		t.Fatalf("expected at least one issue")
	}

	if issues[0].Type != models.IssueRaceCondition {
		t.Fatalf("expected IssueRaceCondition, got %s", issues[0].Type)
	}
}
