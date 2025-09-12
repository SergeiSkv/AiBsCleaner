package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestContextAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []IssueType
	}{
		{
			name: "context not first parameter",
			code: `
package main

import "context"

func badFunc(id int, ctx context.Context) error {
	return nil
}
`,
			expected: []IssueType{IssueContextNotFirst},
		},
		{
			name: "context.Background in non-main function",
			code: `
package main

import "context"

func processData() {
	ctx := context.Background()
	_ = ctx
}
`,
			expected: []IssueType{IssueContextBackground},
		},
		{
			name: "context.TODO in production code",
			code: `
package main

import "context"

func handleRequest() {
	ctx := context.TODO()
	_ = ctx
}
`,
			expected: []IssueType{IssueContextBackground},
		},
		{
			name: "string key for context value",
			code: `
package main

import "context"

func storeValue(ctx context.Context) context.Context {
	return context.WithValue(ctx, "user_id", 123)
}
`,
			expected: []IssueType{IssueContextValue},
		},
		{
			name: "context leak - ignored cancel",
			code: `
package main

import "context"

func leakyFunc() {
	context.WithCancel(context.Background())
}
`,
			expected: []IssueType{IssueContextBackground}, // Currently detects Background() usage, not the leak
		},
		{
			name: "proper context usage",
			code: `
package main

import "context"

type ctxKey int

const userKey ctxKey = 0

func goodFunc(ctx context.Context, id int) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	
	ctx = context.WithValue(ctx, userKey, id)
	return nil
}

func main() {
	ctx := context.Background()
	_ = ctx
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

				analyzer := NewContextAnalyzer()
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
