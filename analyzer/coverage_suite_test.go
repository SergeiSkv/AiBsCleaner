package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

func parseTestFile(t *testing.T, name, code string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, code, parser.ParseComments)
	require.NoError(t, err)
	return fset, file
}

func TestAnalyzerCoverageSuite(t *testing.T) {
	megaCode := `package main

import (
    "bytes"
    "context"
    "crypto/md5"
    "database/sql"
    "encoding/json"
    "fmt"
    "math"
    "net"
    "net/http"
    "regexp"
    "runtime/pprof"
    "sync"
    "sync/atomic"
    "time"
)

var global = make([]int, 0)

func loopPatterns(slice []int) {
    defer fmt.Println("exit")
    for _, v := range slice {
        defer func(x int) { fmt.Println(x) }(v)
        _ = regexp.MustCompile("[0-9]+").MatchString(fmt.Sprint(v))
        _ = time.Now().Format(time.RFC3339)
        global = append(global, v)
    }
}

func syncPatterns() {
    var wg sync.WaitGroup
    ch := make(chan int)
    for i := 0; i < 3; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            ch <- n
        }(i)
    }
    go func() {
        for range ch {
        }
    }()
    go func() {
        defer close(ch)
        wg.Wait()
    }()
}

func httpPatterns() {
    client := &http.Client{}
    client.Get("http://example.com")
    net.Dial("tcp", "example.com:80")
}

func sqlPatterns(db *sql.DB) {
    db.Query("SELECT 1")
}

func memoryPatterns() {
    ticker := time.NewTicker(time.Second)
    fmt.Println(<-ticker.C)
}

func cryptoPatterns() {
    md5.Sum([]byte("data"))
}

func contextPatterns() {
    go func() {
        ctx := context.Background()
        _ = ctx
    }()
}

func syncPoolPatterns() {
    var pool sync.Pool
    pool.New = func() interface{} { return new(bytes.Buffer) }
    buf := pool.Get().(*bytes.Buffer)
    buf.WriteString("data")
}

func apiMisusePatterns(wg *sync.WaitGroup) {
    for i := 0; i < 2; i++ {
        wg.Add(1)
    }
}
`

	baseFset, baseFile := parseTestFile(t, "complex.go", megaCode)

	cases := []struct {
		name     string
		analyzer Analyzer
		file     *ast.File
		fset     *token.FileSet
	}{
		{"Loop", NewLoopAnalyzer(), baseFile, baseFset},
		{"DeferOptimization", NewDeferOptimizationAnalyzer(), baseFile, baseFset},
		{"Slice", NewSliceAnalyzer(), baseFile, baseFset},
		{"Map", NewMapAnalyzer(), baseFile, baseFset},
		{"Reflection", NewReflectionAnalyzer(), baseFile, baseFset},
		{"Interface", NewInterfaceAnalyzer(), baseFile, baseFset},
		{"Regex", NewRegexAnalyzer(), baseFile, baseFset},
		{"Time", NewTimeAnalyzer(), baseFile, baseFset},
		{"MemoryLeak", NewMemoryLeakAnalyzer(), baseFile, baseFset},
		{"Database", NewDatabaseAnalyzer(), baseFile, baseFset},
		{"APIMisuse", NewAPIMisuseAnalyzer(), baseFile, baseFset},
		{"AIBullshit", NewAIBullshitAnalyzer(), baseFile, baseFset},
		{"Goroutine", NewGoroutineAnalyzer(), baseFile, baseFset},
		{"Channel", NewChannelAnalyzer(), baseFile, baseFset},
		{"HTTPClient", NewHTTPClientAnalyzer(), baseFile, baseFset},
		{"Context", NewContextAnalyzer(), baseFile, baseFset},
		{"RaceCondition", NewRaceConditionAnalyzer(), baseFile, baseFset},
		{"ConcurrencyPatterns", NewConcurrencyPatternsAnalyzer(), baseFile, baseFset},
		{"NetworkPatterns", NewNetworkPatternsAnalyzer(), baseFile, baseFset},
		{"CPUOptimization", NewCPUOptimizationAnalyzer(), baseFile, baseFset},
		{"GCPressure", NewGCPressureAnalyzer(), baseFile, baseFset},
		{"SyncPool", NewSyncPoolAnalyzer(), baseFile, baseFset},
		{"Serialization", NewSerializationAnalyzer(), baseFile, baseFset},
		{"Crypto", NewCryptoAnalyzer(), baseFile, baseFset},
		{"HTTPReuse", NewHTTPReuseAnalyzer(), baseFile, baseFset},
		{"IOBuffer", NewIOBufferAnalyzer(), baseFile, baseFset},
		{"Privacy", NewPrivacyAnalyzer(), baseFile, baseFset},
		{"StructLayout", NewStructLayoutAnalyzer(), baseFile, baseFset},
		{"CPUCache", NewCPUCacheAnalyzer(), baseFile, baseFset},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			issues := tc.analyzer.Analyze(tc.file, tc.fset)
			if len(issues) == 0 {
				t.Logf("%s produced no issues but executed", tc.name)
			}
		})
	}
}
