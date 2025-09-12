package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestTimeAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "time.Now in loop",
			code: `package main
import "time"
func test() {
	for i := 0; i < 100; i++ {
		now := time.Now()
		_ = now
	}
}`,
			expected: []string{"TIME_NOW_IN_LOOP"},
		},
		{
			name: "time.Now in range loop",
			code: `package main
import "time"
func test(items []int) {
	for _, item := range items {
		now := time.Now()
		processItem(item, now)
	}
}`,
			expected: []string{"TIME_NOW_IN_LOOP"},
		},
		{
			name: "time.Format in loop",
			code: `package main
import "time"
func test(timestamps []time.Time) {
	for _, ts := range timestamps {
		formatted := ts.Format("2006-01-02")
		_ = formatted
	}
}`,
			expected: []string{"TIME_FORMAT_IN_LOOP"},
		},
		{
			name: "time.Now outside loop - no issue",
			code: `package main
import "time"
func test() {
	now := time.Now()
	for i := 0; i < 100; i++ {
		processItem(i, now)
	}
}`,
			expected: []string{},
		},
		{
			name: "time operations outside loop - no issue",
			code: `package main
import "time"
func test() {
	now := time.Now()
	formatted := now.Format("2006-01-02 15:04:05")
	_ = formatted
}`,
			expected: []string{},
		},
		{
			name: "nested loops with time operations",
			code: `package main
import "time"
func test() {
	data := [][]int{{1, 2}, {3, 4}}
	for _, row := range data {
		for _, item := range row {
			now := time.Now()
			process(item, now)
		}
	}
}`,
			expected: []string{"TIME_NOW_IN_LOOP"},
		},
		{
			name: "multiple time issues in loop",
			code: `package main
import "time"
func test(items []int) {
	for _, item := range items {
		now := time.Now()
		formatted := now.Format("2006-01-02")
		process(item, formatted)
	}
}`,
			expected: []string{"TIME_NOW_IN_LOOP", "TIME_FORMAT_IN_LOOP"},
		},
		{
			name: "time comparison in loop - no issue",
			code: `package main
import "time"
func test(timestamps []time.Time) {
	deadline := time.Now().Add(time.Hour)
	for _, ts := range timestamps {
		if ts.Before(deadline) {
			process(ts)
		}
	}
}`,
			expected: []string{},
		},
		{
			name: "time.Since in loop",
			code: `package main
import "time"
func test(start time.Time) {
	for i := 0; i < 100; i++ {
		elapsed := time.Since(start)
		if elapsed > time.Second {
			break
		}
		process(i)
	}
}`,
			expected: []string{},
		},
		{
			name: "time parsing in loop",
			code: `package main
import "time"
func test(dateStrings []string) {
	for _, dateStr := range dateStrings {
		parsed, _ := time.Parse("2006-01-02", dateStr)
		_ = parsed
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

				analyzer := NewTimeAnalyzer()
				issues := analyzer.Analyze(node, fset)

				issueTypes := make(map[string]bool)
				for _, issue := range issues {
					issueTypes[issue.Type.String()] = true
				}

				for _, expected := range tt.expected {
					if !issueTypes[expected] {
						t.Errorf("Expected issue %s not found", expected)
					}
				}

				if len(tt.expected) == 0 && len(issues) > 0 {
					t.Errorf("Expected no issues, but found %d", len(issues))
				}
			},
		)
	}
}
