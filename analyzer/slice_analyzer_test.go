package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestSliceAnalyzerAppendInLoop(t *testing.T) {
	code := `package main
func run(xs []int) []int {
    var dst []int
    for _, x := range xs {
        dst = append(dst, x)
    }
    return dst
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	analyzer := NewSliceAnalyzer()
	issues := analyzer.Analyze(file, fset)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
}
