package analyzer

import "go/token"

// SeverityLevel represents issue severity level as uint8 for efficiency
type SeverityLevel uint8

const (
	SeverityLevelLow SeverityLevel = iota
	SeverityLevelMedium
	SeverityLevelHigh
	SeverityLevelCritical
)

// Analysis thresholds and limits
const (
	MaxQueriesPerFunction     = 10
	MaxNestedLoops            = 3
	MaxFunctionParams         = 5
	MaxSearchDepth            = 100
	MediumComplexityThreshold = 10
	MaxFunctionStatements     = 50
)

type Issue struct {
	File       string
	Line       int
	Column     int
	Position   token.Position
	Type       IssueType
	Severity   SeverityLevel
	Message    string
	Suggestion string
	Code       string
	CanBeFixed bool
	WhyBad     string // Detailed explanation why this is problematic
}

type Analyzer interface {
	Name() string
	Analyze(node interface{}, fset *token.FileSet) []*Issue
}
