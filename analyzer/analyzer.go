package analyzer

import (
	"go/ast"
	"go/token"
)

func Analyze(filename string, file *ast.File, fset *token.FileSet, projectPath string) []Issue {
	var issues []Issue

	// Per-file analyzers
	analyzers := []Analyzer{
		NewLoopAnalyzer(),
		NewStringConcatAnalyzer(),
		NewDeferAnalyzer(),
		NewDeferOptimizationAnalyzer(),
		NewSliceAnalyzer(),
		NewMapAnalyzer(),
		NewReflectionAnalyzer(),
		NewGoroutineAnalyzer(),
		NewInterfaceAnalyzer(),
		NewRegexAnalyzer(),
		NewTimeAnalyzer(),
		NewComplexityAnalyzer(),
		NewMemoryLeakAnalyzer(),
		NewTestCoverageAnalyzer(),
		NewDatabaseAnalyzer(),
		NewNilPtrAnalyzer(),
		NewCodeSmellAnalyzer(),
		NewAPIMisuseAnalyzer(),  // Detects API errors like pprof.StartCPUProfile(nil)
		NewAIBullshitAnalyzer(), // Main AI bullshit code analyzer

		// New analyzers for concurrent and error handling
		NewContextAnalyzer(),       // Context misuse and leaks
		NewChannelAnalyzer(),       // Channel deadlocks and misuse
		NewRaceConditionAnalyzer(), // Race conditions detection
		NewErrorHandlingAnalyzer(), // Error handling best practices
		NewHTTPClientAnalyzer(),    // HTTP client configuration issues

		// Security analyzers
		NewPrivacyAnalyzer(), // Privacy and security issues
		// NOTE: DependencyAnalyzer removed from here - should be run once per project!
	}

	for _, analyzer := range analyzers {
		issues = append(issues, analyzer.Analyze(filename, file, fset)...)
	}

	return issues
}

// AnalyzeDependencies runs dependency analysis once for the entire project
func AnalyzeDependencies(projectPath string) []Issue {
	analyzer := NewDependencyAnalyzer(projectPath)
	// Use empty filename since this is project-level analysis
	return analyzer.Analyze("go.mod", nil, nil)
}
