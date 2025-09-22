//go:build ignore
// +build ignore

package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBenchmarkComparator(t *testing.T) {
	bc := NewBenchmarkComparator()
	assert.NotNil(t, bc)
}

func TestParseBenchmarkOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected []BenchmarkResult
		wantErr  bool
	}{
		{
			name: "valid benchmark output",
			output: `goos: darwin
goarch: amd64
pkg: github.com/test/pkg
BenchmarkStringConcat-8    	1000000	      1050 ns/op	     512 B/op	      10 allocs/op
BenchmarkStringBuilder-8   	5000000	       300 ns/op	     128 B/op	       2 allocs/op
PASS
ok  	github.com/test/pkg	3.456s`,
			expected: []BenchmarkResult{
				{
					Name:        "BenchmarkStringConcat",
					Iterations:  1000000,
					NsPerOp:     1050,
					BytesPerOp:  512,
					AllocsPerOp: 10,
				},
				{
					Name:        "BenchmarkStringBuilder",
					Iterations:  5000000,
					NsPerOp:     300,
					BytesPerOp:  128,
					AllocsPerOp: 2,
				},
			},
			wantErr: false,
		},
		{
			name: "benchmark with sub-benchmarks",
			output: `BenchmarkMap/small-8         	10000000	       105 ns/op	      16 B/op	       1 allocs/op
BenchmarkMap/medium-8        	 5000000	       350 ns/op	      64 B/op	       2 allocs/op
BenchmarkMap/large-8         	 1000000	      1200 ns/op	     256 B/op	       8 allocs/op`,
			expected: []BenchmarkResult{
				{
					Name:        "BenchmarkMap/small",
					Iterations:  10000000,
					NsPerOp:     105,
					BytesPerOp:  16,
					AllocsPerOp: 1,
				},
				{
					Name:        "BenchmarkMap/medium",
					Iterations:  5000000,
					NsPerOp:     350,
					BytesPerOp:  64,
					AllocsPerOp: 2,
				},
				{
					Name:        "BenchmarkMap/large",
					Iterations:  1000000,
					NsPerOp:     1200,
					BytesPerOp:  256,
					AllocsPerOp: 8,
				},
			},
			wantErr: false,
		},
		{
			name:     "empty output",
			output:   "",
			expected: []BenchmarkResult{},
			wantErr:  false,
		},
		{
			name:     "no benchmark results",
			output:   "PASS\nok  	github.com/test/pkg	0.001s",
			expected: []BenchmarkResult{},
			wantErr:  false,
		},
		{
			name: "benchmark with only ns/op",
			output: `BenchmarkSimple-8    	1000000	      1050 ns/op
PASS`,
			expected: []BenchmarkResult{
				{
					Name:        "BenchmarkSimple",
					Iterations:  1000000,
					NsPerOp:     1050,
					BytesPerOp:  0,
					AllocsPerOp: 0,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := NewBenchmarkComparator()
			results, err := bc.parseBenchmarkOutput(tt.output)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.expected), len(results))

				for i, expected := range tt.expected {
					assert.Equal(t, expected.Name, results[i].Name)
					assert.Equal(t, expected.Iterations, results[i].Iterations)
					assert.Equal(t, expected.NsPerOp, results[i].NsPerOp)
					assert.Equal(t, expected.BytesPerOp, results[i].BytesPerOp)
					assert.Equal(t, expected.AllocsPerOp, results[i].AllocsPerOp)
				}
			}
		})
	}
}

