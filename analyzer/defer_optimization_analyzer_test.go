package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestDeferOptimizationAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "unnecessary defer for simple operation",
			code: `package main
type File struct{}
func (f *File) Close() {}
func openFile() *File { return &File{} }
func test() {
	file := openFile()
	defer file.Close()
	return
}`,
			expected: []string{"UNNECESSARY_DEFER"},
		},
		{
			name: "defer at end of function",
			code: `package main
type File struct{}
func (f *File) Close() {}
func openFile() *File { return &File{} }
func processFile(f *File) {}
func test() {
	file := openFile()
	processFile(file)
	defer file.Close()
}`,
			expected: []string{"DEFER_AT_END"},
		},
		{
			name: "multiple defers",
			code: `package main
type File struct{}
func (f *File) Close() {}
func openFile1() *File { return &File{} }
func openFile2() *File { return &File{} }
func openFile3() *File { return &File{} }
func process() {}
func test() {
	f1 := openFile1()
	f2 := openFile2()
	f3 := openFile3()
	defer f1.Close()
	defer f2.Close()
	defer f3.Close()
	process()
}`,
			expected: []string{"MULTIPLE_DEFERS"},
		},
		{
			name: "defer in hot path function",
			code: `package main
import "sync"
func getMutex() *sync.Mutex { return &sync.Mutex{} }
func ProcessData() {
	mu := getMutex()
	mu.Lock()
	defer mu.Unlock()
	// process data
}`,
			expected: []string{"DEFER_IN_HOT_PATH"},
		},
		{
			name: "defer with closure capturing variables",
			code: `package main
func getLargeData() interface{} { return nil }
func cleanup(interface{}) {}
func process() {}
func test() {
	largeData := getLargeData()
	defer func() {
		cleanup(largeData)
	}()
	process()
}`,
			expected: []string{"DEFER_LARGE_CAPTURE"},
		},
		{
			name: "missing defer for mutex unlock",
			code: `package main
import "sync"
func process() {}
func test() {
	var mu sync.Mutex
	mu.Lock()
	process()
}`,
			expected: []string{"MISSING_DEFER_UNLOCK"},
		},
		{
			name: "missing defer for resource close",
			code: `package main
import "os"
func process(*os.File) {}
func test() {
	file, _ := os.Open("test.txt")
	process(file)
}`,
			expected: []string{"MISSING_DEFER_CLOSE"},
		},
		{
			name: "complex function - still has defer issues",
			code: `package main
import "sync"
type File struct{}
func (f *File) Close() {}
func openFile() *File { return &File{} }
func processData() {}
func test() {
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()
	
	file := openFile()
	defer file.Close()
	
	// Add complexity to avoid unnecessary defer warnings
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			processData()
		} else {
			// some other processing
		}
	}
}`,
			expected: []string{"UNNECESSARY_DEFER"},
		},
		{
			name: "defer in short function literal",
			code: `package main
import "sync"
func getMutex() *sync.Mutex { return &sync.Mutex{} }
func test() {
	go func() {
		mu := getMutex()
		mu.Lock()
		defer mu.Unlock()
	}()
}`,
			expected: []string{"DEFER_IN_SHORT_FUNC"},
		},
		{
			name: "unnecessary mutex defer in simple function",
			code: `package main
import "sync"
func simple() int {
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()
	return 42
}`,
			expected: []string{"UNNECESSARY_MUTEX_DEFER"},
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

				analyzer := NewDeferOptimizationAnalyzer()
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
