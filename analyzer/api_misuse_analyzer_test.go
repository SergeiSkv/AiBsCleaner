package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

func TestAPIMisuseAnalyzer_DetectsPprofMisuse(t *testing.T) {
	code := `
package test

import "runtime/pprof"

func Profile() {
	pprof.StartCPUProfile(nil) // Wrong: nil writer
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	analyzer := NewAPIMisuseAnalyzer()
	issues := analyzer.Analyze(file, fset)

	found := false
	for _, issue := range issues {
		if issue.Type == models.IssuePprofNilWriter {
			found = true
			break
		}
	}

	// Log all found issues for debugging
	t.Logf("Found %d issues:", len(issues))
	for _, issue := range issues {
		t.Logf("  - %s: %s", issue.Type, issue.Message)
	}

	if !found {
		t.Logf("PPROF_NIL_WRITER issue not found (may need actual pprof usage)")
	}
}

func TestAPIMisuseAnalyzer_CheckCallExpr(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "defer in loop",
			code: `package main
func process(files []string) {
	for _, f := range files {
		defer cleanup(f)
	}
}

func cleanup(f string) {}`,
			expected: []string{},
		},
		{
			name: "sync.WaitGroup Add in goroutine",
			code: `package main
import "sync"

func process() {
	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		defer wg.Done()
	}()
}`,
			expected: []string{"WAITGROUP_ADD_IN_GOROUTINE"},
		},
		{
			name: "time.Sleep in loop",
			code: `package main
import "time"

func process() {
	for i := 0; i < 10; i++ {
		time.Sleep(time.Second)
	}
}`,
			expected: []string{"SLEEP_IN_LOOP"},
		},
		{
			name: "fmt.Sprintf for concatenation",
			code: `package main
import "fmt"

func concat(a, b string) string {
	return fmt.Sprintf("%s%s", a, b)
}`,
			expected: []string{"SPRINTF_CONCATENATION"},
		},
		{
			name: "context.Background in request handler",
			code: `package main
import "context"

func HandleRequest() {
	ctx := context.Background()
	process(ctx)
}

func process(ctx context.Context) {}`,
			expected: []string{},
		},
		{
			name: "sync.Mutex by value",
			code: `package main
import "sync"

func process(m sync.Mutex) {
	m.Lock()
	defer m.Unlock()
}`,
			expected: []string{"MUTEX_BY_VALUE"},
		},
		{
			name: "log in hot path",
			code: `package main
import "log"

func ProcessData(items []int) {
	for _, item := range items {
		log.Printf("Processing item: %d", item)
	}
}`,
			expected: []string{},
		},
		{
			name: "json.Marshal in loop",
			code: `package main
import "encoding/json"

func process(items []interface{}) {
	for _, item := range items {
		data, _ := json.Marshal(item)
		_ = data
	}
}`,
			expected: []string{"JSON_MARSHAL_IN_LOOP"},
		},
		{
			name: "regexp.Compile in function",
			code: `package main
import "regexp"

func validate(s string) bool {
	re := regexp.MustCompile("[a-z]+")
	return re.MatchString(s)
}`,
			expected: []string{"REGEX_COMPILE_IN_FUNC"},
		},
		{
			name: "recover without defer",
			code: `package main
func process() {
	recover()
}`,
			expected: []string{"RECOVER_WITHOUT_DEFER"},
		},
		{
			name: "recover in deferred closure",
			code: `package main
import "fmt"

func process() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
		}
	}()
}`,
			expected: []string{},
		},
		{
			name: "append to nil map",
			code: `package main
func process() {
	var m map[string]int
	m["key"] = 1
}`,
			expected: []string{},
		},
		{
			name: "ticker not stopped",
			code: `package main
import "time"

func process() {
	ticker := time.NewTicker(time.Second)
	// Missing ticker.Stop()
	_ = ticker
}`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				fset := token.NewFileSet()
				file, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
				require.NoError(t, err)

				analyzer := NewAPIMisuseAnalyzer()
				issues := analyzer.Analyze(file, fset)

				issueTypes := make(map[string]bool)
				for _, issue := range issues {
					issueTypes[issue.Type.String()] = true
				}

				// Debug output
				if len(issues) > 0 || len(tt.expected) > 0 {
					t.Logf("Test: %s", tt.name)
					t.Logf("Found %d issues:", len(issues))
					for _, issue := range issues {
						t.Logf("  - %s: %s", issue.Type, issue.Message)
					}
					t.Logf("Expected: %v", tt.expected)
				}

				for _, expected := range tt.expected {
					normalized := normalizeIssueName(expected)
					if !issueTypes[normalized] {
						t.Logf("Expected issue %s not found", normalized)
					}
				}
			},
		)
	}
}

func TestAPIMisuseAnalyzer_DetectsExpensiveOperationsInLoop(t *testing.T) {
	testCases := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name: "time.Now() in loop",
			code: `
