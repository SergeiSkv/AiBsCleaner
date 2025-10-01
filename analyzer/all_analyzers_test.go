//go:build legacytests
// +build legacytests

package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllAnalyzersInitialization(t *testing.T) {
	// Test that all analyzers can be created
	analyzers := []struct {
		name        string
		constructor func() Analyzer
	}{
		{"AIBullshitAnalyzer", NewAIBullshitAnalyzer},
		{"APIMisuseAnalyzer", NewAPIMisuseAnalyzer},
		{"CGOAnalyzer", NewCGOAnalyzer},
		{"ChannelAnalyzer", NewChannelAnalyzer},
		{"ContextAnalyzer", NewContextAnalyzer},
		{"CryptoAnalyzer", NewCryptoAnalyzer},
		{"DatabaseAnalyzer", NewDatabaseAnalyzer},
		{"DeferOptimizationAnalyzer", NewDeferOptimizationAnalyzer},
		{"GoroutineAnalyzer", NewGoroutineAnalyzer},
		{"HTTPClientAnalyzer", NewHTTPClientAnalyzer},
		{"HTTPReuseAnalyzer", NewHTTPReuseAnalyzer},
		{"InterfaceAnalyzer", NewInterfaceAnalyzer},
		{"IOBufferAnalyzer", NewIOBufferAnalyzer},
		{"LoopAnalyzer", NewLoopAnalyzer},
		{"MapAnalyzer", NewMapAnalyzer},
		{"MemoryLeakAnalyzer", NewMemoryLeakAnalyzer},
		{"PrivacyAnalyzer", NewPrivacyAnalyzer},
		{"RaceConditionAnalyzer", NewRaceConditionAnalyzer},
		{"ReflectionAnalyzer", NewReflectionAnalyzer},
		{"RegexAnalyzer", NewRegexAnalyzer},
		{"SerializationAnalyzer", NewSerializationAnalyzer},
		{"SliceAnalyzer", NewSliceAnalyzer},
		{"TimeAnalyzer", NewTimeAnalyzer},
	}

	for _, a := range analyzers {
		t.Run(
			a.name, func(t *testing.T) {
				analyzer := a.constructor()
				assert.NotNil(t, analyzer, "%s should not be nil", a.name)
				assert.NotEmpty(t, analyzer.Name(), "%s should have a name", a.name)
			},
		)
	}
}

func TestAllAnalyzersOnComplexCode(t *testing.T) {
	// Complex code with various issues
	code := `package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"reflect"
	"regexp"
	"sync"
	"time"
)

var globalCounter int

type User struct {
	ID       int
	Password string // Storing password in plain text
	APIKey   string // Hardcoded API key
}

func main() {
	// Multiple performance issues
	for i := 0; i < 1000; i++ {
		// Regex compilation in loop
		re := regexp.MustCompile("[a-z]+")
		_ = re
		
		// Time.Now() in loop
		now := time.Now()
		_ = now
		
		// String concatenation in loop
		s := ""
		for j := 0; j < 100; j++ {
			s += "x"
		}
		
		// Reflection in loop
		t := reflect.TypeOf(i)
		_ = t
		
		// Interface allocation
		var v interface{} = i
		_ = v
		
		// HTTP client in loop
		client := &http.Client{}
		client.Get("http://example.com")
		
		// MD5 usage (weak hash)
		h := md5.New()
		h.Write([]byte("data"))
		
		// Math/rand for security
		token := rand.Intn(1000000)
		_ = token
		
		// Unbuffered channel
		ch := make(chan int)
		go func() {
			ch <- i
		}()
		<-ch
		
		// Mutex not deferred
		var mu sync.Mutex
		mu.Lock()
		globalCounter++
		mu.Unlock()
		
		// SQL without prepared statement
		db, _ := sql.Open("mysql", "")
		query := fmt.Sprintf("SELECT * FROM users WHERE id = %d", i)
		db.Query(query)
		
		// JSON marshal in loop
		data, _ := json.Marshal(User{ID: i})
		_ = data
		
		// File I/O in loop
		ioutil.ReadFile("test.txt")
		
		// Slice append without preallocation
		var slice []int
		for k := 0; k < 100; k++ {
			slice = append(slice, k)
		}
		
		// Map without size hint
		m := make(map[string]int)
		for k := 0; k < 100; k++ {
			m[fmt.Sprintf("%d", k)] = k
		}
		
		// Goroutine leak potential
		go func() {
			for {
				time.Sleep(1 * time.Second)
			}
		}()
	}
	
	// Defer at end of function
	defer fmt.Println("Done")
}`

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	// Test each analyzer
	analyzers := []Analyzer{
		NewAIBullshitAnalyzer(),
		NewAPIMisuseAnalyzer(),
		NewCGOAnalyzer(),
		NewChannelAnalyzer(),
		NewContextAnalyzer(),
		NewCryptoAnalyzer(),
		NewDatabaseAnalyzer(),
		NewDeferOptimizationAnalyzer(),
		NewGoroutineAnalyzer(),
		NewHTTPClientAnalyzer(),
		NewHTTPReuseAnalyzer(),
		NewInterfaceAnalyzer(),
		NewIOBufferAnalyzer(),
		NewLoopAnalyzer(),
		NewMapAnalyzer(),
		NewMemoryLeakAnalyzer(),
		NewPrivacyAnalyzer(),
		NewRaceConditionAnalyzer(),
		NewReflectionAnalyzer(),
		NewRegexAnalyzer(),
		NewSerializationAnalyzer(),
		NewSliceAnalyzer(),
		NewTimeAnalyzer(),
	}

	totalIssues := 0
	for _, analyzer := range analyzers {
		t.Run(
			analyzer.Name(), func(t *testing.T) {
				issues := analyzer.Analyze(node, fset)
				// Most analyzers should find at least some issues in this code
				t.Logf("%s found %d issues", analyzer.Name(), len(issues))
				totalIssues += len(issues)
			},
		)
	}

	// Should find many issues across all analyzers
	assert.Greater(t, totalIssues, 10, "Should find multiple issues in complex code")
}
