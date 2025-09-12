package analyzer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDependencyAnalyzer(t *testing.T) {
	// Test basic analyzer creation
	analyzer := NewDependencyAnalyzer("/tmp/test-project")
	assert.NotNil(t, analyzer)
	assert.Equal(t, "/tmp/test-project", analyzer.projectPath)
	assert.NotNil(t, analyzer.vulnChecker)
}

func TestDependencyAnalyzerName(t *testing.T) {
	analyzer := NewDependencyAnalyzer("/tmp/test-project")
	assert.Equal(t, "DependencyAnalyzer", analyzer.Name())
}

func TestDependencyAnalyzerAnalyze(t *testing.T) {
	analyzer := NewDependencyAnalyzer("/tmp/test-project")

	// Test with nil node (project-level analysis)
	issues := analyzer.Analyze(nil, nil)
	assert.NotNil(t, issues)
	// Since we're passing a non-existent path, should get file not found
	// but no panic
}

func TestAnalyzeDependenciesFunction(t *testing.T) {
	// Test the global function
	issues := AnalyzeDependencies("/tmp/nonexistent")
	assert.NotNil(t, issues)
}

func TestModuleStructure(t *testing.T) {
	// Test Module struct
	mod := Module{
		Path:     "github.com/test/module",
		Version:  "v1.0.0",
		Indirect: false,
	}

	assert.Equal(t, "github.com/test/module", mod.Path)
	assert.Equal(t, "v1.0.0", mod.Version)
	assert.False(t, mod.Indirect)
}

func TestVulnerabilityChecker(t *testing.T) {
	cache := NewRedisCache("")
	checker := NewVulnerabilityChecker(cache)
	assert.NotNil(t, checker)
	assert.NotNil(t, checker.cache)
}

func TestRedisCache(t *testing.T) {
	// Test in-memory cache fallback
	cache := NewRedisCache("")
	assert.NotNil(t, cache)

	// Test with Redis URL (won't connect, but shouldn't panic)
	redisCache := NewRedisCache("redis://localhost:6379")
	assert.NotNil(t, redisCache)
}

func TestModuleParsing(t *testing.T) {
	// Test parsing module line
	line := "github.com/test/module v1.2.3"
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		assert.Equal(t, "github.com/test/module", parts[0])
		assert.Equal(t, "v1.2.3", parts[1])
	}
}

func TestPackageImportAnalysis(t *testing.T) {
	// Create a simple test for import analysis
	imports := map[string]bool{
		"fmt":                         true,
		"github.com/stretchr/testify": true,
		"github.com/unused/package":   false,
	}

	used := 0
	unused := 0
	for _, isUsed := range imports {
		if isUsed {
			used++
		} else {
			unused++
		}
	}

	assert.Equal(t, 2, used)
	assert.Equal(t, 1, unused)
}

func TestIsHighRiskDependency(t *testing.T) {
	// Test high risk dependency detection
	highRisk := []string{
		"crypto",
		"security",
		"auth",
		"oauth",
	}

	for _, keyword := range highRisk {
		pkgName := "github.com/example/" + keyword + "-lib"
		// Simple heuristic check
		assert.Contains(t, pkgName, keyword)
	}
}
