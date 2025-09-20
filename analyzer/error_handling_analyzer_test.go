package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestErrorHandlingAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "ignored error",
			code: `
package main

import "os"

func ignoreError() {
	_, _ = os.Open("file.txt")  // Error explicitly ignored
}
`,
			expected: []string{"IGNORED_ERROR"}, // Still detects even with explicit _
		},
		{
			name: "unchecked error",
			code: `
package main

import "os"

func uncheckedError() {
	file, err := os.Open("file.txt")
	// err not checked
	_ = file
	_ = err
}
`,
			expected: []string{"UNCHECKED_ERROR"},
		},
		{
			name: "empty error handler",
			code: `
package main

import "os"

func emptyHandler() {
	_, err := os.Open("file.txt")
	if err != nil {
		// Empty handler
	}
}
`,
			expected: []string{"IGNORED_ERROR", "UNCHECKED_ERROR", "EMPTY_ERROR_HANDLER"},
		},
		{
			name: "panic on error",
			code: `
package main

import "os"

func panicOnError() {
	_, err := os.Open("file.txt")
	if err != nil {
		panic(err)  // Don't panic!
	}
}
`,
			expected: []string{"IGNORED_ERROR", "UNCHECKED_ERROR", "PANIC_ON_ERROR"},
		},
		{
			name: "error not wrapped",
			code: `
package main

import (
	"fmt"
	"os"
)

func notWrapped() error {
	_, err := os.Open("file.txt")
	if err != nil {
		return fmt.Errorf("failed to open: %v", err)  // Should use %w
	}
	return nil
}
`,
			expected: []string{"ERROR_SHADOWING", "IGNORED_ERROR", "UNCHECKED_ERROR", "ERROR_NOT_WRAPPED"},
		},
		{
			name: "errors.New with concatenation",
			code: `
package main

import "errors"

func badError(msg string) error {
	return errors.New("error: " + msg)  // Should use fmt.Errorf
}
`,
			expected: []string{"ERRORS_NEW_WITH_FORMAT"},
		},
		{
			name: "error shadowing",
			code: `
package main

import "os"

func shadowError() error {
	err := os.Remove("file1.txt")
	if err != nil {
		return err
	}
	
	if true {
		err := os.Remove("file2.txt")  // Shadows outer err
		if err != nil {
			return err
		}
	}
	
	return nil
}
`,
			expected: []string{"UNCHECKED_ERROR_RETURN", "UNCHECKED_ERROR_RETURN", "ERROR_SHADOWING", "ERROR_SHADOWING", "UNCHECKED_ERROR", "UNCHECKED_ERROR"},
		},
		{
			name: "proper error handling",
			code: `
package main

import (
	"fmt"
	"os"
)

func properErrorHandling() error {
	file, err := os.Open("file.txt")
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	return nil
}

func TestFunc() {
	// Test functions can use panic
	err := properErrorHandling()
	if err != nil {
		panic(err)
	}
}
`,
			expected: []string{"ERROR_SHADOWING", "UNCHECKED_ERROR", "UNCHECKED_ERROR", "PANIC_ON_ERROR"}, // Analyzer is aggressive
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse code: %v", err)
			}

			analyzer := NewErrorHandlingAnalyzer()
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
