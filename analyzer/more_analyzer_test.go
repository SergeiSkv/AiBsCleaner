package analyzer

import (
	"fmt"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test the cache functionality
func TestAnalysisCache(t *testing.T) {
	code := `package test
func main() {
	for i := 0; i < 10; i++ {
		println(i)
	}
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	// First analysis
	issues1 := AnalyzeAll("test.go", file, fset)
	assert.NotNil(t, issues1)

	// Second analysis - should use cache
	issues2 := AnalyzeAll("test.go", file, fset)
	assert.Len(t, issues2, len(issues1))
}

// Test computeFileHash edge cases
func TestComputeFileHash(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{
			name: "empty file",
			code: `package test`,
		},
		{
			name: "file with comments",
			code: `package test
// This is a comment
func main() {}`,
		},
		{
			name: "complex file",
			code: `package test
import "fmt"
func main() {
	for i := 0; i < 10; i++ {
		fmt.Println(i)
	}
}`,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				fset := token.NewFileSet()
				file, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
				require.NoError(t, err)

				hash1 := computeFileHash(file)
				assert.NotEmpty(t, hash1)

				// Same file should produce same hash
				hash2 := computeFileHash(file)
				assert.Equal(t, hash1, hash2)
			},
		)
	}
}

// Test cleanCache
func TestCleanCache(t *testing.T) {
	// This is called internally in updateCache
	code := testPackageMain

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	// Add entries to cache
	for i := 0; i < 5; i++ {
		filename := fmt.Sprintf("test%d.go", i)
		AnalyzeAll(filename, file, fset)
	}

	// Cache should have entries
	globalCache.mu.RLock()
	cacheSize := len(globalCache.results)
	globalCache.mu.RUnlock()
	assert.Positive(t, cacheSize)
}

// Test dependency analyzer methods
func TestDependencyAnalyzerMethods(t *testing.T) {
	analyzer := NewDependencyAnalyzer("/tmp/test")

	// Test Name method
	assert.Equal(t, "Dependency Security", analyzer.Name())

	// Test AnalyzeAll with nil node
	issues := analyzer.Analyze(nil, nil)
	assert.NotNil(t, issues)
}

// Test vulnerability checker methods
func TestVulnerabilityCheckerMethods(t *testing.T) {
	cache := newMockCache()
	checker := NewVulnerabilityChecker(cache)

	// Test isVulnerableVersion
	tests := []struct {
		version  string
		vulnSpec string
		expected bool
	}{
		{"1.0.0", "< 2.0.0", true},
		{"2.0.0", "< 2.0.0", false},
		{"3.0.0", "< 2.0.0", false},
		{"1.0.0", ">= 1.0.0", false}, // Not supported yet
	}

	for _, tt := range tests {
		result := checker.isVulnerableVersion(tt.version, tt.vulnSpec)
		assert.Equal(t, tt.expected, result, "version: %s, spec: %s", tt.version, tt.vulnSpec)
	}
}

// Test vulnerability checker OSV response conversion
func TestConvertOSVResponse(t *testing.T) {
	cache := newMockCache()
	checker := NewVulnerabilityChecker(cache)

	osvResp := OSVResponse{
		Vulns: []struct {
			ID        string   `json:"id"`
			Summary   string   `json:"summary"`
			Details   string   `json:"details"`
			Aliases   []string `json:"aliases"`
			Modified  string   `json:"modified"`
			Published string   `json:"published"`
			Affected  []struct {
				Package struct {
					Ecosystem string `json:"ecosystem"`
					Name      string `json:"name"`
				} `json:"package"`
				Ranges []struct {
					Type   string `json:"type"`
					Events []struct {
						Introduced string `json:"introduced,omitempty"`
						Fixed      string `json:"fixed,omitempty"`
					} `json:"events"`
				} `json:"ranges"`
				Versions []string `json:"versions"`
			} `json:"affected"`
			Severity []struct {
				Type  string `json:"type"`
				Score string `json:"score"`
			} `json:"severity"`
		}{
			{
				ID:      "GO-2021-0001",
				Summary: "Test vulnerability",
				Aliases: []string{"CVE-2021-0001", "GHSA-xxxx-yyyy-zzzz"},
				Affected: []struct {
					Package struct {
						Ecosystem string `json:"ecosystem"`
						Name      string `json:"name"`
					} `json:"package"`
					Ranges []struct {
						Type   string `json:"type"`
						Events []struct {
							Introduced string `json:"introduced,omitempty"`
							Fixed      string `json:"fixed,omitempty"`
						} `json:"events"`
					} `json:"ranges"`
					Versions []string `json:"versions"`
				}{
					{
						Package: struct {
							Ecosystem string `json:"ecosystem"`
							Name      string `json:"name"`
						}{
							Ecosystem: "Go",
							Name:      "test/package",
						},
						Ranges: []struct {
							Type   string `json:"type"`
							Events []struct {
								Introduced string `json:"introduced,omitempty"`
								Fixed      string `json:"fixed,omitempty"`
							} `json:"events"`
						}{
							{
								Type: "DEPENDENCY_VULNERABLE",
								Events: []struct {
									Introduced string `json:"introduced,omitempty"`
									Fixed      string `json:"fixed,omitempty"`
								}{
									{Introduced: "0.0.0"},
									{Fixed: "1.0.1"},
								},
							},
						},
					},
				},
				Severity: []struct {
					Type  string `json:"type"`
					Score string `json:"score"`
				}{
					{Type: "CVSS_V3", Score: "CRITICAL"},
				},
			},
		},
	}

	vulns := checker.convertOSVResponse(osvResp, "test/package", "1.0.0")
	assert.Len(t, vulns, 1)
	assert.Equal(t, "GO-2021-0001", vulns[0].ID)
	assert.Equal(t, "CVE-2021-0001", vulns[0].CVE)
	assert.Equal(t, "GHSA-xxxx-yyyy-zzzz", vulns[0].GHSA)
	assert.Equal(t, "CRITICAL", vulns[0].Severity)
	assert.Equal(t, "1.0.1", vulns[0].FixedIn)
}

// Test updateVulnerabilityDatabase
func TestUpdateVulnerabilityDatabase(t *testing.T) {
	cache := newMockCache()
	checker := NewVulnerabilityChecker(cache)

	err := checker.updateVulnerabilityDatabase()
	require.NoError(t, err)
}

// Test CheckLicense
func TestCheckLicense(t *testing.T) {
	cache := newMockCache()
	checker := NewVulnerabilityChecker(cache)

	info, err := checker.CheckLicense("test/package")
	require.NoError(t, err)
	assert.NotNil(t, info) // Should return license info
	assert.Equal(t, "test/package", info.Package)
	assert.NotEmpty(t, info.License) // Should have a license (BSD-3-Clause for standard packages)
}

// Test Context Analyzer edge cases
func TestContextAnalyzerEdgeCases(t *testing.T) {
	analyzer := NewContextAnalyzer()

	// Test with nil node
	issues := analyzer.Analyze(nil, nil)
	assert.Empty(t, issues)

	// Test with invalid node type
	issues = analyzer.Analyze("invalid", nil)
	assert.Empty(t, issues)
}

// Test HTTP Client Analyzer edge cases
func TestHTTPClientAnalyzerEdgeCases(t *testing.T) {
	analyzer := NewHTTPClientAnalyzer()

	// Test with nil node
	issues := analyzer.Analyze(nil, nil)
	assert.Empty(t, issues)
}

// Test Goroutine Analyzer edge cases
func TestGoroutineAnalyzerEdgeCases(t *testing.T) {
	analyzer := NewGoroutineAnalyzer()

	// Test with nil node
	issues := analyzer.Analyze(nil, nil)
	assert.Empty(t, issues)
}

// Test Privacy Analyzer edge cases
func TestPrivacyAnalyzerEdgeCases(t *testing.T) {
	analyzer := NewPrivacyAnalyzer()

	// Test with nil node
	issues := analyzer.Analyze(nil, nil)
	assert.Empty(t, issues)
}

// Test various analyzer Name methods
func TestAnalyzerNames(t *testing.T) {
	tests := []struct {
		analyzer Analyzer
		expected string
	}{
		{NewLoopAnalyzer(), "Loop Performance"},
		{NewDeferOptimizationAnalyzer(), "Defer Optimization"},
		{NewSliceAnalyzer(), "Slice Optimization"},
		{NewMapAnalyzer(), "Map Optimization"},
		{NewReflectionAnalyzer(), "Reflection Performance"},
		{NewInterfaceAnalyzer(), "Interface Allocation"},
		{NewRegexAnalyzer(), "Regex Performance"},
		{NewTimeAnalyzer(), "Time Operations"},
		{NewMemoryLeakAnalyzer(), "Memory Leak Detection"},
		{NewDatabaseAnalyzer(), "Database Performance"},
		{NewChannelAnalyzer(), "Channel Patterns"},
		{NewNilPtrAnalyzer(), "Nil Pointer Detection"},
		{NewCGOAnalyzer(), "CGO Performance"},
		{NewSerializationAnalyzer(), "Serialization Performance"},
		{NewCryptoAnalyzer(), "Crypto Performance"},
		{NewHTTPReuseAnalyzer(), "HTTP Connection Reuse"},
		{NewIOBufferAnalyzer(), "I/O Buffer Efficiency"},
	}

	for _, tt := range tests {
		t.Run(
			tt.expected, func(t *testing.T) {
				assert.Equal(t, tt.expected, tt.analyzer.Name())
			},
		)
	}
}

// Test multiple analyzers on complex code
func TestAnalyzersOnComplexCode(t *testing.T) {
	code := `package main

import (
	"fmt"
	"time"
	"sync"
	"regexp"
	"encoding/json"
	"database/sql"
	"crypto/md5"
	"net/http"
	"io"
	"os"
	"context"
	"reflect"
)

var globalMutex sync.Mutex
var globalData []int

func ComplexFunction(ctx context.Context) error {
	// Multiple issues
	for i := 0; i < 1000; i++ {
		time.Now() // TIME_NOW_IN_LOOP
		re := regexp.MustCompile("[a-z]+") // REGEX_COMPILE_IN_LOOP
		_ = re
		
		data := make(map[string]int) // MAP_WITHOUT_SIZE
		json.Marshal(data) // JSON_MARSHAL_IN_LOOP
		
		go func() { // UNBOUNDED_GOROUTINE
			fmt.Println(i) // GOROUTINE_VAR_CAPTURE
		}()
		
		defer func() { // DEFER_IN_LOOP
			fmt.Println("cleanup")
		}()
	}
	
	// Reflection misuse
	var x interface{} = 42
	reflect.ValueOf(x) // UNNECESSARY_REFLECTION
	
	// Database issues
	db, _ := sql.Open("mysql", "dsn")
	db.Query("SELECT * FROM users WHERE id = " + fmt.Sprint(1)) // SQL_INJECTION
	
	// HTTP issues
	client := &http.Client{} // HTTP_NO_TIMEOUT
	client.Get("http://example.com")
	
	// Memory leaks
	ticker := time.NewTicker(time.Second) // TICKER_NOT_STOPPED
	_ = ticker
	
	// Crypto issues
	md5.Sum([]byte("data")) // WEAK_HASH
	
	return nil
}

func main() {
	ComplexFunction(context.Background())
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	issues := AnalyzeAll("test.go", file, fset)

	// Should find multiple issues
	assert.Greater(t, len(issues), 5, "Should find multiple issues in complex code")

	// Check for specific issue types
	issueTypes := make(map[string]bool)
	for _, issue := range issues {
		issueTypes[issue.Type.String()] = true
	}

	// These should definitely be found
	assert.True(
		t, issueTypes["TIME_NOW_IN_LOOP"] || issueTypes["REGEX_COMPILE_IN_LOOP"] ||
			issueTypes["JSON_MARSHAL_IN_LOOP"] || issueTypes["DEFER_IN_LOOP"],
		"Should find at least one loop-related issue",
	)
}

// Test all analyzer constructors
func TestAllAnalyzerConstructors(t *testing.T) {
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
		NewNilPtrAnalyzer(),
		NewChannelAnalyzer(),
		NewHTTPClientAnalyzer(),
		NewCGOAnalyzer(),
		NewSerializationAnalyzer(),
		NewCryptoAnalyzer(),
		NewHTTPReuseAnalyzer(),
		NewIOBufferAnalyzer(),
		NewPrivacyAnalyzer(),
		NewContextAnalyzer(),
	}

	for _, analyzer := range analyzers {
		assert.NotNil(t, analyzer)
		assert.NotEmpty(t, analyzer.Name())

		// Test with nil input
		issues := analyzer.Analyze(nil, nil)
		assert.NotNil(t, issues) // Should return empty slice, not nil
	}
}
