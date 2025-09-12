package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRaceConditionAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "shared variable without sync",
			code: `package main
var counter int

func increment() {
	counter++ // Race condition
}

func test() {
	go increment()
	go increment()
}`,
			expected: []string{"RACE_CONDITION_INCDEC"},
		},
		{
			name: "map concurrent access",
			code: `package main
func test() {
	mapData := make(map[string]int)
	
	go func() {
		mapData["key"] = 1
	}()
	
	go func() {
		_ = mapData["key"]
	}()
}`,
			expected: []string{"CONCURRENT_MAP_ACCESS"},
		},
		{
			name: "proper mutex usage",
			code: `package main
import "sync"

var (
	counter int
	mu sync.Mutex
)

func increment() {
	mu.Lock()
	defer mu.Unlock()
	counter++
}

func test() {
	go increment()
	go increment()
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

				analyzer := NewRaceConditionAnalyzer()
				issues := analyzer.Analyze(node, fset)

				issueTypes := make(map[string]bool)
				for _, issue := range issues {
					issueTypes[issue.Type.String()] = true
				}

				for _, expected := range tt.expected {
					assert.True(t, issueTypes[expected], "Expected issue %s not found", expected)
				}

				if len(tt.expected) == 0 {
					assert.Empty(t, issues, "Expected no issues")
				}
			},
		)
	}
}
