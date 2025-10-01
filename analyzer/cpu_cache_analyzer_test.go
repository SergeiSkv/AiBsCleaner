//go:build legacytests
// +build legacytests

package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCPUCacheAnalyzerFalseSharing(t *testing.T) {
	code := `package test

import (
    "sync"
    "sync/atomic"
)

type worker struct {
    lock sync.Mutex
    flag atomic.Bool
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	analyzer := NewCPUCacheAnalyzer()
	issues := analyzer.Analyze(file, fset)

	if len(issues) == 0 {
		t.Fatalf("expected false sharing issue, got none")
	}
}

func TestCPUCacheAnalyzerNoIssue(t *testing.T) {
	code := `package test

import "sync"

type metrics struct {
    lock sync.Mutex
    value int
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	analyzer := NewCPUCacheAnalyzer()
	issues := analyzer.Analyze(file, fset)

	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %d", len(issues))
	}
}
