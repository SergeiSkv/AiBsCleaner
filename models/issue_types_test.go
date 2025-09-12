package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIssueSeverity(t *testing.T) {
	tests := []struct {
		name     string
		issue    IssueType
		expected SeverityLevel
	}{
		{"Memory leak is high", IssueMemoryLeak, SeverityLevelHigh},
		{"Goroutine leak is high", IssueGoroutineLeak, SeverityLevelHigh},
		{"Race condition is high", IssueRaceCondition, SeverityLevelHigh},
		{"Nested loop is medium", IssueNestedLoop, SeverityLevelMedium},
		{"Alloc in loop is medium", IssueAllocInLoop, SeverityLevelMedium},
		{"Unknown defaults to low", IssueType(9999), SeverityLevelLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			severity := tt.issue.Severity()
			require.Equal(t, tt.expected, severity)
		})
	}
}

func TestIssueGetAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		issue    IssueType
		expected AnalyzerType
	}{
		{"Nested loop", IssueNestedLoop, AnalyzerLoop},
		{"Alloc in loop", IssueAllocInLoop, AnalyzerLoop},
		{"Defer in loop", IssueDeferInLoop, AnalyzerLoop},
		{"Slice capacity", IssueSliceCapacity, AnalyzerSlice},
		{"Map capacity", IssueMapCapacity, AnalyzerMap},
		{"Defer overhead", IssueDeferOverhead, AnalyzerDeferOptimization},
		{"Race condition", IssueRaceCondition, AnalyzerRaceCondition},
		{"HTTP no timeout", IssueHTTPNoTimeout, AnalyzerHTTPClient},
		{"Memory leak", IssueMemoryLeak, AnalyzerMemoryLeak},
		{"Context background", IssueContextBackground, AnalyzerContext},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := tt.issue.GetAnalyzer()
			require.Equal(t, tt.expected, analyzer)
		})
	}
}

func TestIssueGetPVEID(t *testing.T) {
	// Test that PVE IDs are properly formatted
	pveID := IssueNestedLoop.GetPVEID()
	require.Regexp(t, `^PVE-\d{3}$`, pveID)

	pveID2 := IssueMemoryLeak.GetPVEID()
	require.Regexp(t, `^PVE-\d{3}$`, pveID2)

	// Different issues should have different IDs
	require.NotEqual(t, pveID, pveID2)
}

func TestIssueTypeString(t *testing.T) {
	// Test String() method for enum (enumer removes "Issue" prefix)
	require.Equal(t, "NestedLoop", IssueNestedLoop.String())
	require.Equal(t, "AllocInLoop", IssueAllocInLoop.String())
	require.Equal(t, "MemoryLeak", IssueMemoryLeak.String())
}

func TestIssueTypeValues(t *testing.T) {
	// Test that IssueTypeValues() returns all enum values
	values := IssueTypeValues()
	require.NotEmpty(t, values)
	require.Contains(t, values, IssueNestedLoop)
	require.Contains(t, values, IssueMemoryLeak)
	require.Contains(t, values, IssueRaceCondition)
}

func TestIssueTypeIsValid(t *testing.T) {
	// Test IsAIssueType() validation
	require.True(t, IssueNestedLoop.IsAIssueType())
	require.True(t, IssueMemoryLeak.IsAIssueType())
	require.True(t, IssueRaceCondition.IsAIssueType())
}

func TestSeverityLevelString(t *testing.T) {
	// Enumer removes "SeverityLevel" prefix
	require.Equal(t, "Low", SeverityLevelLow.String())
	require.Equal(t, "Medium", SeverityLevelMedium.String())
	require.Equal(t, "High", SeverityLevelHigh.String())
}

func TestSeverityLevelValues(t *testing.T) {
	values := SeverityLevelValues()
	require.Len(t, values, 3)
	require.Contains(t, values, SeverityLevelLow)
	require.Contains(t, values, SeverityLevelMedium)
	require.Contains(t, values, SeverityLevelHigh)
}

func TestSeverityLevelIsValid(t *testing.T) {
	require.True(t, SeverityLevelLow.IsASeverityLevel())
	require.True(t, SeverityLevelMedium.IsASeverityLevel())
	require.True(t, SeverityLevelHigh.IsASeverityLevel())
}

func TestAnalyzerTypeString(t *testing.T) {
	require.Equal(t, "Loop", AnalyzerLoop.String())
	require.Equal(t, "Slice", AnalyzerSlice.String())
	require.Equal(t, "Map", AnalyzerMap.String())
}

func TestAnalyzerTypeValues(t *testing.T) {
	values := AnalyzerTypeValues()
	require.NotEmpty(t, values)
	require.Contains(t, values, AnalyzerLoop)
	require.Contains(t, values, AnalyzerSlice)
	require.Contains(t, values, AnalyzerMemoryLeak)
}

func TestAnalyzerTypeIsValid(t *testing.T) {
	require.True(t, AnalyzerLoop.IsAAnalyzerType())
	require.True(t, AnalyzerMemoryLeak.IsAAnalyzerType())
	require.True(t, AnalyzerSlice.IsAAnalyzerType())
}

func TestIssueTypeCoverage(t *testing.T) {
	// Test that all issue types have proper severity and analyzer mappings
	allIssues := []IssueType{
		IssueNestedLoop,
		IssueAllocInLoop,
		IssueAppendInLoop,
		IssueDeferInLoop,
		IssueMemoryLeak,
		IssueGlobalVar,
		IssueSliceCapacity,
		IssueMapCapacity,
		IssueStringConcat,
		IssueDeferInShortFunc,
		IssueRaceCondition,
		IssueGoroutineLeak,
		IssueHTTPNoTimeout,
		IssueContextBackground,
	}

	for _, issue := range allIssues {
		t.Run(issue.String(), func(t *testing.T) {
			// Ensure severity is valid
			severity := issue.Severity()
			require.Contains(t, []SeverityLevel{SeverityLevelLow, SeverityLevelMedium, SeverityLevelHigh}, severity)

			// Ensure analyzer is valid
			analyzer := issue.GetAnalyzer()
			require.NotEmpty(t, analyzer.String())

			// Ensure PVE ID is formatted correctly
			pveID := issue.GetPVEID()
			require.Regexp(t, `^PVE-\d{3}$`, pveID)
		})
	}
}
