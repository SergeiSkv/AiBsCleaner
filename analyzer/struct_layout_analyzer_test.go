package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestStructLayoutAnalyzerFlagsPadding(t *testing.T) {
	code := `package main
type S struct {
    A bool
    B int64
    C bool
}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	analyzer := NewStructLayoutAnalyzer()
	issues := analyzer.Analyze(file, fset)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
}
