package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestLoopAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "string range loop",
			code: `package main
func test() {
	for _, char := range "hello world" {
		_ = char
	}
}`,
			expected: []string{"INEFFICIENT_RANGE"},
		},
		{
			name: "nested loop with allocation",
			code: `package main
func test() {
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			s := make([]int, 100)
			_ = s
		}
	}
}`,
			expected: []string{"NESTED_LOOP_ALLOCATION"},
		},
		{
			name: "nested loop with append",
			code: `package main
func test() {
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			var slice []int
			slice = append(slice, j)
			_ = slice
		}
	}
}`,
			expected: []string{"NESTED_LOOP_ALLOCATION"},
		},
		{
			name: "simple nested loop without allocation",
			code: `package main
func test() {
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			_ = i + j
		}
	}
}`,
			expected: []string{},
		},
		{
			name: "range over variable (not literal string)",
			code: `package main
func test(s string) {
	for _, char := range s {
		_ = char
	}
}`,
			expected: []string{},
		},
		{
			name: "simple loop with allocation (not nested)",
			code: `package main
func test() {
	for i := 0; i < 100; i++ {
		s := make([]int, 1000)
		_ = s
	}
}`,
			expected: []string{},
		},
		{
			name: "no issues - efficient loop",
			code: `package main
func test(items []int) int {
	sum := 0
	for _, item := range items {
		sum += item
	}
	return sum
}`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				fset := token.NewFileSet()
				node, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
				if err != nil {
					t.Fatalf("Failed to parse code: %v", err)
				}

				analyzer := NewLoopAnalyzer()
				issues := analyzer.Analyze(node, fset)

				issueTypes := make(map[string]bool)
				for _, issue := range issues {
					issueTypes[issue.Type.String()] = true
				}

				for _, expected := range tt.expected {
					normalized := normalizeIssueName(expected)
					if !issueTypes[normalized] {
						t.Logf("Expected issue %s not found", normalized)
					}
				}

				if len(tt.expected) == 0 && len(issues) > 0 {
					t.Logf("Expected no issues, but found %d", len(issues))
				}
			},
		)
	}
}
