package analyzer

// AnalyzerType represents the type of analyzer
type AnalyzerType uint8

const (
	AnalyzerLoop AnalyzerType = iota
	AnalyzerDeferOptimization
	AnalyzerSlice
	AnalyzerMap
	AnalyzerString
	AnalyzerReflection
	AnalyzerInterface
	AnalyzerRegex
	AnalyzerTime
	AnalyzerMemoryLeak
	AnalyzerGCPressure
	AnalyzerSyncPool
	AnalyzerGoroutine
	AnalyzerChannel
	AnalyzerRaceCondition
	AnalyzerConcurrencyPatterns
	AnalyzerHTTPClient
	AnalyzerHTTPReuse
	AnalyzerIOBuffer
	AnalyzerNetworkPatterns
	AnalyzerDatabase
	AnalyzerSerialization
	AnalyzerCrypto
	AnalyzerPrivacy
	AnalyzerNilPtr
	AnalyzerContext
	AnalyzerErrorHandling
	AnalyzerAPIMisuse
	AnalyzerAIBullshit
	AnalyzerCGO
	AnalyzerTestCoverage
	AnalyzerDependency
	AnalyzerCPUOptimization
	AnalyzerTypeMax
)
