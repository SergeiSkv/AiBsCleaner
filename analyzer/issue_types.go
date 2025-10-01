package analyzer

// IssueType represents specific issue types as an enum
//
//go:generate go run github.com/dmarkham/enumer@latest -type=IssueType -trimprefix=Iпере
type IssueType uint16

const (
	// Loop issues (0-19)
	IssueNestedLoop IssueType = iota
	IssueAllocInLoop
	IssueStringConcatInLoop
	IssueAppendInLoop
	IssueDeferInLoop
	IssueRegexInLoop
	IssueTimeInLoop
	IssueSQLInLoop
	IssueDNSInLoop
	IssueReflectionInLoop
	IssueCPUIntensiveLoop

	// Memory issues (20-39)
	IssueMemoryLeak IssueType = 20 + iota - 11
	IssueGlobalVar
	IssueLargeAllocation
	IssueHighGCPressure
	IssueFrequentAllocation
	IssueLargeHeapAlloc
	IssuePointerHeavyStruct
	IssueMissingDefer
	IssueMissingClose

	// Slice issues (40-49)
	IssueSliceCapacity IssueType = 40 + iota - 20
	IssueSliceCopy
	IssueSliceAppend
	IssueSliceRangeCopy
	IssueSliceAppendInLoop
	IssueSlicePrealloc

	// Map issues (50-59)
	IssueMapCapacity IssueType = 50 + iota - 26
	IssueMapClear
	IssueMapPrealloc

	// String issues (60-69)
	IssueStringConcat IssueType = 60 + iota - 29
	IssueStringBuilder
	IssueStringInefficient

	// Defer issues (70-79)
	IssueDeferInShortFunc IssueType = 70 + iota - 32
	IssueDeferOverhead
	IssueUnnecessaryDefer
	IssueDeferAtEnd
	IssueMultipleDefers
	IssueDeferInHotPath
	IssueDeferLargeCapture
	IssueUnnecessaryMutexDefer
	IssueMissingDeferUnlock
	IssueMissingDeferClose

	// Concurrency issues (80-109)
	IssueRaceCondition IssueType = 80 + iota - 42
	IssueRaceConditionGlobal
	IssueUnsyncMapAccess
	IssueRaceClosure
	IssueGoroutineLeak
	IssueUnbufferedChannel
	IssueGoroutineOverhead
	IssueSyncMutexValue
	IssueWaitgroupMisuse
	IssueRaceInDefer
	IssueAtomicMisuse
	IssueGoroutineNoRecover
	IssueGoroutineCapturesLoop
	IssueWaitGroupAddInLoop
	IssueWaitGroupWaitBeforeStart
	IssueMutexForReadOnly
	IssueSelectWithSingleCase
	IssueBusyWait
	IssueContextBackgroundInGoroutine
	IssueGoroutinePerRequest
	IssueNoWorkerPool
	IssueUnbufferedSignalChan
	IssueSelectDefault
	IssueChannelSize
	IssueRangeOverChannel
	IssueChannelDeadlock
	IssueChannelMultipleClose
	IssueChannelSendOnClosed

	// Network & HTTP issues (110-129)
	IssueHTTPNoTimeout IssueType = 110 + iota - 59
	IssueHTTPNoClose
	IssueHTTPDefaultClient
	IssueHTTPNoContext
	IssueKeepaliveMissing
	IssueConnectionPool
	IssueNoReuseConnection
	IssueHTTPNoConnectionReuse

	// Database issues (130-139)
	IssueNoPreparedStmt IssueType = 130 + iota - 67
	IssueMissingDBClose
	IssueSQLNPlusOne

	// Interface & Reflection issues (140-149)
	IssueReflection IssueType = 140 + iota - 70
	IssueInterfaceAllocation
	IssueEmptyInterface
	IssueInterfacePollution

	// Time & Regex issues (150-159)
	IssueTimeAfterLeak IssueType = 150 + iota - 74
	IssueTimeFormat
	IssueTimeNowInLoop
	IssueRegexCompile
	IssueRegexCompileInLoop

	// Context issues (160-169)
	IssueContextBackground IssueType = 160 + iota - 79
	IssueContextValue
	IssueMissingContextCancel
	IssueContextLeak
	IssueContextInStruct
	IssueContextNotFirst
	IssueContextMisuse

	// Error handling issues (170-179)
	IssueErrorIgnored IssueType = 170 + iota - 86
	IssueErrorCheckMissing
	IssuePanicRecover
	IssueErrorStringFormat
	IssuePanicRisk
	IssuePanicInLibrary

	// Nil pointer issues (180-189)
	IssueNilCheck IssueType = 180 + iota - 92
	IssueNilReturn
	IssuePotentialNilDeref
	IssuePotentialNilIndex
	IssueRangeOverNil
	IssueNilMethodCall
	IssueUncheckedParam

	// AI Bullshit issues (200-229)
	IssueAIBullshitConcurrency IssueType = 200 + iota - 99
	IssueAIReflectionOverkill
	IssueAIPatternAbuse
	IssueAIEnterpriseHelloWorld
	IssueAICaptainObvious
	IssueAIOverengineeredSimple
	IssueAIGeneratedComment
	IssueAIUnnecessaryComplexity
	IssueAIOverAbstraction
	IssueAIVariable
	IssueAIErrorHandling
	IssueAIStructure
	IssueAIRepetition
	IssueAIFactorySimple
	IssueAIRedundantElse
	IssueAIGoroutineOverkill
	IssueAIUnnecessaryReflection
	IssueAIUnnecessaryInterface

	// GC Pressure issues - continuing from AI issues
	IssueHighGCPressureDetected
	IssueFrequentAllocationDetected
	IssueLargeHeapAllocDetected
	IssuePointerHeavyStructDetected

	// Sync Pool issues
	IssueSyncPoolOpportunity
	IssueSyncPoolPutMissing
	IssueSyncPoolTypeAssert
	IssueSyncPoolMisuse

	// API Misuse issues
	IssueAPIMisuse
	IssueWGMisuse
	IssuePprofInProd
	IssuePprofNilWriter
	IssueDebugInProd
	IssueWaitgroupAddInGoroutine
	IssueContextBackgroundMisuse
	IssueSleepInLoop
	IssueSprintfConcatenation
	IssueLogInHotPath
	IssueRecoverWithoutDefer
	IssueJSONMarshalInLoop
	IssueRegexCompileInFunc
	IssueMutexByValue

	// Privacy issues
	IssuePrivacyHardcodedSecret
	IssuePrivacyAWSKey
	IssuePrivacyJWTToken
	IssuePrivacyEmailPII
	IssuePrivacySSNPII
	IssuePrivacyCreditCardPII
	IssuePrivacyLoggingSensitive
	IssuePrivacyPrintingSensitive
	IssuePrivacyExposedField
	IssuePrivacyUnencryptedDBWrite
	IssuePrivacyDirectInputToDB

	// Dependency issues
	IssueDependencyDeprecated
	IssueDependencyVulnerable
	IssueDependencyOutdated
	IssueDependencyCGO
	IssueDependencyUnsafe
	IssueDependencyInternal
	IssueDependencyIndirect
	IssueDependencyLocalReplace
	IssueDependencyNoChecksum
	IssueDependencyEmptyChecksum
	IssueDependencyVersionConflict

	// Test Coverage issues
	IssueMissingTest
	IssueMissingExample
	IssueMissingBenchmark
	IssueUntestedExport
	IssueUntestedType
	IssueUntestedError
	IssueUntestedConcurrency
	IssueUntestedIOFunction

	// Crypto issues
	IssueWeakCrypto
	IssueInsecureRandom
	IssueWeakHash

	// Serialization issues
	IssueJSONInLoop
	IssueXMLInLoop
	IssueSerializationInLoop

	// IO Buffer issues
	IssueUnbufferedIO
	IssueSmallBuffer
	IssueMissingBuffering

	// Network Pattern issues
	IssueNetworkInLoop
	IssueDNSLookupInLoop
	IssueNoConnectionPool

	// CGO issues
	IssueCGOCall
	IssueCGOInLoop
	IssueCGOMemoryLeak

	// CPU Optimization issues
	IssueCPUIntensive
	IssueUnnecessaryCopy
	IssueBoundsCheckElimination
	IssueInefficientAlgorithm
	IssueCacheUnfriendly
	IssueHighComplexityO2
	IssueHighComplexityO3
	IssuePreventsInlining
	IssueExpensiveOpInHotPath
	IssueModuloPowerOfTwo

	// Misc issues
	IssueMagicNumber
	IssueUselessCondition
	IssueEmptyElse
	IssueSleepInsteadOfSync
	IssueConsoleLogDebugging
	IssueHardcodedConfig
	IssueGlobalVariable
	IssuePointerToSlice

	// Struct Layout issues
	IssueStructLayoutUnoptimized
	IssueStructLargePadding
	IssueStructFieldAlignment

	// CPU Cache optimization issues
	IssueCacheFalseSharing
	IssueCacheLineWaste
	IssueCacheLineAlignment
	IssueOversizedType
	IssueUnspecificIntType
	IssueSoAPattern
	IssueNestedRangeCache
	IssueMapRangeCache

	// Sentinel
	IssueTypeMax
)

