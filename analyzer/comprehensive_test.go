//go:build legacytests
// +build legacytests

package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

// Test cache expiration
func TestCacheExpiration(t *testing.T) {
	// Temporarily set shorter cache expiration
	oldMaxAge := globalCache.maxAge
	globalCache.maxAge = 100 * time.Millisecond
	defer func() { globalCache.maxAge = oldMaxAge }()

	code := testPackageMain

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	// First analysis
	issues1 := AnalyzeAll("cache_test.go", file, fset)

	// Should hit cache
	issues2 := AnalyzeAll("cache_test.go", file, fset)
	assert.Len(t, issues2, len(issues1))

	// Wait for the cache to expire
	time.Sleep(150 * time.Millisecond)

	// Should reanalyze
	issues3 := AnalyzeAll("cache_test.go", file, fset)
	assert.Len(t, issues3, len(issues1))
}

// Test all issue severities
func TestIssueSeverities(t *testing.T) {
	severities := []models.SeverityLevel{
		models.SeverityLevelLow,
		models.SeverityLevelMedium,
		models.SeverityLevelHigh,
	}

	for _, severity := range severities {
		assert.NotEmpty(t, string(severity))
	}
}

// Test issue creation with all fields
func TestIssueCreation(t *testing.T) {
	issue := models.Issue{
		File:       "test.go",
		Line:       10,
		Column:     5,
		Type:       models.IssueMemoryLeak,
		Severity:   models.SeverityLevelHigh,
		Message:    "Test message",
		Suggestion: "Test suggestion",
		Code:       "test code",
		WhyBad:     "Explanation",
		Position:   token.Position{Line: 10, Column: 5},
	}

	assert.Equal(t, "test.go", issue.File)
	assert.Equal(t, 10, issue.Line)
	assert.Equal(t, 5, issue.Column)
	assert.Equal(t, "TEST_ISSUE", issue.Type)
	assert.Equal(t, models.SeverityLevelHigh, issue.Severity)
	assert.Equal(t, "Test message", issue.Message)
	assert.Equal(t, "Test suggestion", issue.Suggestion)
	assert.Equal(t, "test code", issue.Code)
	assert.Equal(t, "Explanation", issue.WhyBad)
}

// Test all analyzer edge cases
func TestAnalyzerEdgeCases(t *testing.T) {
	analyzers := []struct {
		name     string
		analyzer Analyzer
	}{
		{"Loop", NewLoopAnalyzer()},
		{"Defer", NewDeferOptimizationAnalyzer()},
		{"Slice", NewSliceAnalyzer()},
		{"Map", NewMapAnalyzer()},
		{"Reflection", NewReflectionAnalyzer()},
		{"Interface", NewInterfaceAnalyzer()},
		{"Regex", NewRegexAnalyzer()},
		{"Time", NewTimeAnalyzer()},
		{"MemoryLeak", NewMemoryLeakAnalyzer()},
		{"Database", NewDatabaseAnalyzer()},
		{"APIMisuse", NewAPIMisuseAnalyzer()},
		{"AIBullshit", NewAIBullshitAnalyzer()},
		{"Goroutine", NewGoroutineAnalyzer()},
		{"Channel", NewChannelAnalyzer()},
		{"HTTPClient", NewHTTPClientAnalyzer()},
		{"CGO", NewCGOAnalyzer()},
		{"Serialization", NewSerializationAnalyzer()},
		{"Crypto", NewCryptoAnalyzer()},
		{"HTTPReuse", NewHTTPReuseAnalyzer()},
		{"IOBuffer", NewIOBufferAnalyzer()},
		{"Privacy", NewPrivacyAnalyzer()},
		{"Context", NewContextAnalyzer()},
		{"Dependency", NewDependencyAnalyzer("/tmp/test")},
	}

	for _, tt := range analyzers {
		t.Run(
			tt.name, func(t *testing.T) {
				// Test with various invalid inputs
				issues := tt.analyzer.Analyze(nil, nil)
				assert.NotNil(t, issues)

				issues = tt.analyzer.Analyze("invalid", nil)
				assert.NotNil(t, issues)

				// Test with an empty file
				emptyCode := `package test`
				fset := token.NewFileSet()
				file, err := parser.ParseFile(fset, "test.go", emptyCode, parser.ParseComments)
				require.NoError(t, err)

				issues = tt.analyzer.Analyze(file, fset)
				assert.NotNil(t, issues)

				// Test Name method
				name := tt.analyzer.Name()
				assert.NotEmpty(t, name)
				assert.NotContains(t, name, "nil")
			},
		)
	}
}

