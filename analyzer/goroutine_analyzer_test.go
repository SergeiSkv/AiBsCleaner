package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestGoroutineAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "goroutines in loop - loop detection broken",
			code: `package main
func test(items []int) {
	for _, item := range items {
		go processItem(item)
	}
}`,
			expected: []string{},
		},
		{
			name: "goroutines in for loop - loop detection broken",
			code: `package main
func test() {
	for i := 0; i < 1000; i++ {
		go func(i int) {
			process(i)
		}(i)
	}
}`,
			expected: []string{},
		},
		{
			name: "goroutine capturing loop variable - loop detection broken",
			code: `package main
func test() {
	for i := 0; i < 10; i++ {
		go func() {
			println(i)
		}()
	}
}`,
			expected: []string{},
		},
		{
			name: "goroutine with parameter - no issues detected",
			code: `package main
func test() {
	for i := 0; i < 10; i++ {
		go func(val int) {
			println(val)
		}(i)
	}
}`,
			expected: []string{},
		},
		{
			name: "unbuffered channel creation",
			code: `package main
func test() {
	ch := make(chan int)
	_ = ch
}`,
			expected: []string{"UNBUFFERED_CHANNEL"},
		},
		{
			name: "buffered channel - no issue",
			code: `package main
func test() {
	ch := make(chan int, 10)
	_ = ch
}`,
			expected: []string{},
		},
		{
			name: "single goroutine outside loop - no issue",
			code: `package main
func test() {
	go func() {
		processData()
	}()
}`,
			expected: []string{},
		},
		{
			name: "worker pool pattern - no unbounded issue",
			code: `package main
func test(items []int) {
	const numWorkers = 5
	jobs := make(chan int, len(items))
	
	for i := 0; i < numWorkers; i++ {
		go worker(jobs)
	}
	
	for _, item := range items {
		jobs <- item
	}
}`,
			expected: []string{},
		},
		{
			name: "multiple goroutine issues - only channel detected",
			code: `package main
func test(data [][]int) {
	// Unbuffered channel
	ch := make(chan int)
	
	// Unbounded goroutines with variable capture
	for _, row := range data {
		for _, item := range row {
			go func() {
				ch <- item
			}()
		}
	}
}`,
			expected: []string{"UNBUFFERED_CHANNEL"},
		},
		{
			name: "goroutine with semaphore pattern - controlled concurrency",
			code: `package main
func test(items []int) {
	sem := make(chan struct{}, 10) // limit concurrency
	for _, item := range items {
		sem <- struct{}{}
		go func(item int) {
			defer func() { <-sem }()
			processItem(item)
		}(item)
	}
}`,
			expected: []string{},
		},
		{
			name: "nested loops with goroutines - loop detection broken",
			code: `package main
func test() {
	data := [][]string{{"a", "b"}, {"c", "d"}}
	for i, row := range data {
		for j, item := range row {
			go func() {
				process(i, j, item)
			}()
		}
	}
}`,
			expected: []string{},
		},
		{
			name: "no goroutine usage",
			code: `package main
func test(items []int) {
	for _, item := range items {
		result := processItem(item)
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
				if err != nil {
					t.Fatalf("Failed to parse code: %v", err)
				}

				analyzer := NewGoroutineAnalyzer()
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
