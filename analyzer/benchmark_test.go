package analyzer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"sync/atomic"
	"testing"
)

// Sample code for benchmarking
const benchmarkCode = `package main

import (
	"fmt"
	"time"
	"sync"
	"encoding/json"
	"regexp"
	"strings"
	"database/sql"
)

var globalMutex sync.Mutex
var globalData []int

func ComplexFunction() {
	// Multiple nested loops
	for i := 0; i < 100; i++ {
		for j := 0; j < 100; j++ {
			for k := 0; k < 100; k++ {
				data := make([]int, 100)
				data[i%100] = i * j * k
				globalData = append(globalData, data...)
			}
		}
	}
	
	// String concatenation in loop
	result := ""
	for i := 0; i < 1000; i++ {
		result += fmt.Sprintf("item-%d", i)
	}
	
	// Regex compilation in loop
	for i := 0; i < 100; i++ {
		re := regexp.MustCompile("[a-z]+")
		_ = re.MatchString("test")
	}
	
	// JSON marshaling in loop
	for i := 0; i < 100; i++ {
		data := map[string]int{"key": i}
		json.Marshal(data)
	}
	
	// Time operations in loop
	for i := 0; i < 1000; i++ {
		_ = time.Now()
	}
	
	// Goroutines without limit
	for i := 0; i < 1000; i++ {
		go func(n int) {
			time.Sleep(time.Millisecond)
			println(n)
		}(i)
	}
}

func StringOperations() {
	var builder strings.Builder
	for i := 0; i < 10000; i++ {
		builder.WriteString("test")
	}
	_ = builder.String()
}
`

// BenchmarkMainAnalyze tests the main AnalyzeAll function
func BenchmarkMainAnalyze(b *testing.B) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "bench.go", benchmarkCode, parser.ParseComments)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(
		func(pb *testing.PB) {
			for pb.Next() {
				_ = AnalyzeAll("bench.go", file, fset)
			}
		},
	)
}

// BenchmarkAnalyzeWithCache tests caching performance
func BenchmarkAnalyzeWithCache(b *testing.B) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "bench.go", benchmarkCode, parser.ParseComments)
	if err != nil {
		b.Fatal(err)
	}

	// First call to populate the cache
	_ = AnalyzeAll("bench_cache.go", file, fset)

	b.ResetTimer()
	b.RunParallel(
		func(pb *testing.PB) {
			for pb.Next() {
				_ = AnalyzeAll("bench_cache.go", file, fset)
			}
		},
	)
}

// Individual analyzer benchmarks
func BenchmarkLoopAnalyzer(b *testing.B) {
	benchmarkAnalyzer(b, NewLoopAnalyzer())
}

func BenchmarkDeferOptimizationAnalyzer(b *testing.B) {
	benchmarkAnalyzer(b, NewDeferOptimizationAnalyzer())
}

func BenchmarkSliceAnalyzer(b *testing.B) {
	benchmarkAnalyzer(b, NewSliceAnalyzer())
}

func BenchmarkMapAnalyzer(b *testing.B) {
	benchmarkAnalyzer(b, NewMapAnalyzer())
}

func BenchmarkReflectionAnalyzer(b *testing.B) {
	benchmarkAnalyzer(b, NewReflectionAnalyzer())
}

func BenchmarkInterfaceAnalyzer(b *testing.B) {
	benchmarkAnalyzer(b, NewInterfaceAnalyzer())
}

func BenchmarkRegexAnalyzer(b *testing.B) {
	benchmarkAnalyzer(b, NewRegexAnalyzer())
}

func BenchmarkTimeAnalyzer(b *testing.B) {
	benchmarkAnalyzer(b, NewTimeAnalyzer())
}

func BenchmarkMemoryLeakAnalyzer(b *testing.B) {
	benchmarkAnalyzer(b, NewMemoryLeakAnalyzer())
}

func BenchmarkDatabaseAnalyzer(b *testing.B) {
	benchmarkAnalyzer(b, NewDatabaseAnalyzer())
}

func BenchmarkGoroutineAnalyzer(b *testing.B) {
	benchmarkAnalyzer(b, NewGoroutineAnalyzer())
}

func BenchmarkChannelAnalyzer(b *testing.B) {
	benchmarkAnalyzer(b, NewChannelAnalyzer())
}

func BenchmarkHTTPClientAnalyzer(b *testing.B) {
	benchmarkAnalyzer(b, NewHTTPClientAnalyzer())
}

func BenchmarkCryptoAnalyzer(b *testing.B) {
	benchmarkAnalyzer(b, NewCryptoAnalyzer())
}

func BenchmarkSerializationAnalyzer(b *testing.B) {
	benchmarkAnalyzer(b, NewSerializationAnalyzer())
}

func BenchmarkAPIMisuse(b *testing.B) {
	benchmarkAnalyzer(b, NewAPIMisuseAnalyzer())
}

func BenchmarkAIBullshit(b *testing.B) {
	benchmarkAnalyzer(b, NewAIBullshitAnalyzer())
}

func BenchmarkGCPressureAnalyzer(b *testing.B) {
	benchmarkAnalyzer(b, NewGCPressureAnalyzer())
}

