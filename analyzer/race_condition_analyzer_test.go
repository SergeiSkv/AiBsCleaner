package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestRaceConditionAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "increment without atomic",
			code: `
package main

var counter int

func incrementCounter() {
	go func() {
		counter++  // Race condition
	}()
}
`,
			expected: []string{"RACE_CONDITION_INCDEC", "RACE_CONDITION"}, // Detects both the inc/dec and general race
		},
		{
			name: "concurrent map access",
			code: `
package main

func mapRace() {
	m := make(map[string]int)
	
	go func() {
		m["key"] = 1  // Concurrent map write
	}()
	
	go func() {
		_ = m["key"]  // Concurrent map read
	}()
}
`,
			expected: []string{"RACE_CONDITION"}, // Detects m as shared variable
		},
		{
			name: "concurrent slice append",
			code: `
package main

func sliceRace() {
	var data []int
	
	go func() {
		data = append(data, 1)  // Race on slice
	}()
}
`,
			expected: []string{"RACE_CONDITION", "CONCURRENT_SLICE_APPEND"}, // Detects both issues
		},
		{
			name: "global variable modification without sync",
			code: `
package main

var GlobalData string

func ModifyGlobal() {
	GlobalData = "modified"  // No synchronization
}
`,
			expected: []string{"RACE_CONDITION_GLOBAL"},
		},
		{
			name: "proper synchronization with mutex",
			code: `
package main

import "sync"

var (
	mu      sync.Mutex
	counter int
)

func safeIncrement() {
	mu.Lock()
	defer mu.Unlock()
	counter++
}

func safeGoroutine() {
	var wg sync.WaitGroup
	
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Local slice creation, no shared state
		localData := make([]int, 0)
		localData = append(localData, 1)
		_ = localData
	}()
	
	wg.Wait()
}
`,
			expected: []string{"RACE_CONDITION", "RACE_CONDITION", "CONCURRENT_SLICE_APPEND"}, // Detects localData twice + append
		},
		{
			name: "atomic operations",
			code: `
package main

import "sync/atomic"

var counter int64

func atomicIncrement() {
	go func() {
		atomic.AddInt64(&counter, 1)  // Safe
	}()
}
`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse code: %v", err)
			}

			analyzer := NewRaceConditionAnalyzer()
			issues := analyzer.Analyze("test.go", node, fset)

			if len(issues) != len(tt.expected) {
				t.Errorf("Expected %d issues, got %d", len(tt.expected), len(issues))
				for _, issue := range issues {
					t.Logf("Got issue: %s - %s", issue.Type, issue.Message)
				}
				return
			}

			for i, expectedType := range tt.expected {
				if i >= len(issues) {
					t.Errorf("Missing expected issue: %s", expectedType)
					continue
				}
				if issues[i].Type != expectedType {
					t.Errorf("Expected issue type %s, got %s", expectedType, issues[i].Type)
				}
			}
		})
	}
}
