package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestChannelAnalyzer(t *testing.T) {
	tests := []struct {
		name string
		code string

		expected []IssueType
	}{
		{
			name: "unbuffered channel deadlock",
			code: `
package main

func deadlock() {
	ch := make(chan int)
	ch <- 1  // Will block forever
}
`,
			expected: []IssueType{IssueChannelDeadlock},
		},
		{
			name: "unbuffered channel in goroutine without select",
			code: `
package main

func process() {
	ch := make(chan int)
	
	go func() {
		ch <- 1  // Potential blocking
	}()
}
`,
			expected: []IssueType{IssueChannelDeadlock, IssueUnbufferedChannel}, // Detects both issues
		},
		{
			name: "multiple channel close",
			code: `
package main

func multiClose() {
	ch := make(chan int)
	close(ch)
	close(ch)  // Panic!
}
`,
			expected: []IssueType{IssueChannelMultipleClose},
		},
		{
			name: "send on closed channel",
			code: `
package main

func sendOnClosed() {
	ch := make(chan int, 1)
	close(ch)
	ch <- 1  // Panic!
}
`,
			expected: []IssueType{IssueChannelSendOnClosed},
		},
		{
			name: "proper channel usage with select",
			code: `
package main

func goodChannel() {
	ch := make(chan int, 1)
	
	go func() {
		select {
		case ch <- 1:
			// Sent successfully
		default:
			// Channel full, skip
		}
	}()
	
	close(ch)
}
`,
			expected: []IssueType{},
		},
		{
			name: "buffered channel usage",
			code: `
package main

func bufferedOK() {
	ch := make(chan int, 10)
	
	go func() {
		ch <- 1  // OK with buffer
	}()
}
`,
			expected: []IssueType{},
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

				analyzer := NewChannelAnalyzer()
				issues := analyzer.Analyze(node, fset)

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
			},
		)
	}
}
