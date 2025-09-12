package analyzer

import "go/token"

type Severity string

const (
	SeverityHigh   Severity = "HIGH"
	SeverityMedium Severity = "MEDIUM"
	SeverityLow    Severity = "LOW"
)

type Issue struct {
	File        string
	Line        int
	Column      int
	Position    token.Position
	Type        string
	Severity    Severity
	Message     string
	Suggestion  string
	Code        string
}

type Analyzer interface {
	Name() string
	Analyze(filename string, node interface{}, fset *token.FileSet) []Issue
}