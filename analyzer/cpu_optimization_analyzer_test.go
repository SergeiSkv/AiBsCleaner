//go:build legacytests
// +build legacytests

package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCPUOptimizationAnalyzer(t *testing.T) {
	code := `package main

func test(data []int) {
	// Potential CPU optimization issues
	for i := 0; i < len(data); i++ {
		if data[i]%2 == 0 {
			data[i] *= 2
		} else {
			data[i] *= 3
		}
	}
}`

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	analyzer := NewCPUOptimizationAnalyzer()
	assert.NotNil(t, analyzer)
	assert.Equal(t, "CPUOptimizationAnalyzer", analyzer.Name())

	// Just run the analyzer
	issues := analyzer.Analyze(node, fset)
	assert.NotNil(t, issues)
}
