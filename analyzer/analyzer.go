package analyzer

import (
	"go/ast"
	"go/token"
)

func Analyze(filename string, file *ast.File, fset *token.FileSet) []Issue {
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
	}

	for _, analyzer := range analyzers {
		issues = append(issues, analyzer.Analyze(filename, file, fset)...)
	}

	return issues
}
