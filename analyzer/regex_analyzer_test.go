package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegexAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "regexp.Compile in loop",
			code: `package main
import "regexp"
func test(patterns []string) {
	for _, pattern := range patterns {
		re := regexp.Compile(pattern)
		_ = re
	}
}`,
			expected: []string{"REGEX_COMPILE_IN_LOOP"},
		},
		{
			name: "regexp.MustCompile in loop",
			code: `package main
import "regexp"
func test(patterns []string) {
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		_ = re
	}
}`,
			expected: []string{"REGEX_COMPILE_IN_LOOP"},
		},
		{
			name: "regexp.Compile with static pattern",
			code: `package main
import "regexp"
func test() {
	re := regexp.Compile("[a-z]+")
	_ = re
}`,
			expected: []string{"REGEX_WITHOUT_MUST"},
		},
		{
			name: "regexp.MustCompile with static pattern - no issue",
			code: `package main
import "regexp"
func test() {
	re := regexp.MustCompile("[a-z]+")
	_ = re
}`,
			expected: []string{},
		},
		{
			name: "regexp.Compile outside loop - no loop issue",
			code: `package main
import "regexp"
func test(pattern string) {
	re := regexp.Compile(pattern)
	_ = re
}`,
			expected: []string{},
		},
		{
			name: "compiled regex reused in loop - no issue",
			code: `package main
import "regexp"
func test(texts []string) {
	re := regexp.MustCompile("[a-z]+")
	for _, text := range texts {
		matches := re.FindAllString(text, -1)
		_ = matches
	}
}`,
			expected: []string{},
		},
		{
			name: "nested loop with regex compilation",
			code: `package main
import "regexp"
func test() {
	patterns := []string{"[a-z]+", "[0-9]+"}
	texts := []string{"hello", "123"}
	for _, pattern := range patterns {
		for _, text := range texts {
			re := regexp.Compile(pattern)
			matches := re.FindAllString(text, -1)
			_ = matches
		}
	}
}`,
			expected: []string{"REGEX_COMPILE_IN_LOOP"},
		},
		{
			name: "multiple regex issues",
			code: `package main
import "regexp"
func test(patterns []string) {
	// Static pattern should use MustCompile
	re1 := regexp.Compile("[a-z]+")
	_ = re1
	
	// Compilation in loop
	for _, pattern := range patterns {
		re2 := regexp.Compile(pattern)
		_ = re2
	}
}`,
			expected: []string{"REGEX_WITHOUT_MUST", "REGEX_COMPILE_IN_LOOP"},
		},
		{
			name: "no regex usage",
			code: `package main
func test(texts []string) {
	for _, text := range texts {
		result := text + "_processed"
		_ = result
	}
}`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				fset := token.NewFileSet()
				node, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
				require.NoError(t, err, "Failed to parse code")

				analyzer := NewRegexAnalyzer()
				issues := analyzer.Analyze(node, fset)

				issueTypes := make(map[string]bool)
				for _, issue := range issues {
					issueTypes[issue.Type.String()] = true
				}

				for _, expected := range tt.expected {
					assert.True(t, issueTypes[expected], "Expected issue %s not found", expected)
				}

				if len(tt.expected) == 0 {
					assert.Empty(t, issues, "Expected no issues, but found %d", len(issues))
				}
			},
		)
	}
}
