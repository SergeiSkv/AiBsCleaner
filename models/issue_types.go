package models

import "fmt"

// IssueType represents specific issue types as an enum
//
//go:generate go run github.com/dmarkham/enumer@latest -type=IssueType -trimprefix=Issue
type IssueType uint16

const (
	// Loop issues (0-19)
	IssueNestedLoop IssueType = iota
	IssueAllocInLoop
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
	IssueHighGCPressure:          SeverityLevelMedium,
	IssueSliceAppendInLoop:       SeverityLevelMedium,
	IssueDeferInLoop:             SeverityLevelMedium,
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

// issueAnalyzerMap maps each issue type to its corresponding analyzer
var issueAnalyzerMap = map[IssueType]AnalyzerType{
	// Loop issues
	IssueNestedLoop:       AnalyzerLoop,
	IssueAllocInLoop:      AnalyzerLoop,
	IssueAppendInLoop:     AnalyzerLoop,
	IssueDeferInLoop:      AnalyzerLoop,
	IssueRegexInLoop:      AnalyzerLoop,
	IssueTimeInLoop:       AnalyzerLoop,
	IssueSQLInLoop:        AnalyzerLoop,
	IssueDNSInLoop:        AnalyzerLoop,
	IssueReflectionInLoop: AnalyzerLoop,
	IssueCPUIntensiveLoop: AnalyzerLoop,

	// Memory issues
	IssueMemoryLeak:         AnalyzerMemoryLeak,
	IssueGlobalVar:          AnalyzerMemoryLeak,
	IssueLargeAllocation:    AnalyzerMemoryLeak,
	IssueHighGCPressure:     AnalyzerGCPressure,
	IssueFrequentAllocation: AnalyzerGCPressure,
	IssueLargeHeapAlloc:     AnalyzerGCPressure,
	IssuePointerHeavyStruct: AnalyzerGCPressure,
	IssueMissingDefer:       AnalyzerDeferOptimization,
	IssueMissingClose:       AnalyzerDeferOptimization,

	// Slice issues
	IssueSliceCapacity:     AnalyzerSlice,
	IssueSliceCopy:         AnalyzerSlice,
	IssueSliceAppend:       AnalyzerSlice,
	IssueSliceRangeCopy:    AnalyzerSlice,
	IssueSliceAppendInLoop: AnalyzerSlice,
	IssueSlicePrealloc:     AnalyzerSlice,

	// Map issues
	IssueMapCapacity: AnalyzerMap,
	IssueMapClear:    AnalyzerMap,
	IssueMapPrealloc: AnalyzerMap,

	// String issues
	IssueStringConcat:      AnalyzerString,
	IssueStringBuilder:     AnalyzerString,
	IssueStringInefficient: AnalyzerString,

	// Defer issues
	IssueDeferInShortFunc:      AnalyzerDeferOptimization,
	IssueDeferOverhead:         AnalyzerDeferOptimization,
	IssueUnnecessaryDefer:      AnalyzerDeferOptimization,
	IssueDeferAtEnd:            AnalyzerDeferOptimization,
	IssueMultipleDefers:        AnalyzerDeferOptimization,
	IssueDeferInHotPath:        AnalyzerDeferOptimization,
	IssueDeferLargeCapture:     AnalyzerDeferOptimization,
	IssueUnnecessaryMutexDefer: AnalyzerDeferOptimization,
	IssueMissingDeferUnlock:    AnalyzerDeferOptimization,
	IssueMissingDeferClose:     AnalyzerDeferOptimization,

	// Concurrency issues
	IssueRaceCondition:                AnalyzerRaceCondition,
	IssueRaceConditionGlobal:          AnalyzerRaceCondition,
	IssueUnsyncMapAccess:              AnalyzerRaceCondition,
	IssueRaceClosure:                  AnalyzerRaceCondition,
	IssueGoroutineLeak:                AnalyzerGoroutine,
	IssueUnbufferedChannel:            AnalyzerChannel,
	IssueGoroutineOverhead:            AnalyzerGoroutine,
	IssueSyncMutexValue:               AnalyzerConcurrencyPatterns,
	IssueWaitgroupMisuse:              AnalyzerConcurrencyPatterns,
	IssueRaceInDefer:                  AnalyzerRaceCondition,
	IssueAtomicMisuse:                 AnalyzerConcurrencyPatterns,
	IssueGoroutineNoRecover:           AnalyzerGoroutine,
	IssueGoroutineCapturesLoop:        AnalyzerGoroutine,
	IssueWaitGroupAddInLoop:           AnalyzerConcurrencyPatterns,
	IssueWaitGroupWaitBeforeStart:     AnalyzerConcurrencyPatterns,
	IssueMutexForReadOnly:             AnalyzerConcurrencyPatterns,
	IssueSelectWithSingleCase:         AnalyzerChannel,
	IssueBusyWait:                     AnalyzerConcurrencyPatterns,
	IssueContextBackgroundInGoroutine: AnalyzerContext,
	IssueGoroutinePerRequest:          AnalyzerGoroutine,
	IssueNoWorkerPool:                 AnalyzerGoroutine,
	IssueUnbufferedSignalChan:         AnalyzerChannel,
	IssueSelectDefault:                AnalyzerChannel,
	IssueChannelSize:                  AnalyzerChannel,
	IssueRangeOverChannel:             AnalyzerChannel,
	IssueChannelDeadlock:              AnalyzerChannel,
	IssueChannelMultipleClose:         AnalyzerChannel,
	IssueChannelSendOnClosed:          AnalyzerChannel,

	// Network & HTTP issues
	IssueHTTPNoTimeout:         AnalyzerHTTPClient,
	IssueHTTPNoClose:           AnalyzerHTTPClient,
	IssueHTTPDefaultClient:     AnalyzerHTTPClient,
	IssueHTTPNoContext:         AnalyzerHTTPClient,
	IssueKeepaliveMissing:      AnalyzerHTTPReuse,
	IssueConnectionPool:        AnalyzerHTTPReuse,
	IssueNoReuseConnection:     AnalyzerHTTPReuse,
	IssueHTTPNoConnectionReuse: AnalyzerHTTPReuse,

	// Database issues
	IssueNoPreparedStmt: AnalyzerDatabase,
	IssueMissingDBClose: AnalyzerDatabase,
	IssueSQLNPlusOne:    AnalyzerDatabase,

	// Interface & Reflection issues
	IssueReflection:          AnalyzerReflection,
	IssueInterfaceAllocation: AnalyzerInterface,
	IssueEmptyInterface:      AnalyzerInterface,
	IssueInterfacePollution:  AnalyzerInterface,

	// Time & Regex issues
	IssueTimeAfterLeak:      AnalyzerTime,
	IssueTimeFormat:         AnalyzerTime,
	IssueTimeNowInLoop:      AnalyzerTime,
	IssueRegexCompile:       AnalyzerRegex,
	IssueRegexCompileInLoop: AnalyzerRegex,

	// Context issues
	IssueContextBackground:    AnalyzerContext,
	IssueContextValue:         AnalyzerContext,
	IssueMissingContextCancel: AnalyzerContext,
	IssueContextLeak:          AnalyzerContext,
	IssueContextInStruct:      AnalyzerContext,
	IssueContextNotFirst:      AnalyzerContext,
	IssueContextMisuse:        AnalyzerContext,

	// Error handling issues
	IssueErrorIgnored:      AnalyzerErrorHandling,
	IssueErrorCheckMissing: AnalyzerErrorHandling,
	IssuePanicRecover:      AnalyzerErrorHandling,
	IssueErrorStringFormat: AnalyzerErrorHandling,
	IssuePanicRisk:         AnalyzerErrorHandling,
	IssuePanicInLibrary:    AnalyzerErrorHandling,

	// AI Bullshit issues
	IssueAIBullshitConcurrency:   AnalyzerAIBullshit,
	IssueAIReflectionOverkill:    AnalyzerAIBullshit,
	IssueAIPatternAbuse:          AnalyzerAIBullshit,
	IssueAIEnterpriseHelloWorld:  AnalyzerAIBullshit,
	IssueAICaptainObvious:        AnalyzerAIBullshit,
	IssueAIOverengineeredSimple:  AnalyzerAIBullshit,
	IssueAIGeneratedComment:      AnalyzerAIBullshit,
	IssueAIUnnecessaryComplexity: AnalyzerAIBullshit,
	IssueAIOverAbstraction:       AnalyzerAIBullshit,
	IssueAIVariable:              AnalyzerAIBullshit,
	IssueAIErrorHandling:         AnalyzerAIBullshit,
	IssueAIStructure:             AnalyzerAIBullshit,
	IssueAIRepetition:            AnalyzerAIBullshit,
	IssueAIFactorySimple:         AnalyzerAIBullshit,
	IssueAIRedundantElse:         AnalyzerAIBullshit,
	IssueAIGoroutineOverkill:     AnalyzerAIBullshit,
	IssueAIUnnecessaryReflection: AnalyzerAIBullshit,
	IssueAIUnnecessaryInterface:  AnalyzerAIBullshit,

	// GC Pressure issues
	IssueHighGCPressureDetected:     AnalyzerGCPressure,
	IssueFrequentAllocationDetected: AnalyzerGCPressure,
	IssueLargeHeapAllocDetected:     AnalyzerGCPressure,
	IssuePointerHeavyStructDetected: AnalyzerGCPressure,

	// Sync Pool issues
	IssueSyncPoolOpportunity: AnalyzerSyncPool,
	IssueSyncPoolPutMissing:  AnalyzerSyncPool,
	IssueSyncPoolTypeAssert:  AnalyzerSyncPool,
	IssueSyncPoolMisuse:      AnalyzerSyncPool,

	// API Misuse issues
	IssueAPIMisuse:               AnalyzerAPIMisuse,
	IssueWGMisuse:                AnalyzerAPIMisuse,
	IssuePprofInProd:             AnalyzerAPIMisuse,
	IssuePprofNilWriter:          AnalyzerAPIMisuse,
	IssueDebugInProd:             AnalyzerAPIMisuse,
	IssueWaitgroupAddInGoroutine: AnalyzerAPIMisuse,
	IssueContextBackgroundMisuse: AnalyzerAPIMisuse,
	IssueSleepInLoop:             AnalyzerAPIMisuse,
	IssueSprintfConcatenation:    AnalyzerAPIMisuse,
	IssueLogInHotPath:            AnalyzerAPIMisuse,
	IssueRecoverWithoutDefer:     AnalyzerAPIMisuse,
	IssueJSONMarshalInLoop:       AnalyzerAPIMisuse,
	IssueRegexCompileInFunc:      AnalyzerAPIMisuse,
	IssueMutexByValue:            AnalyzerAPIMisuse,

	// Privacy issues
	IssuePrivacyHardcodedSecret:    AnalyzerPrivacy,
	IssuePrivacyAWSKey:             AnalyzerPrivacy,
	IssuePrivacyJWTToken:           AnalyzerPrivacy,
	IssuePrivacyEmailPII:           AnalyzerPrivacy,
	IssuePrivacySSNPII:             AnalyzerPrivacy,
	IssuePrivacyCreditCardPII:      AnalyzerPrivacy,
	IssuePrivacyLoggingSensitive:   AnalyzerPrivacy,
	IssuePrivacyPrintingSensitive:  AnalyzerPrivacy,
	IssuePrivacyExposedField:       AnalyzerPrivacy,
	IssuePrivacyUnencryptedDBWrite: AnalyzerPrivacy,
	IssuePrivacyDirectInputToDB:    AnalyzerPrivacy,

	// Dependency issues
	IssueDependencyDeprecated:      AnalyzerDependency,
	IssueDependencyVulnerable:      AnalyzerDependency,
	IssueDependencyOutdated:        AnalyzerDependency,
	IssueDependencyCGO:             AnalyzerDependency,
	IssueDependencyUnsafe:          AnalyzerDependency,
	IssueDependencyInternal:        AnalyzerDependency,
	IssueDependencyIndirect:        AnalyzerDependency,
	IssueDependencyLocalReplace:    AnalyzerDependency,
	IssueDependencyNoChecksum:      AnalyzerDependency,
	IssueDependencyEmptyChecksum:   AnalyzerDependency,
	IssueDependencyVersionConflict: AnalyzerDependency,

	// Test Coverage issues
	IssueMissingTest:         AnalyzerTestCoverage,
	IssueMissingExample:      AnalyzerTestCoverage,
	IssueMissingBenchmark:    AnalyzerTestCoverage,
	IssueUntestedExport:      AnalyzerTestCoverage,
	IssueUntestedType:        AnalyzerTestCoverage,
	IssueUntestedError:       AnalyzerTestCoverage,
	IssueUntestedConcurrency: AnalyzerTestCoverage,
	IssueUntestedIOFunction:  AnalyzerTestCoverage,

	// Crypto issues
	IssueWeakCrypto:     AnalyzerCrypto,
	IssueInsecureRandom: AnalyzerCrypto,
	IssueWeakHash:       AnalyzerCrypto,

	// Serialization issues
	IssueJSONInLoop:          AnalyzerSerialization,
	IssueXMLInLoop:           AnalyzerSerialization,
	IssueSerializationInLoop: AnalyzerSerialization,

	// IO Buffer issues
	IssueUnbufferedIO:     AnalyzerIOBuffer,
	IssueSmallBuffer:      AnalyzerIOBuffer,
	IssueMissingBuffering: AnalyzerIOBuffer,

	// Network Pattern issues
	IssueNetworkInLoop:    AnalyzerNetworkPatterns,
	IssueDNSLookupInLoop:  AnalyzerNetworkPatterns,
	IssueNoConnectionPool: AnalyzerNetworkPatterns,

	// CGO issues
	IssueCGOCall:       AnalyzerCGO,
	IssueCGOInLoop:     AnalyzerCGO,
	IssueCGOMemoryLeak: AnalyzerCGO,

	// CPU Optimization issues
	IssueCPUIntensive:           AnalyzerCPUOptimization,
	IssueUnnecessaryCopy:        AnalyzerCPUOptimization,
	IssueBoundsCheckElimination: AnalyzerCPUOptimization,
	IssueInefficientAlgorithm:   AnalyzerCPUOptimization,
	IssueCacheUnfriendly:        AnalyzerCPUOptimization,
	IssueHighComplexityO2:       AnalyzerCPUOptimization,
	IssueHighComplexityO3:       AnalyzerCPUOptimization,
	IssuePreventsInlining:       AnalyzerCPUOptimization,
	IssueExpensiveOpInHotPath:   AnalyzerCPUOptimization,
	IssueModuloPowerOfTwo:       AnalyzerCPUOptimization,

	// Misc issues
	IssueMagicNumber:         AnalyzerAPIMisuse,
	IssueUselessCondition:    AnalyzerAPIMisuse,
	IssueEmptyElse:           AnalyzerAPIMisuse,
	IssueSleepInsteadOfSync:  AnalyzerAPIMisuse,
	IssueConsoleLogDebugging: AnalyzerAPIMisuse,
	IssueHardcodedConfig:     AnalyzerAPIMisuse,
	IssueGlobalVariable:      AnalyzerAPIMisuse,
	IssuePointerToSlice:      AnalyzerAPIMisuse,

	// Struct Layout issues
	IssueStructLayoutUnoptimized: AnalyzerCPUOptimization,
	IssueStructLargePadding:      AnalyzerCPUOptimization,
	IssueStructFieldAlignment:    AnalyzerCPUOptimization,

	// CPU Cache optimization issues
	IssueCacheFalseSharing:  AnalyzerCPUOptimization,
	IssueCacheLineWaste:     AnalyzerCPUOptimization,
	IssueCacheLineAlignment: AnalyzerCPUOptimization,
	IssueOversizedType:      AnalyzerCPUOptimization,
	IssueUnspecificIntType:  AnalyzerCPUOptimization,
	IssueSoAPattern:         AnalyzerCPUOptimization,
	IssueNestedRangeCache:   AnalyzerCPUOptimization,
	IssueMapRangeCache:      AnalyzerCPUOptimization,
}

// GetAnalyzer returns the analyzer type that detects this issue
func (i IssueType) GetAnalyzer() AnalyzerType {
	if analyzer, ok := issueAnalyzerMap[i]; ok {
		return analyzer
	}
	return AnalyzerLoop // Default fallback
}

// GetPVEID returns the PVE-ID for this issue type (e.g., PVE-001)
func (i IssueType) GetPVEID() string {
	return fmt.Sprintf("PVE-%03d", int(i))
}
