//go:build legacytests
// +build legacytests

package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestTestCoverageAnalyzerNoop(t *testing.T) {
	code := `package main
func DoThing() {}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	analyzer := NewTestCoverageAnalyzer()
	issues := analyzer.Analyze(file, fset)
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %d", len(issues))
	}
}
