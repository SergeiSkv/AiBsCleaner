package analyzer

import (
	"go/ast"
	"go/token"
)

func Analyze(filename string, file *ast.File, fset *token.FileSet, projectPath string) []Issue {
	var issues []Issue

	// Per-file analyzers - focused on performance, not duplicating standard linters
	analyzers := []Analyzer{
		// Performance analyzers (unique to this tool)
		NewLoopAnalyzer(),              // Loop performance issues
		NewDeferOptimizationAnalyzer(), // Defer optimization opportunities
		NewSliceAnalyzer(),             // Slice preallocation and append issues
		NewMapAnalyzer(),               // Map preallocation
		NewReflectionAnalyzer(),        // Reflection performance
		NewInterfaceAnalyzer(),         // Interface allocation overhead
		NewRegexAnalyzer(),             // Regex compilation in loops
		NewTimeAnalyzer(),              // Time.Format in loops
		NewMemoryLeakAnalyzer(),        // Memory leak patterns
		NewDatabaseAnalyzer(),          // Database query issues

		// Specialized analyzers (not covered by standard linters)
		NewAPIMisuseAnalyzer(),  // API misuse like pprof.StartCPUProfile(nil)
		NewAIBullshitAnalyzer(), // AI-generated over-engineering
		NewGoroutineAnalyzer(),  // Goroutine performance issues (not leaks)
		NewNilPtrAnalyzer(),     // Nil pointer patterns (not error checks)
		NewChannelAnalyzer(),    // Channel buffer sizes and patterns
		NewHTTPClientAnalyzer(), // HTTP client performance configuration

		// Security/privacy (specialized)
		NewPrivacyAnalyzer(), // Privacy and data exposure issues

	}

	for _, analyzer := range analyzers {
		issues = append(issues, analyzer.Analyze(filename, file, fset)...)
	}

	return issues
}

// AnalyzeDependencies runs dependency analysis once for the entire project
func AnalyzeDependencies(projectPath string) []Issue {
	analyzer := NewDependencyAnalyzer(projectPath)
	// Use an empty filename since this is project-level analysis
	return analyzer.Analyze("go.mod", nil, nil)
}
