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

func TestGCPressureAnalyzer(t *testing.T) {
	// Simple test to check if analyzer works
	code := `package main

func test() {
	// Potential GC pressure issues
	for i := 0; i < 1000000; i++ {
		s := make([]byte, 1024)
		_ = s
	}
	
	// String concatenation in loop
	result := ""
	for i := 0; i < 1000; i++ {
		result += "x"
	}
}`

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	analyzer := NewGCPressureAnalyzer()
	assert.NotNil(t, analyzer)
	assert.Equal(t, "GCPressureAnalyzer", analyzer.Name())

	// Just run the analyzer
	issues := analyzer.Analyze(node, fset)
	assert.NotNil(t, issues)
}
