package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncPoolAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "pool get without put",
			code: `package main
import "sync"

var pool = &sync.Pool{}

func ProcessData() {
	obj := pool.Get()
	// Missing pool.Put(obj)
	_ = obj
}`,
			expected: []string{"POOL_GET_WITHOUT_PUT"},
		},
		{
			name: "buffer allocation in hot path",
			code: `package main
import "bytes"

func HandleRequest() {
	for i := 0; i < 100; i++ {
		buf := bytes.NewBuffer(nil)
		buf.WriteString("data")
	}
}`,
			expected: []string{"BUFFER_ALLOCATION_IN_HOT_PATH"},
		},
		{
			name: "frequent allocations in hot path",
			code: `package main

func ProcessItems() {
	for i := 0; i < 10; i++ {
		data := make([]byte, 1024)
		item := make(map[string]string)
		result := new(struct{})
		wrapper := &struct{ data []byte }{data: data}
		_ = item
		_ = result
		_ = wrapper
	}
}`,
			expected: []string{"MISSING_SYNC_POOL"},
		},
		{
			name: "proper pool usage",
			code: `package main
import "sync"

var pool = &sync.Pool{
	New: func() interface{} {
		return make([]byte, 1024)
	},
}

func test() {
	obj := pool.Get()
	defer pool.Put(obj)
	// Use obj
}`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
			require.NoError(t, err)

			analyzer := NewSyncPoolAnalyzer()
			issues := analyzer.Analyze(node, fset)

			issueTypes := make(map[string]bool)
			for _, issue := range issues {
				issueTypes[issue.Type.String()] = true
			}

			for _, expected := range tt.expected {
				assert.True(t, issueTypes[expected], "Expected issue %s not found", expected)
			}

			if len(tt.expected) == 0 {
				assert.Empty(t, issues, "Expected no issues but found: %v", issues)
			}
		})
	}
}