// Test WalkWithContext with complex scenarios
func TestWalkWithContextComplex(t *testing.T) {
	code := `package main

import "fmt"

func outer() {
	for i := 0; i < 10; i++ {
		func() {
			for j := 0; j < 5; j++ {
				fmt.Println(i, j)
			}
		}()
	}
}

func nested() {
	for a := 0; a < 3; a++ {
		for b := 0; b < 4; b++ {
			for c := 0; c < 5; c++ {
				println(a, b, c)
			}
		}
	}
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	var maxLoopDepth int
	var foundTripleNested bool
	var funcNames []string

	WalkWithContext(
		file, func(node ast.Node, ctx *AnalysisContext) bool {
			if ctx.LoopDepth > maxLoopDepth {
				maxLoopDepth = ctx.LoopDepth
			}

			if ctx.LoopDepth >= 3 {
				foundTripleNested = true
			}

			if ctx.CurrentFunc != "" && !contains(funcNames, ctx.CurrentFunc) {
				funcNames = append(funcNames, ctx.CurrentFunc)
			}

			return true
		},
	)

	assert.GreaterOrEqual(t, maxLoopDepth, 2, "Should detect at least double nested loops")
	assert.True(t, foundTripleNested, "Should detect triple nested loops")
	assert.Contains(t, funcNames, "outer")
	assert.Contains(t, funcNames, "nested")
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Test specific analyzer patterns
func TestSpecificAnalyzerPatterns(t *testing.T) {
	t.Run(
		"DeferOptimization", func(t *testing.T) {
			code := `package main
import "sync"

func test1() {
	var mu sync.Mutex
	mu.Lock()
	mu.Unlock() // MISSING_DEFER
}

func test2() {
	defer println("deferred") // UNNECESSARY_DEFER (simple op)
}

func test3() {
	for i := 0; i < 10; i++ {
		defer println(i) // DEFER_IN_LOOP
	}
}`

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
			require.NoError(t, err)

			analyzer := NewDeferOptimizationAnalyzer()
			issues := analyzer.Analyze(file, fset)
			assert.NotEmpty(t, issues)

			// Check for specific issue types
			issueTypes := make(map[string]bool)
			for _, issue := range issues {
				issueTypes[issue.Type.String()] = true
			}

			assert.True(
				t, issueTypes["MISSING_DEFER"] || issueTypes["DEFER_IN_LOOP"] ||
					issueTypes["UNNECESSARY_DEFER"], "Should find defer-related issues",
			)
		},
	)

	t.Run(
		"CryptoAnalyzer", func(t *testing.T) {
			code := `package main
import (
	"crypto/md5"
	"crypto/sha1"
	"math/rand"
)

func insecure() {
	md5.Sum([]byte("data")) // WEAK_HASH
	sha1.Sum([]byte("data")) // WEAK_HASH
	rand.Intn(100) // MATH_RAND_FOR_SECURITY
}`

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
			require.NoError(t, err)

			analyzer := NewCryptoAnalyzer()
			issues := analyzer.Analyze(file, fset)
			assert.NotEmpty(t, issues)

			// Should find weak crypto issues
			hasWeakCrypto := false
			for _, issue := range issues {
				if strings.Contains(issue.Type.String(), "WEAK") || strings.Contains(issue.Type.String(), "MATH_RAND") {
					hasWeakCrypto = true
					break
				}
			}
			assert.True(t, hasWeakCrypto, "Should detect weak cryptography")
		},
	)
}

// Test AnalysisContext initialization
func TestAnalysisContextInit(t *testing.T) {
	ctx := &AnalysisContext{
		Filename:    "test.go",
		InLoop:      false,
		LoopDepth:   0,
		CurrentFunc: "",
		Imports:     make(map[string]bool),
		FuncDecls:   make(map[string]*ast.FuncDecl),
		TypeDecls:   make(map[string]*ast.TypeSpec),
		NodeCount:   0,
		StartTime:   time.Now(),
	}

	assert.Equal(t, "test.go", ctx.Filename)
	assert.False(t, ctx.InLoop)
	assert.Equal(t, 0, ctx.LoopDepth)
	assert.Empty(t, ctx.CurrentFunc)
	assert.NotNil(t, ctx.Imports)
	assert.NotNil(t, ctx.FuncDecls)
	assert.NotNil(t, ctx.TypeDecls)
}

// Test nil handling in all analyzers
func TestAnalyzersNilHandling(t *testing.T) {
	analyzers := []Analyzer{
		NewLoopAnalyzer(),
		NewDeferOptimizationAnalyzer(),
		NewSliceAnalyzer(),
		NewMapAnalyzer(),
		NewReflectionAnalyzer(),
		NewInterfaceAnalyzer(),
		NewRegexAnalyzer(),
		NewTimeAnalyzer(),
		NewMemoryLeakAnalyzer(),
		NewDatabaseAnalyzer(),
		NewAPIMisuseAnalyzer(),
		NewAIBullshitAnalyzer(),
		NewGoroutineAnalyzer(),
		NewChannelAnalyzer(),
		NewHTTPClientAnalyzer(),
		NewCGOAnalyzer(),
		NewSerializationAnalyzer(),
		NewCryptoAnalyzer(),
		NewHTTPReuseAnalyzer(),
		NewIOBufferAnalyzer(),
		NewPrivacyAnalyzer(),
		NewContextAnalyzer(),
		NewDependencyAnalyzer("/tmp/test"),
	}

	for _, analyzer := range analyzers {
		t.Run(
			analyzer.Name(), func(t *testing.T) {
				// Should not panic with a nil file set
				issues := analyzer.Analyze(nil, nil)
				assert.NotNil(t, issues)

				// Should not panic with nil AST node
				fset := token.NewFileSet()
				issues = analyzer.Analyze(nil, fset)
				assert.NotNil(t, issues)
			},
		)
	}
}

// Test performance optimization analyzers
func TestPerformanceOptimizationAnalyzers(t *testing.T) {
	code := `package main

import (
	"fmt"
	"strings"
	"bytes"
)

func stringConcat() string {
	result := ""
	for i := 0; i < 1000; i++ {
		result += fmt.Sprint(i) // STRING_CONCAT_IN_LOOP
	}
	return result
}

func betterConcat() string {
	var builder strings.Builder
	for i := 0; i < 1000; i++ {
		builder.WriteString(fmt.Sprint(i))
	}
	return builder.String()
}

func bufferUsage() {
	var buf bytes.Buffer
	for i := 0; i < 100; i++ {
		buf.WriteString("data")
	}
}

func sliceAppend() {
	var data []int
	for i := 0; i < 1000; i++ {
		data = append(data, i) // SLICE_APPEND_IN_LOOP without preallocation
	}
}

func mapUsage() {
	m := make(map[string]int) // Could use size hint if known
	for i := 0; i < 100; i++ {
		m[fmt.Sprint(i)] = i
	}
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	issues := AnalyzeAll("test.go", file, fset)

	// Should find performance issues
	assert.NotEmpty(t, issues, "Should find performance issues")

	// Check for specific performance patterns
	var foundStringConcat, foundSliceAppend bool
	for _, issue := range issues {
		if strings.Contains(issue.Type.String(), "STRING") || strings.Contains(issue.Message, "string") {
			foundStringConcat = true
		}
		if strings.Contains(issue.Type.String(), "SLICE") || strings.Contains(issue.Message, "slice") {
			foundSliceAppend = true
		}
	}

	assert.True(
		t, foundStringConcat || foundSliceAppend,
		"Should detect string concatenation or slice append issues",
	)
}
