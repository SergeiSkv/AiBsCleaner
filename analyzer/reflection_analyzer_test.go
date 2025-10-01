package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestReflectionAnalyzer(t *testing.T) {
	code := `package main
import "reflect"
func run(xs []interface{}) {
    for _, x := range xs {
        _ = reflect.TypeOf(x)
    }
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	analyzer := NewReflectionAnalyzer()
	issues := analyzer.Analyze(file, fset)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
}
