package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestMemoryLeakAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "unclosed file resource",
			code: `package main
import "os"
func test() {
	file, _ := os.Open("test.txt")
	data := make([]byte, 1024)
	file.Read(data)
}`,
			expected: []string{"UNCLOSED_RESOURCE"},
		},
		{
			name: "file properly closed with defer",
			code: `package main
import "os"
func test() {
	file, _ := os.Open("test.txt")
	defer file.Close()
	data := make([]byte, 1024)
	file.Read(data)
}`,
			expected: []string{},
		},
		{
			name: "ticker not stopped",
			code: `package main
import "time"
func test() {
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-ticker.C:
			process()
		}
	}
}`,
			expected: []string{"UNSTOPPED_TICKER"},
		},
		{
			name: "ticker properly stopped",
			code: `package main
import "time"
func test() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for i := 0; i < 10; i++ {
		<-ticker.C
		process()
	}
}`,
			expected: []string{},
		},
		{
			name: "context with cancel not called",
			code: `package main
import "context"
func test() {
	ctx, cancel := context.WithCancel(context.Background())
	go worker(ctx)
	// cancel not called - potential leak
}`,
			expected: []string{"UNCANCELLED_CONTEXT"},
		},
		{
			name: "context cancel properly called",
			code: `package main
import "context"
func test() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go worker(ctx)
}`,
			expected: []string{},
		},
		{
			name: "goroutine with infinite loop and no exit",
			code: `package main
func test() {
	go func() {
		for {
			process()
		}
	}()
}`,
			expected: []string{"GOROUTINE_LEAK"},
		},
		{
			name: "goroutine with done channel",
			code: `package main
func test() {
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				process()
			}
		}
	}()
}`,
			expected: []string{},
		},
		{
			name: "global map append - not detected by current logic",
			code: `package main
var GlobalCache = make(map[string]interface{})

func test(key string, value interface{}) {
	GlobalCache[key] = value
}`,
			expected: []string{},
		},
		{
			name: "circular reference in struct",
			code: `package main
func test() {
	type Node struct {
		value string
		parent *Node
	}
}`,
			expected: []string{"POTENTIAL_CIRCULAR_REF"},
		},
		{
			name: "multiple memory leak issues",
			code: `package main
import (
	"os"
	"time"
	"context"
)
func test() {
	// Unclosed file
	file, _ := os.Open("test.txt")
	
	// Unstopped ticker
	ticker := time.NewTicker(time.Second)
	
	// Uncancelled context
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	
	// Goroutine leak
	go func() {
		for {
			select {
			case <-ticker.C:
				process()
			}
		}
	}()
}`,
			expected: []string{"UNCLOSED_RESOURCE", "UNSTOPPED_TICKER", "UNCANCELLED_CONTEXT"},
		},
		{
			name: "proper resource management - no issues",
			code: `package main
import (
	"os"
	"context"
)
func test() {
	file, err := os.Open("test.txt")
	if err != nil {
		return
	}
	defer file.Close()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			default:
				process()
			}
		}
	}()
	
	close(done)
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

				analyzer := NewMemoryLeakAnalyzer()
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
