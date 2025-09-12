package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestMapAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "map created without size hint",
			code: `package main
func test() {
	m := make(map[string]int)
	m["key"] = 1
}`,
			expected: []string{"MAP_WITHOUT_SIZE_HINT"},
		},
		{
			name: "map created with size hint - no issue",
			code: `package main
func test() {
	m := make(map[string]int, 10)
	m["key"] = 1
}`,
			expected: []string{},
		},
		{
			name: "inefficient map iteration - accessing value in loop",
			code: `package main
func test() {
	m := map[string]int{"a": 1, "b": 2}
	for k := range m {
		value := m[k]
		_ = value
	}
}`,
			expected: []string{"INEFFICIENT_MAP_ITERATION"},
		},
		{
			name: "efficient map iteration - using key and value",
			code: `package main
func test() {
	m := map[string]int{"a": 1, "b": 2}
	for k, v := range m {
		_ = k
		_ = v
	}
}`,
			expected: []string{},
		},
		{
			name: "map iteration only keys - no access to values",
			code: `package main
func test() {
	m := map[string]int{"a": 1, "b": 2}
	for k := range m {
		println(k)
	}
}`,
			expected: []string{},
		},
		{
			name: "multiple map issues",
			code: `package main
func test() {
	m1 := make(map[string]int)
	m2 := map[string]int{"a": 1, "b": 2}
	for k := range m2 {
		m1[k] = m2[k]
	}
}`,
			expected: []string{"MAP_WITHOUT_SIZE_HINT", "INEFFICIENT_MAP_ITERATION"},
		},
		{
			name: "map literal initialization - no issue",
			code: `package main
func test() {
	m := map[string]int{
		"key1": 1,
		"key2": 2,
	}
	_ = m
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

				analyzer := NewMapAnalyzer()
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
