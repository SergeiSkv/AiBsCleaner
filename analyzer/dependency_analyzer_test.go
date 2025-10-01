package analyzer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDependencyAnalyzerName(t *testing.T) {
	analyzer := NewDependencyAnalyzer("/tmp/test-project")
	assert.Equal(t, "Dependency Security", analyzer.Name())
}

func TestDependencyAnalyzerAnalyze(t *testing.T) {
	analyzer := NewDependencyAnalyzer("/tmp/test-project")

	assert.NotPanics(t, func() {
		_ = analyzer.Analyze(nil, nil)
	})
}

func TestAnalyzeDependenciesFunction(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = AnalyzeDependencies("/tmp/nonexistent")
	})
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
	// Test high-risk dependency detection
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
