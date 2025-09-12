package analyzer

var issueAnalyzerMap = initIssueAnalyzerMap()

func getAnalyzerMappings() []struct {
	analyzer AnalyzerType
	issues   []IssueType
} {
	return []struct {
		analyzer AnalyzerType
		issues   []IssueType
	}{
		{AnalyzerLoop, []IssueType{
			IssueNestedLoop, IssueAllocInLoop, IssueStringConcatInLoop,
			IssueAppendInLoop, IssueDeferInLoop, IssueRegexInLoop,
			IssueTimeInLoop, IssueSQLInLoop, IssueDNSInLoop,
			IssueReflectionInLoop, IssueCPUIntensiveLoop,
		}},
		{AnalyzerMemoryLeak, []IssueType{
			IssueMemoryLeak, IssueGlobalVar, IssueLargeAllocation,
		}},
		{AnalyzerGCPressure, []IssueType{
			IssueHighGCPressure, IssueFrequentAllocation,
			IssueLargeHeapAlloc, IssuePointerHeavyStruct,
			IssueHighGCPressureDetected, IssueFrequentAllocationDetected,
			IssueLargeHeapAllocDetected, IssuePointerHeavyStructDetected,
		}},
		{AnalyzerSlice, []IssueType{
			IssueSliceCapacity, IssueSliceCopy, IssueSliceAppend,
			IssueSliceRangeCopy, IssueSliceAppendInLoop, IssueSlicePrealloc,
		}},
		{AnalyzerMap, []IssueType{
			IssueMapCapacity, IssueMapClear, IssueMapPrealloc,
		}},
		{AnalyzerString, []IssueType{
			IssueStringConcat, IssueStringBuilder, IssueStringInefficient,
		}},
		{AnalyzerDeferOptimization, []IssueType{
			IssueDeferInShortFunc, IssueDeferOverhead, IssueUnnecessaryDefer,
			IssueDeferAtEnd, IssueMultipleDefers, IssueDeferInHotPath,
			IssueDeferLargeCapture, IssueUnnecessaryMutexDefer,
			IssueMissingDefer, IssueMissingClose, IssueMissingDeferUnlock,
			IssueMissingDeferClose,
		}},
		{AnalyzerRaceCondition, []IssueType{
			IssueRaceCondition, IssueRaceConditionGlobal,
			IssueUnsyncMapAccess, IssueRaceClosure,
		}},
		{AnalyzerGoroutine, []IssueType{
			IssueGoroutineLeak, IssueGoroutineOverhead,
			IssueGoroutinePerRequest, IssueNoWorkerPool,
		}},
		{AnalyzerChannel, []IssueType{
			IssueUnbufferedChannel, IssueUnbufferedSignalChan,
			IssueSelectDefault, IssueChannelSize, IssueRangeOverChannel,
		}},
		{AnalyzerHTTPClient, []IssueType{
			IssueHTTPNoTimeout, IssueHTTPNoClose,
			IssueHTTPDefaultClient, IssueHTTPNoContext,
		}},
		{AnalyzerDatabase, []IssueType{
			IssueNoPreparedStmt, IssueMissingDBClose, IssueSQLNPlusOne,
		}},
		{AnalyzerAIBullshit, []IssueType{
			IssueAIBullshitConcurrency, IssueAIReflectionOverkill,
			IssueAIPatternAbuse, IssueAIEnterpriseHelloWorld,
			IssueAICaptainObvious, IssueAIOverengineeredSimple,
			IssueAIGeneratedComment, IssueAIUnnecessaryComplexity,
			IssueAIOverAbstraction, IssueAIVariable, IssueAIErrorHandling,
			IssueAIStructure, IssueAIRepetition, IssueAIFactorySimple,
			IssueAIRedundantElse, IssueAIGoroutineOverkill,
			IssueAIUnnecessaryReflection, IssueAIUnnecessaryInterface,
		}},
		{AnalyzerNilPtr, []IssueType{
			IssueNilCheck, IssueNilReturn, IssuePotentialNilDeref,
			IssuePotentialNilIndex, IssueRangeOverNil,
			IssueNilMethodCall, IssueUncheckedParam,
		}},
		{AnalyzerPrivacy, []IssueType{
			IssuePrivacyHardcodedSecret, IssuePrivacyAWSKey,
			IssuePrivacyJWTToken, IssuePrivacyEmailPII,
			IssuePrivacySSNPII, IssuePrivacyCreditCardPII,
			IssuePrivacyLoggingSensitive, IssuePrivacyPrintingSensitive,
			IssuePrivacyExposedField, IssuePrivacyUnencryptedDBWrite,
			IssuePrivacyDirectInputToDB,
		}},
		{AnalyzerDependency, []IssueType{
			IssueDependencyDeprecated, IssueDependencyVulnerable,
			IssueDependencyOutdated, IssueDependencyCGO, IssueDependencyUnsafe,
			IssueDependencyInternal, IssueDependencyIndirect,
			IssueDependencyLocalReplace, IssueDependencyNoChecksum,
			IssueDependencyEmptyChecksum, IssueDependencyVersionConflict,
		}},
		{AnalyzerTestCoverage, []IssueType{
			IssueMissingTest, IssueMissingExample, IssueMissingBenchmark,
			IssueUntestedExport, IssueUntestedType, IssueUntestedError,
			IssueUntestedConcurrency, IssueUntestedIOFunction,
		}},
		{AnalyzerCrypto, []IssueType{
			IssueWeakCrypto, IssueInsecureRandom, IssueWeakHash,
		}},
		{AnalyzerSerialization, []IssueType{
			IssueJSONInLoop, IssueXMLInLoop, IssueSerializationInLoop,
		}},
		{AnalyzerIOBuffer, []IssueType{
			IssueUnbufferedIO, IssueSmallBuffer, IssueMissingBuffering,
		}},
		{AnalyzerHTTPReuse, []IssueType{
			IssueKeepaliveMissing, IssueConnectionPool, IssueNoReuseConnection,
			IssueHTTPNoConnectionReuse,
		}},
		{AnalyzerCGO, []IssueType{
			IssueCGOCall, IssueCGOInLoop, IssueCGOMemoryLeak,
		}},
		{AnalyzerNetworkPatterns, []IssueType{
			IssueNetworkInLoop, IssueDNSLookupInLoop, IssueNoConnectionPool,
		}},
		{AnalyzerCPUOptimization, []IssueType{
			IssueCPUIntensive, IssueUnnecessaryCopy, IssueBoundsCheckElimination,
			IssueInefficientAlgorithm, IssueCacheUnfriendly, IssueHighComplexityO2,
			IssueHighComplexityO3, IssuePreventsInlining, IssueExpensiveOpInHotPath,
			IssueModuloPowerOfTwo,
		}},
		{AnalyzerSyncPool, []IssueType{
			IssueSyncPoolOpportunity, IssueSyncPoolPutMissing, IssueSyncPoolTypeAssert,
			IssueSyncPoolMisuse,
		}},
		{AnalyzerContext, []IssueType{
			IssueContextBackground, IssueContextValue, IssueMissingContextCancel,
			IssueContextLeak, IssueContextInStruct, IssueContextNotFirst,
			IssueContextMisuse,
		}},
		{AnalyzerErrorHandling, []IssueType{
			IssueErrorIgnored, IssueErrorCheckMissing, IssuePanicRecover,
			IssueErrorStringFormat, IssuePanicRisk, IssuePanicInLibrary,
		}},
		{AnalyzerAPIMisuse, []IssueType{
			IssueAPIMisuse, IssueWGMisuse, IssuePprofInProd, IssuePprofNilWriter,
			IssueDebugInProd, IssueWaitgroupAddInGoroutine, IssueContextBackgroundMisuse,
			IssueSleepInLoop, IssueSprintfConcatenation, IssueLogInHotPath,
			IssueRecoverWithoutDefer, IssueJSONMarshalInLoop, IssueRegexCompileInFunc,
			IssueMutexByValue,
		}},
		{AnalyzerConcurrencyPatterns, []IssueType{
			IssueSyncMutexValue, IssueWaitgroupMisuse, IssueRaceInDefer,
			IssueAtomicMisuse, IssueGoroutineNoRecover, IssueGoroutineCapturesLoop,
			IssueWaitGroupAddInLoop, IssueWaitGroupWaitBeforeStart, IssueMutexForReadOnly,
			IssueSelectWithSingleCase, IssueBusyWait, IssueContextBackgroundInGoroutine,
			IssueChannelDeadlock, IssueChannelMultipleClose, IssueChannelSendOnClosed,
		}},
		{AnalyzerInterface, []IssueType{
			IssueReflection, IssueInterfaceAllocation, IssueEmptyInterface,
			IssueInterfacePollution,
		}},
		{AnalyzerTime, []IssueType{
			IssueTimeAfterLeak, IssueTimeFormat, IssueTimeNowInLoop,
		}},
		{AnalyzerRegex, []IssueType{
			IssueRegexCompile, IssueRegexCompileInLoop,
		}},
		// Misc issues default to Loop analyzer
		{AnalyzerLoop, []IssueType{
			IssueMagicNumber, IssueUselessCondition, IssueEmptyElse,
			IssueSleepInsteadOfSync, IssueConsoleLogDebugging,
			IssueHardcodedConfig, IssueGlobalVariable, IssuePointerToSlice,
			IssueTypeMax,
		}},
	}
}

func initIssueAnalyzerMap() map[IssueType]AnalyzerType {
	m := make(map[IssueType]AnalyzerType, 200)

	// Get mappings from helper function
	mappings := getAnalyzerMappings()

	// Populate the map
	for _, mapping := range mappings {
		for _, issue := range mapping.issues {
			m[issue] = mapping.analyzer
		}
	}

	return m
}