func BenchmarkRaceConditionAnalyzer(b *testing.B) {
	benchmarkAnalyzer(b, NewRaceConditionAnalyzer())
}

// Helper function to benchmark individual analyzers
func benchmarkAnalyzer(b *testing.B, analyzer Analyzer) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "bench.go", benchmarkCode, parser.ParseComments)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(
		func(pb *testing.PB) {
			for pb.Next() {
				_ = analyzer.Analyze(file, fset)
			}
		},
	)
}

// Benchmark for WalkWithContext
func BenchmarkWalkWithContext(b *testing.B) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "bench.go", benchmarkCode, parser.ParseComments)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(
		func(pb *testing.PB) {
			for pb.Next() {
				WalkWithContext(
					file, func(node ast.Node, ctx *AnalysisContext) bool {
						// Simple traversal
						return true
					},
				)
			}
		},
	)
}

// Benchmark for cache operations
func BenchmarkCacheHit(b *testing.B) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "cache_bench.go", testEmptyMainFunction, parser.ParseComments)
	if err != nil {
		b.Fatal(err)
	}

	// Populate cache
	_ = AnalyzeAll("cache_hit_bench.go", file, fset)

	b.ResetTimer()
	var hitFailed atomic.Bool
	b.RunParallel(
		func(pb *testing.PB) {
			for pb.Next() {
				// This should hit the cache
				if _, ok := checkCache("cache_hit_bench.go", file); !ok {
					hitFailed.Store(true)
				}
			}
		},
	)
	if hitFailed.Load() {
		b.Fatal("Expected cache hit")
	}
}

func BenchmarkCacheMiss(b *testing.B) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "cache_bench.go", testEmptyMainFunction, parser.ParseComments)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	var counter atomic.Uint64
	b.RunParallel(
		func(pb *testing.PB) {
			for pb.Next() {
				// This should miss the cache (different filename each time)
				i := counter.Add(1)
				filename := fmt.Sprintf("cache_miss_%d.go", i)
				checkCache(filename, file)
			}
		},
	)
}

// Benchmark for hash computation
func BenchmarkComputeFileHash(b *testing.B) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "bench.go", benchmarkCode, parser.ParseComments)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(
		func(pb *testing.PB) {
			for pb.Next() {
				_ = computeFileHash(file)
			}
		},
	)
}

// Parallel benchmarks
func BenchmarkAnalyzeParallel(b *testing.B) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "bench.go", benchmarkCode, parser.ParseComments)
	if err != nil {
		b.Fatal(err)
	}

	b.RunParallel(
		func(pb *testing.PB) {
			for pb.Next() {
				_ = AnalyzeAll("bench_parallel.go", file, fset)
			}
		},
	)
}

// Benchmark different code sizes
func BenchmarkAnalyzeSmallFile(b *testing.B) {
	benchmarkWithCode(b, testMainFunction)
}

func BenchmarkAnalyzeMediumFile(b *testing.B) {
	code := generateCode(100) // 100 functions
	benchmarkWithCode(b, code)
}

func BenchmarkAnalyzeLargeFile(b *testing.B) {
	code := generateCode(1000) // 1000 functions
	benchmarkWithCode(b, code)
}

// Helper to benchmark with specific code
func benchmarkWithCode(b *testing.B, code string) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "bench.go", code, parser.ParseComments)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(
		func(pb *testing.PB) {
			for pb.Next() {
				_ = AnalyzeAll("bench.go", file, fset)
			}
		},
	)
}

// Generate code with N functions
func generateCode(n int) string {
	var code strings.Builder
	code.WriteString("package main\n\n")

	for i := 0; i < n; i++ {
		code.WriteString(
			fmt.Sprintf(
				`
func Function%d() {
	for i := 0; i < 10; i++ {
		data := make([]int, 100)
		_ = data
	}
}
`, i,
			),
		)
	}

	return code.String()
}

// Memory allocation benchmarks
func BenchmarkAnalyzerMemoryAllocation(b *testing.B) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "bench.go", benchmarkCode, parser.ParseComments)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(
		func(pb *testing.PB) {
			for pb.Next() {
				issues := AnalyzeAll("bench.go", file, fset)
				// Ensure issues are used to prevent optimization
				_ = issues
			}
		},
	)
}

// Benchmark specific patterns
func BenchmarkNestedLoopDetection(b *testing.B) {
	code := `package main
func nested() {
	for i := 0; i < 100; i++ {
		for j := 0; j < 100; j++ {
			for k := 0; k < 100; k++ {
				_ = i + j + k
			}
		}
	}
}`
	benchmarkPattern(b, code, NewLoopAnalyzer())
}

func BenchmarkStringConcatenationDetection(b *testing.B) {
	code := `package main
func concat() {
	s := ""
	for i := 0; i < 1000; i++ {
		s += "test"
	}
}`
	benchmarkPattern(b, code, NewLoopAnalyzer())
}

func benchmarkPattern(b *testing.B, code string, analyzer Analyzer) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "pattern.go", code, parser.ParseComments)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = analyzer.Analyze(file, fset)
	}
}
