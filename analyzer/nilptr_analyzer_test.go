package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestNilPtrAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "potential nil dereference - not detected for local vars",
			code: `package main
func test() {
	var ptr *string
	println(*ptr)
}`,
			expected: []string{},
		},
		{
			name: "nil check before dereference - no issue",
			code: `package main
func test(ptr *string) {
	if ptr != nil {
		println(*ptr)
	}
}`,
			expected: []string{},
		},
		{
			name: "unchecked type assertion - detected correctly",
			code: `package main
func test(data interface{}) {
	str := data.(string)
	println(str)
}`,
			expected: []string{"UNCHECKED_TYPE_ASSERTION"},
		},
		{
			name: "safe type assertion with ok check",
			code: `package main
func test(data interface{}) {
	if str, ok := data.(string); ok {
		println(str)
	}
}`,
			expected: []string{},
		},
		{
			name: "potential nil map access - not detected for local vars",
			code: `package main
func test() {
	var m map[string]int
	value := m["key"]
	println(value)
}`,
			expected: []string{},
		},
		{
			name: "nil check before map access",
			code: `package main
func test(m map[string]int) {
	if m != nil {
		value := m["key"]
		println(value)
	}
}`,
			expected: []string{},
		},
		{
			name: "range over nil slice - not detected for local vars",
			code: `package main
func test() {
	var items []string
	for _, item := range items {
		println(item)
	}
}`,
			expected: []string{},
		},
		{
			name: "method call on potentially nil receiver - not detected for local vars",
			code: `package main
type MyStruct struct {
	value string
}

func (m *MyStruct) GetValue() string {
	return m.value
}

func test() {
	var obj *MyStruct
	result := obj.GetValue()
	println(result)
}`,
			expected: []string{},
		},
		{
			name: "unchecked pointer parameter - detects param issue",
			code: `package main
func test(ptr *string) {
	println(*ptr)
}`,
			expected: []string{"UNCHECKED_PARAM"},
		},
		{
			name: "type switch assignment - no unchecked assertion",
			code: `package main
func test(data interface{}) {
	switch v := data.(type) {
	case string:
		println(v)
	case int:
		println(v)
	}
}`,
			expected: []string{},
		},
		{
			name: "function returning potential nil",
			code: `package main
import "os"
func test() {
	file, _ := os.Open("nonexistent.txt")
	file.Read(nil)
}`,
			expected: []string{"NIL_METHOD_CALL"},
		},
		{
			name: "nil check after function call - analyzer still flags it",
			code: `package main
import "os"
func test() {
	file, err := os.Open("test.txt")
	if err == nil && file != nil {
		file.Read(nil)
		file.Close()
	}
}`,
			expected: []string{"NIL_METHOD_CALL", "POTENTIAL_NIL_DEREF"},
		},
		{
			name: "multiple nil pointer issues",
			code: `package main
func test(ptr *string, data interface{}) {
	// Unchecked parameter
	value := *ptr
	
	// Unchecked type assertion
	str := data.(string)
	
	// Potential nil map
	var m map[string]int
	result := m["key"]
	
	println(value, str, result)
}`,
			expected: []string{"UNCHECKED_PARAM"},
		},
		{
			name: "proper nil handling - no issues",
			code: `package main
func test(ptr *string, data interface{}) {
	if ptr != nil {
		value := *ptr
		println(value)
	}
	
	if str, ok := data.(string); ok {
		println(str)
	}
	
	m := make(map[string]int)
	result := m["key"]
	println(result)
}`,
			expected: []string{},
		},
		{
			name: "slice field access on nil struct - not detected for local vars",
			code: `package main
type Container struct {
	items []string
}

func test() {
	var c *Container
	println(len(c.items))
}`,
			expected: []string{},
		},
		{
			name: "interface method call",
			code: `package main
type Reader interface {
	Read() string
}

func test(r Reader) {
	data := r.Read()
	println(data)
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

				analyzer := NewNilPtrAnalyzer()
				issues := analyzer.Analyze(node, fset)

				issueTypes := make(map[string]bool)
				for _, issue := range issues {
					issueTypes[issue.Type.String()] = true
				}

				for _, expected := range tt.expected {
					if !issueTypes[expected] {
						t.Errorf("Expected issue %s not found", expected)
					}
				}

				if len(tt.expected) == 0 && len(issues) > 0 {
					t.Errorf("Expected no issues, but found %d", len(issues))
				}
			},
		)
	}
}
