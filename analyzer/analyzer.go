package analyzer

import (
	"go/ast"
	"go/token"
)

func Analyze(filename string, file *ast.File, fset *token.FileSet, projectPath string) []Issue {
	var issues []Issue

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
		NewAPIMisuseAnalyzer(),  // Детектит API ошибки типа pprof.StartCPUProfile(nil)
		NewAIBullshitAnalyzer(), // Главный анализатор AI bullshit кода

		// New analyzers for concurrent and error handling
		NewContextAnalyzer(),       // Context misuse and leaks
		NewChannelAnalyzer(),       // Channel deadlocks and misuse
		NewRaceConditionAnalyzer(), // Race conditions detection
		NewErrorHandlingAnalyzer(), // Error handling best practices
		NewHTTPClientAnalyzer(),    // HTTP client configuration issues

		// Security and dependency analyzers
		NewPrivacyAnalyzer(),               // Privacy and security issues
		NewDependencyAnalyzer(projectPath), // Dependency health and vulnerabilities
	}

	for _, analyzer := range analyzers {
		issues = append(issues, analyzer.Analyze(filename, file, fset)...)
	}

	return issues
}
