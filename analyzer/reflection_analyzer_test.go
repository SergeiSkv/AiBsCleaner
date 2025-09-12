package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReflectionAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "reflection in loop",
			code: `package main
import "reflect"
func test(items []interface{}) {
	for _, item := range items {
		t := reflect.TypeOf(item)
		_ = t
	}
}`,
			expected: []string{"REFLECTION_IN_HOT_PATH"},
		},
		{
			name: "reflection in for loop",
			code: `package main
import "reflect"
func test() {
	items := []interface{}{1, 2, 3}
	for i := 0; i < len(items); i++ {
		v := reflect.ValueOf(items[i])
		_ = v
	}
}`,
			expected: []string{"REFLECTION_IN_HOT_PATH"},
		},
		{
			name: "reflect.TypeOf in nested loop",
			code: `package main
import "reflect"
func test() {
	data := [][]interface{}{{1, 2}, {3, 4}}
	for _, row := range data {
		for _, item := range row {
			t := reflect.TypeOf(item)
			_ = t
		}
	}
}`,
			expected: []string{"REFLECTION_IN_HOT_PATH"},
		},
		{
			name: "reflect.ValueOf in hot path",
			code: `package main
import "reflect"
func ProcessItems(items []interface{}) {
	for _, item := range items {
		val := reflect.ValueOf(item)
		if val.IsValid() {
			process(val)
		}
	}
}`,
			expected: []string{"REFLECTION_IN_HOT_PATH"},
		},
		{
			name: "reflection outside loop - low severity",
			code: `package main
import "reflect"
func test(item interface{}) {
	t := reflect.TypeOf(item)
	v := reflect.ValueOf(item)
	_ = t
	_ = v
}`,
			expected: []string{"REFLECTION_USAGE"},
		},
		{
			name: "type switch instead of reflection - no issue",
			code: `package main
func test(item interface{}) {
	switch v := item.(type) {
	case int:
		process(v)
	case string:
		process(v)
	}
}`,
			expected: []string{},
		},
		{
			name: "reflection with method call",
			code: `package main
import "reflect"
func test(items []interface{}) {
	for _, item := range items {
		v := reflect.ValueOf(item)
		method := v.MethodByName("String")
		if method.IsValid() {
			method.Call(nil)
		}
	}
}`,
			expected: []string{"REFLECTION_IN_HOT_PATH"},
		},
		{
			name: "no reflection usage",
			code: `package main
func test(items []int) {
	for _, item := range items {
		result := item * 2
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

				analyzer := NewReflectionAnalyzer()
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
