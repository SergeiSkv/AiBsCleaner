package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestRegexAnalyzer(t *testing.T) {
	code := `package main
import "regexp"
func run(patterns []string) {
    for _, p := range patterns {
        _ = regexp.MustCompile(p)
    }
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	analyzer := NewRegexAnalyzer()
	issues := analyzer.Analyze(file, fset)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
}
