package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestSyncPoolAnalyzer(t *testing.T) {
	code := `package main
import "sync"
var pool sync.Pool
func run() {
    obj := pool.Get()
    _ = obj
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	analyzer := NewSyncPoolAnalyzer()
	issues := analyzer.Analyze(file, fset)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
}
