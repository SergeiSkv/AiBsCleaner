package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestTimeAnalyzer(t *testing.T) {
	code := `package main
import "time"
func run() {
    for i := 0; i < 10; i++ {
        _ = time.Now()
    }
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	analyzer := NewTimeAnalyzer()
	issues := analyzer.Analyze(file, fset)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
}
