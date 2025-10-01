package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestInterfaceAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "type assertion in loop - not detected by current analyzer",
			code: `package main
func test(items []interface{}) {
	for _, item := range items {
		str := item.(string)
		_ = str
	}
}`,
			expected: []string{},
		},
		{
			name: "type assertion with ok check in loop - not detected",
			code: `package main
func test(items []interface{}) {
	for _, item := range items {
		if str, ok := item.(string); ok {
			_ = str
		}
	}
}`,
			expected: []string{},
		},
		{
			name: "type assertion outside loop - no issue",
			code: `package main
func test(item interface{}) {
		str := item.(string)
		_ = str
}`,
			expected: []string{},
		},
		{
			name: "empty interface parameter - analyzer doesn't detect this pattern",
			code: `package main
func process(data interface{}) {}
func test() {
	var empty interface{}
	process(empty)
}`,
			expected: []string{},
		},
		{
			name: "multiple interface issues - none detected",
			code: `package main
func process(data interface{}) {}
func test(items []interface{}) {
	for _, item := range items {
		str := item.(string)
		var empty interface{}
		process(empty)
		_ = str
	}
}`,
			expected: []string{},
		},
		{
			name: "type switch instead of assertion - no issue",
			code: `package main
func test(items []interface{}) {
	for _, item := range items {
		switch v := item.(type) {
		case string:
			process(v)
		case int:
			process(v)
		}
	}
}`,
			expected: []string{},
		},
		{
			name: "concrete type parameters - no issue",
			code: `package main
func test() {
	process("hello")
	process(42)
}

func process(data string) {
	// process data
}`,
			expected: []string{},
		},
		{
			name: "nested type assertion in loop - not detected",
			code: `package main
func test() {
	data := [][]interface{}{{"a", "b"}, {"c", "d"}}
	for _, row := range data {
		for _, item := range row {
			str := item.(string)
			_ = str
		}
	}
}`,
			expected: []string{},
		},
		{
			name: "type assertion in for-range with index - not detected",
			code: `package main
func test(items []interface{}) {
	for i := range items {
		str := items[i].(string)
		_ = str
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
				if err != nil {
					t.Fatalf("Failed to parse code: %v", err)
				}

				analyzer := NewInterfaceAnalyzer()
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