// Severity returns the default severity for this issue type
var issueSeverityMap = map[IssueType]SeverityLevel{
	// HIGH severity issues
	IssueMemoryLeak:             SeverityLevelHigh,
	IssueGoroutineLeak:          SeverityLevelHigh,
	IssueRaceCondition:          SeverityLevelHigh,
	IssueSQLNPlusOne:            SeverityLevelHigh,
	IssueHTTPNoTimeout:          SeverityLevelHigh,
	IssuePrivacyHardcodedSecret: SeverityLevelHigh,
	IssueDependencyVulnerable:   SeverityLevelHigh,
	IssueWeakCrypto:             SeverityLevelHigh,
	IssuePanicInLibrary:         SeverityLevelHigh,
	IssueHighComplexityO3:       SeverityLevelHigh,
	IssueCGOMemoryLeak:          SeverityLevelHigh,
	IssueTypeMax:                SeverityLevelHigh,

	// MEDIUM severity issues
	IssueNestedLoop:              SeverityLevelMedium,
	IssueAllocInLoop:             SeverityLevelMedium,
	IssueStringConcatInLoop:      SeverityLevelMedium,
	IssueHighGCPressure:          SeverityLevelMedium,
	IssueSliceAppendInLoop:       SeverityLevelMedium,
	IssueDeferInLoop:             SeverityLevelMedium,
	IssueUncheckedParam:          SeverityLevelMedium,
	IssueAIUnnecessaryComplexity: SeverityLevelMedium,
	IssueStructLayoutUnoptimized: SeverityLevelMedium,
	IssueCacheFalseSharing:       SeverityLevelHigh,
	IssueSoAPattern:              SeverityLevelMedium,
	IssueNestedRangeCache:        SeverityLevelMedium,
}

func (i IssueType) Severity() SeverityLevel {
	if severity, ok := issueSeverityMap[i]; ok {
		return severity
	}
	// LOW severity - all other issues
	return SeverityLevelLow
}

// GetAnalyzer returns the analyzer type that detects this issue
func (i IssueType) GetAnalyzer() AnalyzerType {
	if analyzer, ok := issueAnalyzerMap[i]; ok {
		return analyzer
	}
	return AnalyzerLoop // Default fallback
}
