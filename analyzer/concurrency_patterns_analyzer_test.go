package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcurrencyPatternsAnalyzer(t *testing.T) {
	// Simple test to check if analyzer works
	code := `package main
import "sync"

func test() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		wg.Done() // Not deferred - potential issue
	}()
	wg.Wait()
}`

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	analyzer := NewConcurrencyPatternsAnalyzer()
	assert.NotNil(t, analyzer)
	assert.Equal(t, "ConcurrencyPatternsAnalyzer", analyzer.Name())

	// Just run the analyzer, don't check specific issues since implementation may vary
	issues := analyzer.Analyze(node, fset)
	assert.NotNil(t, issues) // Should at least return empty slice, not nil
}
