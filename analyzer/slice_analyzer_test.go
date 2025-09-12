package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSliceAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "slice without capacity hint",
			code: `package main
func test() {
	s := make([]int, 0)
	_ = s
}`,
			expected: []string{"SLICE_WITHOUT_CAPACITY"},
		},
		{
			name: "append in loop",
			code: `package main
func test() []int {
	var result []int
	for i := 0; i < 100; i++ {
		result = append(result, i*2)
	}
	return result
}`,
			expected: []string{"SLICE_APPEND_IN_LOOP"},
		},
		{
			name: "slice with proper capacity - still flagged by simple analyzer",
			code: `package main
func test() {
	s := make([]int, 0, 1000)
	for i := 0; i < 1000; i++ {
		s = append(s, i)
	}
	_ = s
}`,
			expected: []string{"SLICE_APPEND_IN_LOOP"}, // Simple analyzer flags all append-in-loop cases
		},
		{
			name: "append outside loop",
			code: `package main
func test() []int {
	var result []int
	result = append(result, 1, 2, 3)
	return result
}`,
			expected: []string{},
		},
		{
			name: "make with capacity",
			code: `package main
func test() {
	s := make([]int, 10, 20)
	_ = s
}`,
			expected: []string{},
		},
		{
			name: "regular array not slice",
			code: `package main
func test() {
	var arr [10]int
	_ = arr
}`,
			expected: []string{},
		},
		{
			name: "pre-allocated slice - simple analyzer still flags",
			code: `package main
func test() []int {
	s := make([]int, 0, 100)
	for i := 0; i < 100; i++ {
		s = append(s, i)
	}
	return s
}`,
			expected: []string{"SLICE_APPEND_IN_LOOP"}, // Simple analyzer flags all append-in-loop cases
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				fset := token.NewFileSet()
				node, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
				require.NoError(t, err, "Failed to parse code")

				analyzer := NewSliceAnalyzer()
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
