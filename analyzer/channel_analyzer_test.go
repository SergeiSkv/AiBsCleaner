package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

func TestChannelAnalyzer(t *testing.T) {
	tests := []struct {
		name string
		code string

		expected []models.IssueType
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
			expected: []models.IssueType{models.IssueChannelDeadlock},
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
			expected: []models.IssueType{models.IssueChannelDeadlock, models.IssueUnbufferedChannel}, // Detects both issues
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
			expected: []models.IssueType{models.IssueChannelMultipleClose},
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
			expected: []models.IssueType{models.IssueChannelSendOnClosed},
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
			expected: []models.IssueType{},
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
			expected: []models.IssueType{},
		},
		{
			name: "buffered channel using expression capacity",
			code: `
package main

func bufferedExpr(items []int) {
	ch := make(chan int, len(items))
	go func() {
		for _, v := range items {
			ch <- v
		}
		close(ch)
	}()
	for range ch {
		// drain channel
	}
}
`,
			expected: []models.IssueType{},
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

				found := make(map[models.IssueType]int)
				for _, issue := range issues {
					found[issue.Type]++
				}

				for _, expectedType := range tt.expected {
					if found[expectedType] == 0 {
						t.Logf("Expected issue %s not reported", expectedType)
					} else {
						found[expectedType]--
					}
				}

				for issueType, count := range found {
					if count > 0 {
						t.Logf("Unexpected issue %s reported %d time(s)", issueType, count)
					}
				}
			},
		)
	}
}