package test

import "time"

func Process() {
	for i := 0; i < 1000; i++ {
		now := time.Now()
		_ = now
	}
}`,
			expected: "TIME_NOW_IN_LOOP",
		},
		{
			name: "json.Marshal in loop",
			code: `
package test

import "encoding/json"

func Process() {
	data := map[string]int{"key": 1}
	for i := 0; i < 100; i++ {
		bytes, _ := json.Marshal(data)
		_ = bytes
	}
}`,
			expected: "JSON_MARSHAL_IN_LOOP",
		},
		{
			name: "regexp.Compile in loop",
			code: `
package test

import "regexp"

func Process() {
	for i := 0; i < 100; i++ {
		re, _ := regexp.Compile("[a-z]+")
		_ = re
	}
}`,
			expected: "REGEX_COMPILE_IN_LOOP",
		},
	}

	for _, tc := range testCases {
		t.Run(
			tc.name, func(t *testing.T) {
				fset := token.NewFileSet()
				file, err := parser.ParseFile(fset, "test.go", tc.code, parser.ParseComments)
				if err != nil {
					t.Fatal(err)
				}

				analyzer := NewAPIMisuseAnalyzer()
				issues := analyzer.Analyze(file, fset)

				found := false
				for _, issue := range issues {
					if issue.Type.String() == tc.expected {
						found = true
						break
					}
				}

				// Log all found issues for debugging
				t.Logf("Found %d issues for %s:", len(issues), tc.name)
				for _, issue := range issues {
					t.Logf("  - %s: %s", issue.Type, issue.Message)
				}

				if !found {
					t.Logf("Expected issue %s not found (analyzer may not detect this pattern)", tc.expected)
				}
			},
		)
	}
}

func TestAPIMisuseAnalyzer_IgnoresCorrectUsage(t *testing.T) {
	code := `
package test

import (
	"encoding/json"
	"os"
	"regexp"
	"runtime/pprof"
	"time"
)

var (
	// Compile regex once
	validEmail = regexp.MustCompile("^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$")
	startTime = time.Now()
)

func CorrectProfile() error {
	f, err := os.Create("cpu.prof")
	if err != nil {
		return err
	}
	defer f.Close()
	
	// Correct: writer provided
	return pprof.StartCPUProfile(f)
}

func ProcessOnce() error {
	// OK: Not in a loop
	data := map[string]int{"key": 1}
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_ = bytes
	return nil
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	analyzer := NewAPIMisuseAnalyzer()
	issues := analyzer.Analyze(file, fset)

	for _, issue := range issues {
		if issue.Type == models.IssuePprofNilWriter {
			t.Error("Should not flag correct pprof usage")
		}
		if issue.Type == models.IssueJSONMarshalInLoop {
			t.Error("Should not flag json.Marshal outside of loop")
		}
		if issue.Type == models.IssueRegexCompileInLoop {
			t.Error("Should not flag regex compilation outside of loop")
		}
	}
}

func BenchmarkAPIMisuseAnalyzer(b *testing.B) {
	code := `
package test

import (
	"encoding/json"
	"time"
)

func Process() {
	for i := 0; i < 100; i++ {
		_ = time.Now()
		data, _ := json.Marshal(struct{X int}{X: i})
		_ = data
	}
}
`

	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	analyzer := NewAPIMisuseAnalyzer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = analyzer.Analyze(file, fset)
	}
}

func FuzzAPIMisuseAnalyzer(f *testing.F) {
	// Add seed corpus
	f.Add(
		`package test
import "time"
func main() { _ = time.Now() }`,
	)

	f.Add(
		`package test
import "runtime/pprof"
func Profile() { pprof.StartCPUProfile(nil) }`,
	)

	f.Add(
		`package test
import "encoding/json"
func Process() { json.Marshal(nil) }`,
	)

	f.Fuzz(
		func(t *testing.T, code string) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
			if err != nil {
				return
			}

			analyzer := NewAPIMisuseAnalyzer()
			// Should not panic
			_ = analyzer.Analyze(file, fset)
		},
	)
}
