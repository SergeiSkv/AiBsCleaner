package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestSerializationAnalyzer(t *testing.T) {
	code := `package main
import "encoding/json"
func run(items []interface{}) {
    for _, item := range items {
        _, _ = json.Marshal(item)
    }
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	analyzer := NewSerializationAnalyzer()
	issues := analyzer.Analyze(file, fset)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
}
