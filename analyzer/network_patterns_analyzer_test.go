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

func TestNetworkPatternsAnalyzer(t *testing.T) {
	// Simple test to check if analyzer works
	code := `package main
import "net"

func test() {
	conn, _ := net.Dial("tcp", "example.com:80")
	defer conn.Close()
}`

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	analyzer := NewNetworkPatternsAnalyzer()
	assert.NotNil(t, analyzer)
	assert.Equal(t, "NetworkPatternsAnalyzer", analyzer.Name())

	// Just run the analyzer
	issues := analyzer.Analyze(node, fset)
	assert.NotNil(t, issues)
}
