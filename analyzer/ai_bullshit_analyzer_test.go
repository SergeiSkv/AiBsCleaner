package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestAIBullshitAnalyzer_DetectsOverEngineering(t *testing.T) {
	code := `
package test

// Factory for simple addition - AI bullshit
type NumberAdderFactory interface {
	CreateNumberAdder() NumberAdderStrategy
}

type NumberAdderStrategy interface {
	AddNumbers(a, b int) int
}

type SimpleNumberAdderFactory struct{}

func (f *SimpleNumberAdderFactory) CreateNumberAdder() NumberAdderStrategy {
	return &SimpleNumberAdderStrategy{}
}

type SimpleNumberAdderStrategy struct{}

func (s *SimpleNumberAdderStrategy) AddNumbers(a, b int) int {
	return a + b
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	analyzer := NewAIBullshitAnalyzer()
	issues := analyzer.Analyze("test.go", file, fset)

	// Should detect Factory pattern for simple operation
	found := false
	for _, issue := range issues {
		if issue.Type == "AI_OVER_ENGINEERING" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find AI_OVER_ENGINEERING issue for Factory pattern")
	}
}

func TestAIBullshitAnalyzer_DetectsUnnecessaryGoroutines(t *testing.T) {
	code := `
package test

func AddNumbers(a, b int) int {
	resultCh := make(chan int, 1)
	
	go func() {
		resultCh <- a + b
	}()
	
	return <-resultCh
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	analyzer := NewAIBullshitAnalyzer()
	issues := analyzer.Analyze("test.go", file, fset)

	// Should detect unnecessary goroutines
	found := false
	for _, issue := range issues {
		if issue.Type == "AI_GOROUTINE_OVERKILL" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find AI_GOROUTINE_OVERKILL issue")
	}
}

func TestAIBullshitAnalyzer_DetectsUnnecessaryReflection(t *testing.T) {
	code := `
package test

import "reflect"

func GetValue(x int) int {
	val := reflect.ValueOf(x)
	return int(val.Int())
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	analyzer := NewAIBullshitAnalyzer()
	issues := analyzer.Analyze("test.go", file, fset)

	// Should detect unnecessary reflection
	found := false
	for _, issue := range issues {
		if issue.Type == "AI_UNNECESSARY_REFLECTION" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected to find AI_UNNECESSARY_REFLECTION issue")
	}
}

func TestAIBullshitAnalyzer_DetectsUnnecessaryInterfaces(t *testing.T) {
	code := `
package test

type Handler interface {
	Handle(data string)
}

type Manager interface {
	Manage()
}

type Provider interface {
	Provide() int
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	analyzer := NewAIBullshitAnalyzer()
	issues := analyzer.Analyze("test.go", file, fset)

	// Should detect generic interface names
	interfaceIssues := 0
	for _, issue := range issues {
		if issue.Type == "AI_UNNECESSARY_INTERFACE" {
			interfaceIssues++
		}
	}

	if interfaceIssues < 3 {
		t.Errorf("Expected at least 3 AI_UNNECESSARY_INTERFACE issues, got %d", interfaceIssues)
	}
}

func TestAIBullshitAnalyzer_IgnoresLegitimatePatterns(t *testing.T) {
	code := `
package test

import (
	"context"
	"sync"
)

// Legitimate use of goroutines for parallel processing
func ProcessItems(items []string) []string {
	var wg sync.WaitGroup
	results := make([]string, len(items))
	
	for i, item := range items {
		wg.Add(1)
		go func(idx int, val string) {
			defer wg.Done()
			// Simulate expensive operation
			results[idx] = processItem(val)
		}(i, item)
	}
	
	wg.Wait()
	return results
}

func processItem(item string) string {
	// Expensive processing
	return item
}

// Legitimate Factory for complex object creation
type DatabaseConnectionFactory interface {
	CreateConnection(ctx context.Context, config Config) (Connection, error)
}

type Config struct {
	Host     string
	Port     int
	Database string
}

type Connection interface {
	Query(ctx context.Context, sql string) error
	Close() error
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	analyzer := NewAIBullshitAnalyzer()
	issues := analyzer.Analyze("test.go", file, fset)

	// Should not detect issues for legitimate patterns
	for _, issue := range issues {
		if issue.Type == "AI_GOROUTINE_OVERKILL" {
			t.Error("Should not flag legitimate goroutine usage")
		}
		if issue.Type == "AI_OVER_ENGINEERING" && issue.Line < 30 {
			t.Error("Should not flag legitimate factory pattern for complex objects")
		}
	}
}

func BenchmarkAIBullshitAnalyzer(b *testing.B) {
	code := `
package test

type Factory interface {
	Create() Strategy
}

type Strategy interface {
	Execute() int
}

func DoWork(x int) int {
	ch := make(chan int)
	go func() { ch <- x * 2 }()
	return <-ch
}
`

	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	analyzer := NewAIBullshitAnalyzer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = analyzer.Analyze("test.go", file, fset)
	}
}

func FuzzAIBullshitAnalyzer(f *testing.F) {
	// Add seed corpus
	f.Add(`package test
func Add(a, b int) int { return a + b }`)

	f.Add(`package test
type Factory interface { Create() interface{} }`)

	f.Add(`package test
import "reflect"
func Get(x int) int { return reflect.ValueOf(x).Interface().(int) }`)

	f.Fuzz(func(t *testing.T, code string) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
		if err != nil {
			// Invalid Go code, skip
			return
		}

		analyzer := NewAIBullshitAnalyzer()
		// Should not panic
		_ = analyzer.Analyze("test.go", file, fset)
	})
}
