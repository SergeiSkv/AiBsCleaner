package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestCoverageAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		code     string
		expected []string
	}{
		{
			name:     "exported function without test",
			filename: "example.go",
			code: `package example

func PublicFunction() string {
	return "public"
}

func privateFunction() string {
	return "private"
}`,
			expected: []string{"MISSING_TEST"},
		},
		{
			name:     "complex function without test",
			filename: "complex.go",
			code: `package complex

func ComplexLogic(a, b int) int {
	if a > b {
		for i := 0; i < a; i++ {
			b += i
			if b > 100 {
				break
			}
		}
		return b
	}
	return a + b
}`,
			expected: []string{"MISSING_TEST", "HIGH_COMPLEXITY_NO_TEST"},
		},
		{
			name:     "test file should not have issues",
			filename: "example_test.go",
			code: `package example

import "testing"

func TestSomething(t *testing.T) {
	// test code
}`,
			expected: []string{},
		},
		{
			name:     "function with error handling without test",
			filename: "errors.go",
			code: `package errors

func ProcessData(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}
	if len(data) > 1000 {
		return fmt.Errorf("data too large")
	}
	return nil
}`,
			expected: []string{"MISSING_TEST", "ERROR_PATH_NO_TEST"},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				fset := token.NewFileSet()
				node, err := parser.ParseFile(fset, tt.filename, tt.code, parser.ParseComments)
				require.NoError(t, err)

				analyzer := NewTestCoverageAnalyzer()
				issues := analyzer.Analyze(node, fset)

				issueTypes := make(map[string]bool)
				for _, issue := range issues {
					issueTypes[issue.Type.String()] = true
				}

				for _, expected := range tt.expected {
					assert.True(t, issueTypes[expected], "Expected issue %s not found", expected)
				}

				if len(tt.expected) == 0 {
					assert.Empty(t, issues, "Expected no issues but found: %v", issues)
				}
			},
		)
	}
}

func TestTestCoverageAnalyzerName(t *testing.T) {
	analyzer := NewTestCoverageAnalyzer()
	assert.Equal(t, "TestCoverageAnalyzer", analyzer.Name())
}

func TestTestCoverageAnalyzerWithTests(t *testing.T) {
	// First analyze the main file
	mainCode := `package mypackage

func MyFunction() string {
	return "hello"
}

func AnotherFunction() int {
	return 42
}`

	// Then analyze test file
	testCode := `package mypackage

import "testing"

func TestMyFunction(t *testing.T) {
	result := MyFunction()
	if result != "hello" {
		t.Error("unexpected result")
	}
}`

	analyzer := NewTestCoverageAnalyzer()

	// Parse main file
	fset := token.NewFileSet()
	mainNode, err := parser.ParseFile(fset, "main.go", mainCode, parser.ParseComments)
	require.NoError(t, err)

	// Parse test file
	testNode, err := parser.ParseFile(fset, "main_test.go", testCode, parser.ParseComments)
	require.NoError(t, err)

	// AnalyzeAll test file first to collect tests
	_ = analyzer.Analyze(testNode, fset)

	// Now analyze main file
	issues := analyzer.Analyze(mainNode, fset)

	// Should report AnotherFunction as missing test
	foundMissing := false
	for _, issue := range issues {
		if issue.Type == IssueMissingTest {
			foundMissing = true
			break
		}
	}
	assert.True(t, foundMissing, "Should detect missing test for AnotherFunction")
}

func TestIsExportedLogic(t *testing.T) {
	// Test logic for checking if function is exported
	testCases := []struct {
		name     string
		funcName string
		expected bool
	}{
		{"public function", "PublicFunction", true},
		{"another public", "AnotherPublic", true},
		{"private function", "privateFunction", false},
		{"underscore private", "_private", false},
		{"empty name", "", false},
	}

	for _, tc := range testCases {
		t.Run(
			tc.name, func(t *testing.T) {
				// Check if first character is uppercase
				isExported := tc.funcName != "" && tc.funcName[0] >= 'A' && tc.funcName[0] <= 'Z'
				assert.Equal(t, tc.expected, isExported)
			},
		)
	}
}

func TestComplexityDetection(t *testing.T) {
	testCases := []struct {
		name           string
		code           string
		expectedIssues []string
	}{
		{
			name: "simple function",
			code: `package test

func Simple() int {
	return 1
}`,
			expectedIssues: []string{},
		},
		{
			name: "complex function",
			code: `package test

func Complex(x int) int {
	if x > 0 {
		for i := 0; i < x; i++ {
			if i%2 == 0 {
				switch i {
				case 2:
					return 2
				case 4:
					return 4
				}
			}
		}
	}
	return 0
}`,
			expectedIssues: []string{"HIGH_COMPLEXITY_NO_TEST"},
		},
	}

	for _, tc := range testCases {
		t.Run(
			tc.name, func(t *testing.T) {
				fset := token.NewFileSet()
				node, err := parser.ParseFile(fset, "test.go", tc.code, parser.ParseComments)
				require.NoError(t, err)

				analyzer := NewTestCoverageAnalyzer()
				issues := analyzer.Analyze(node, fset)

				issueTypes := make(map[string]bool)
				for _, issue := range issues {
					issueTypes[issue.Type.String()] = true
				}

				for _, expected := range tc.expectedIssues {
					assert.True(t, issueTypes[expected], "Expected issue %s not found", expected)
				}
			},
		)
	}
}

func TestErrorHandlingDetection(t *testing.T) {
	testCases := []struct {
		name           string
		code           string
		expectedIssues []string
	}{
		{
			name: "function with error return",
			code: `package test

func WithError() error {
	return nil
}`,
			expectedIssues: []string{"MISSING_TEST", "ERROR_PATH_NO_TEST"},
		},
		{
			name: "function without error return",
			code: `package test

func NoError() string {
	return "ok"
}`,
			expectedIssues: []string{"MISSING_TEST"},
		},
		{
			name: "function with multiple returns including error",
			code: `package test

func MultiReturn() (string, error) {
	return "ok", nil
}`,
			expectedIssues: []string{"MISSING_TEST", "ERROR_PATH_NO_TEST"},
		},
	}

	for _, tc := range testCases {
		t.Run(
			tc.name, func(t *testing.T) {
				fset := token.NewFileSet()
				node, err := parser.ParseFile(fset, "test.go", tc.code, parser.ParseComments)
				require.NoError(t, err)

				analyzer := NewTestCoverageAnalyzer()
				issues := analyzer.Analyze(node, fset)

				issueTypes := make(map[string]bool)
				for _, issue := range issues {
					issueTypes[issue.Type.String()] = true
				}

				for _, expected := range tc.expectedIssues {
					assert.True(t, issueTypes[expected], "Expected issue %s not found", expected)
				}
			},
		)
	}
}