func TestCompareBenchmarks(t *testing.T) {
	tests := []struct {
		name     string
		before   []BenchmarkResult
		after    []BenchmarkResult
		expected ComparisonReport
	}{
		{
			name: "performance improvement",
			before: []BenchmarkResult{
				{
					Name:        "BenchmarkStringConcat",
					Iterations:  1000000,
					NsPerOp:     1000,
					BytesPerOp:  512,
					AllocsPerOp: 10,
				},
			},
			after: []BenchmarkResult{
				{
					Name:        "BenchmarkStringConcat",
					Iterations:  2000000,
					NsPerOp:     500,
					BytesPerOp:  256,
					AllocsPerOp: 5,
				},
			},
			expected: ComparisonReport{
				Improvements: []BenchmarkComparison{
					{
						Name:              "BenchmarkStringConcat",
						BeforeNsPerOp:     1000,
						AfterNsPerOp:      500,
						SpeedupPercentage: 50.0,
						BeforeAllocsPerOp: 10,
						AfterAllocsPerOp:  5,
						AllocReduction:    50.0,
						BeforeBytesPerOp:  512,
						AfterBytesPerOp:   256,
						MemoryReduction:   50.0,
					},
				},
				Regressions: []BenchmarkComparison{},
			},
		},
		{
			name: "performance regression",
			before: []BenchmarkResult{
				{
					Name:        "BenchmarkMap",
					Iterations:  1000000,
					NsPerOp:     100,
					BytesPerOp:  64,
					AllocsPerOp: 2,
				},
			},
			after: []BenchmarkResult{
				{
					Name:        "BenchmarkMap",
					Iterations:  500000,
					NsPerOp:     200,
					BytesPerOp:  128,
					AllocsPerOp: 4,
				},
			},
			expected: ComparisonReport{
				Improvements: []BenchmarkComparison{},
				Regressions: []BenchmarkComparison{
					{
						Name:              "BenchmarkMap",
						BeforeNsPerOp:     100,
						AfterNsPerOp:      200,
						SpeedupPercentage: -100.0,
						BeforeAllocsPerOp: 2,
						AfterAllocsPerOp:  4,
						AllocReduction:    -100.0,
						BeforeBytesPerOp:  64,
						AfterBytesPerOp:   128,
						MemoryReduction:   -100.0,
					},
				},
			},
		},
		{
			name: "mixed results",
			before: []BenchmarkResult{
				{
					Name:        "BenchmarkA",
					Iterations:  1000000,
					NsPerOp:     1000,
					BytesPerOp:  512,
					AllocsPerOp: 10,
				},
				{
					Name:        "BenchmarkB",
					Iterations:  1000000,
					NsPerOp:     100,
					BytesPerOp:  64,
					AllocsPerOp: 2,
				},
			},
			after: []BenchmarkResult{
				{
					Name:        "BenchmarkA",
					Iterations:  2000000,
					NsPerOp:     500,
					BytesPerOp:  256,
					AllocsPerOp: 5,
				},
				{
					Name:        "BenchmarkB",
					Iterations:  500000,
					NsPerOp:     200,
					BytesPerOp:  128,
					AllocsPerOp: 4,
				},
			},
			expected: ComparisonReport{
				Improvements: []BenchmarkComparison{
					{
						Name:              "BenchmarkA",
						BeforeNsPerOp:     1000,
						AfterNsPerOp:      500,
						SpeedupPercentage: 50.0,
						BeforeAllocsPerOp: 10,
						AfterAllocsPerOp:  5,
						AllocReduction:    50.0,
						BeforeBytesPerOp:  512,
						AfterBytesPerOp:   256,
						MemoryReduction:   50.0,
					},
				},
				Regressions: []BenchmarkComparison{
					{
						Name:              "BenchmarkB",
						BeforeNsPerOp:     100,
						AfterNsPerOp:      200,
						SpeedupPercentage: -100.0,
						BeforeAllocsPerOp: 2,
						AfterAllocsPerOp:  4,
						AllocReduction:    -100.0,
						BeforeBytesPerOp:  64,
						AfterBytesPerOp:   128,
						MemoryReduction:   -100.0,
					},
				},
			},
		},
		{
			name: "new benchmark added",
			before: []BenchmarkResult{
				{
					Name:        "BenchmarkExisting",
					Iterations:  1000000,
					NsPerOp:     100,
					BytesPerOp:  64,
					AllocsPerOp: 2,
				},
			},
			after: []BenchmarkResult{
				{
					Name:        "BenchmarkExisting",
					Iterations:  1000000,
					NsPerOp:     100,
					BytesPerOp:  64,
					AllocsPerOp: 2,
				},
				{
					Name:        "BenchmarkNew",
					Iterations:  2000000,
					NsPerOp:     50,
					BytesPerOp:  32,
					AllocsPerOp: 1,
				},
			},
			expected: ComparisonReport{
				Improvements: []BenchmarkComparison{},
				Regressions:  []BenchmarkComparison{},
			},
		},
		{
			name: "benchmark removed",
			before: []BenchmarkResult{
				{
					Name:        "BenchmarkOld",
					Iterations:  1000000,
					NsPerOp:     100,
					BytesPerOp:  64,
					AllocsPerOp: 2,
				},
				{
					Name:        "BenchmarkKeep",
					Iterations:  1000000,
					NsPerOp:     200,
					BytesPerOp:  128,
					AllocsPerOp: 4,
				},
			},
			after: []BenchmarkResult{
				{
					Name:        "BenchmarkKeep",
					Iterations:  1000000,
					NsPerOp:     200,
					BytesPerOp:  128,
					AllocsPerOp: 4,
				},
			},
			expected: ComparisonReport{
				Improvements: []BenchmarkComparison{},
				Regressions:  []BenchmarkComparison{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := NewBenchmarkComparator()
			report := bc.compareBenchmarks(tt.before, tt.after)

			assert.Equal(t, len(tt.expected.Improvements), len(report.Improvements))
			assert.Equal(t, len(tt.expected.Regressions), len(report.Regressions))

			for i, expected := range tt.expected.Improvements {
				actual := report.Improvements[i]
				assert.Equal(t, expected.Name, actual.Name)
				assert.Equal(t, expected.SpeedupPercentage, actual.SpeedupPercentage)
				assert.Equal(t, expected.AllocReduction, actual.AllocReduction)
				assert.Equal(t, expected.MemoryReduction, actual.MemoryReduction)
			}

			for i, expected := range tt.expected.Regressions {
				actual := report.Regressions[i]
				assert.Equal(t, expected.Name, actual.Name)
				assert.Equal(t, expected.SpeedupPercentage, actual.SpeedupPercentage)
				assert.Equal(t, expected.AllocReduction, actual.AllocReduction)
				assert.Equal(t, expected.MemoryReduction, actual.MemoryReduction)
			}
		})
	}
}

func TestCompareBeforeAfter(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "benchmark-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a test Go file with benchmarks
	testFile := filepath.Join(tmpDir, "bench_test.go")
	testContent := `package main

import (
	"strings"
	"testing"
)

func BenchmarkStringConcat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		s := ""
		for j := 0; j < 10; j++ {
			s += "test"
		}
	}
}

func BenchmarkStringBuilder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var sb strings.Builder
		for j := 0; j < 10; j++ {
			sb.WriteString("test")
		}
		_ = sb.String()
	}
}`

	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Create go.mod file
	goModFile := filepath.Join(tmpDir, "go.mod")
	goModContent := `module testbench

go 1.21`
	err = os.WriteFile(goModFile, []byte(goModContent), 0644)
	require.NoError(t, err)

	t.Run("successful comparison", func(t *testing.T) {
		bc := NewBenchmarkComparator()

		// Mock the changes function - in real scenario this would modify code
		changes := func() error {
			// Simulate code optimization by modifying the file
			optimizedContent := strings.Replace(testContent, `s += "test"`, `s = s + "test"`, 1)
			return os.WriteFile(testFile, []byte(optimizedContent), 0644)
		}

		report, err := bc.CompareBeforeAfter(tmpDir, changes)

		// We can't guarantee specific benchmark results in tests,
		// so just verify the function runs without error
		assert.NoError(t, err)
		assert.NotNil(t, report)
	})

	t.Run("changes function error", func(t *testing.T) {
		bc := NewBenchmarkComparator()

		changes := func() error {
			return fmt.Errorf("simulated error")
		}

		report, err := bc.CompareBeforeAfter(tmpDir, changes)

		assert.Error(t, err)
		assert.Nil(t, report)
		assert.Contains(t, err.Error(), "failed to apply changes")
	})

	t.Run("invalid repo path", func(t *testing.T) {
		bc := NewBenchmarkComparator()

		changes := func() error {
			return nil
		}

		report, err := bc.CompareBeforeAfter("/nonexistent/path", changes)

		assert.Error(t, err)
		assert.Nil(t, report)
	})
}

func TestFormatReport(t *testing.T) {
	report := &ComparisonReport{
		Improvements: []BenchmarkComparison{
			{
				Name:              "BenchmarkStringConcat",
				BeforeNsPerOp:     1000,
				AfterNsPerOp:      500,
				SpeedupPercentage: 50.0,
				BeforeAllocsPerOp: 10,
				AfterAllocsPerOp:  5,
				AllocReduction:    50.0,
				BeforeBytesPerOp:  512,
				AfterBytesPerOp:   256,
				MemoryReduction:   50.0,
			},
		},
		Regressions: []BenchmarkComparison{
			{
				Name:              "BenchmarkMap",
				BeforeNsPerOp:     100,
				AfterNsPerOp:      200,
				SpeedupPercentage: -100.0,
				BeforeAllocsPerOp: 2,
				AfterAllocsPerOp:  4,
				AllocReduction:    -100.0,
				BeforeBytesPerOp:  64,
				AfterBytesPerOp:   128,
				MemoryReduction:   -100.0,
			},
		},
		Summary: "1 improvements, 1 regressions",
	}

	bc := NewBenchmarkComparator()
	output := bc.FormatReport(report)

	assert.Contains(t, output, "Benchmark Comparison Report")
	assert.Contains(t, output, "Improvements")
	assert.Contains(t, output, "BenchmarkStringConcat")
	assert.Contains(t, output, "50.00% faster")
	assert.Contains(t, output, "50.00% fewer allocations")
	assert.Contains(t, output, "50.00% less memory")
	assert.Contains(t, output, "Regressions")
	assert.Contains(t, output, "BenchmarkMap")
	assert.Contains(t, output, "100.00% slower")
	assert.Contains(t, output, "100.00% more allocations")
	assert.Contains(t, output, "100.00% more memory")
}

func TestGenerateSummary(t *testing.T) {
	tests := []struct {
		name     string
		report   ComparisonReport
		expected string
	}{
		{
			name: "improvements only",
			report: ComparisonReport{
				Improvements: []BenchmarkComparison{
					{Name: "Bench1"},
					{Name: "Bench2"},
				},
				Regressions: []BenchmarkComparison{},
			},
			expected: "✅ 2 improvements, 0 regressions",
		},
		{
			name: "regressions only",
			report: ComparisonReport{
				Improvements: []BenchmarkComparison{},
				Regressions: []BenchmarkComparison{
					{Name: "Bench1"},
				},
			},
			expected: "⚠️ 0 improvements, 1 regressions",
		},
		{
			name: "mixed results",
			report: ComparisonReport{
				Improvements: []BenchmarkComparison{
					{Name: "Bench1"},
				},
				Regressions: []BenchmarkComparison{
					{Name: "Bench2"},
					{Name: "Bench3"},
				},
			},
			expected: "⚠️ 1 improvements, 2 regressions",
		},
		{
			name: "no changes",
			report: ComparisonReport{
				Improvements: []BenchmarkComparison{},
				Regressions:  []BenchmarkComparison{},
			},
			expected: "✅ 0 improvements, 0 regressions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := NewBenchmarkComparator()
			summary := bc.generateSummary(tt.report)
			assert.Equal(t, tt.expected, summary)
		})
	}
}
