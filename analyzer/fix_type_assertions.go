package analyzer

import (
	"go/ast"
	"go/token"
)

// SafeAnalyze is a helper function to safely cast and analyze AST nodes
func SafeAnalyze(node interface{}, fset *token.FileSet, analyzeFunc func(ast.Node, *token.FileSet) []Issue) []Issue {
	astNode, ok := node.(ast.Node)
	if !ok {
		return []Issue{}
	}
	return analyzeFunc(astNode, fset)
}

// Constants for magic numbers
const (
	MaxFunctionStatements = 50
	MaxFunctionParams     = 5
	MaxNestedLoops       = 3
	MaxDeferPerFunction  = 5
	MaxQueriesPerFunction = 5
	
	// Complexity thresholds
	HighComplexityThreshold   = 3
	MediumComplexityThreshold = 2
	
	// Performance thresholds
	SlowFunctionThresholdMs = 100
	HighMemoryAllocationMB   = 10
	
	// Coverage thresholds
	MinTestCoveragePercent = 80.0
	
	// Search depth limit
	MaxSearchDepth = 10
	
	// Metric thresholds
	MaxCyclomaticComplexity = 10.0
	MaxResponseTimeMs       = 100.0
	MaxMemoryPerRequestKB   = 1024.0
	MaxCPUUsagePercent     = 50.0
	MaxErrorRatePercent    = 0.01
	TechnicalDebtRatioPercent = 5.0
	
	// Confidence scores
	HighConfidence   = 0.95
	MediumConfidence = 0.85
	LowConfidence    = 0.70
)