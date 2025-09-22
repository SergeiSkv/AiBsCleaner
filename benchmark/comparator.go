//go:build ignor
// +build ignore


package benchma
package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// BenchmarkResult represents a single benchmark result
type BenchmarkResult struct {
	Name          string             `json:"name"`
	Runs          int                `json:"runs"`
	NsPerOp       float64            `json:"ns_per_op"`
	AllocsPerOp   int64              `json:"allocs_per_op"`
	BytesPerOp    int64              `json:"bytes_per_op"`
	MBPerSec      float64            `json:"mb_per_sec,omitempty"`
	CustomMetrics map[string]float64 `json:"custom_metrics,omitempty"`
}

// Comparison represents before/after benchmark comparison
type Comparison struct {
	Name           string          `json:"name"`
	Before         BenchmarkResult `json:"before"`
	After          BenchmarkResult `json:"after"`
	SpeedupPercent float64         `json:"speedup_percent"`
	AllocChange    float64         `json:"alloc_change_percent"`
	BytesChange    float64         `json:"bytes_change_percent"`
	Improved       bool            `json:"improved"`
}

// ComparisonReport represents full benchmark comparison report
type ComparisonReport struct {
	Timestamp       time.Time    `json:"timestamp"`
	RepoURL         string       `json:"repo_url"`
	Branch          string       `json:"branch"`
	CommitBefore    string       `json:"commit_before"`
	CommitAfter     string       `json:"commit_after"`
	Comparisons     []Comparison `json:"comparisons"`
	OverallImproved bool         `json:"overall_improved"`
	Summary         Summary      `json:"summary"`
}

// Summary provides overall statistics
type Summary struct {
	TotalBenchmarks    int     `json:"total_benchmarks"`
	ImprovedCount      int     `json:"improved_count"`
	RegressedCount     int     `json:"regressed_count"`
	UnchangedCount     int     `json:"unchanged_count"`
	AvgSpeedup         float64 `json:"avg_speedup_percent"`
	AvgMemoryReduction float64 `json:"avg_memory_reduction_percent"`
}

// BenchmarkComparator handles benchmark comparisons
type BenchmarkComparator struct {
	workDir string
}

// NewBenchmarkComparator creates a new benchmark comparator
func NewBenchmarkComparator(workDir string) *BenchmarkComparator {
	return &BenchmarkComparator{
		workDir: workDir,
	}
}

// CompareBeforeAfter runs benchmarks before and after changes
func (bc *BenchmarkComparator) CompareBeforeAfter(repoPath string, changes func() error) (*ComparisonReport, error) {
	// 1. Run benchmarks before changes
	beforeResults, commitBefore, err := bc.runBenchmarks(repoPath, "before")
	if err != nil {
		return nil, fmt.Errorf("failed to run before benchmarks: %w", err)
	}

	// 2. Apply changes
	if err := changes(); err != nil {
		return nil, fmt.Errorf("failed to apply changes: %w", err)
	}

	// 3. Run benchmarks after changes
	afterResults, commitAfter, err := bc.runBenchmarks(repoPath, "after")
	if err != nil {
		return nil, fmt.Errorf("failed to run after benchmarks: %w", err)
	}

	// 4. Compare results
	report := bc.generateReport(beforeResults, afterResults, commitBefore, commitAfter)
	report.RepoURL = repoPath

	return report, nil
}

// runBenchmarks runs all benchmarks in the repository
func (bc *BenchmarkComparator) runBenchmarks(repoPath, tag string) (map[string]*BenchmarkResult, string, error) {
	// Get current commit
	commit, err := bc.getCurrentCommit(repoPath)
	if err != nil {
		return nil, "", err
	}

	// Create output file
	outputFile := filepath.Join(bc.workDir, fmt.Sprintf("bench_%s_%s.txt", tag, time.Now().Format("20060102_150405")))

	// Run benchmarks with memory profiling
	cmd := exec.Command(
		"go", "test",
		"-bench=.",
		"-benchmem",
		"-benchtime=10s",
		"-count=3",
		"-cpu=1,2,4",
		"-timeout=30m",
		"./...",
	)
	cmd.Dir = repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("benchmark failed: %v\nstderr: %s", err, stderr.String())
	}

	// Save raw output
	if err := os.WriteFile(outputFile, stdout.Bytes(), 0644); err != nil {
		return nil, "", err
	}

	// Parse results
	results := bc.parseBenchmarkOutput(stdout.String())

	return results, commit, nil
}

// parseBenchmarkOutput parses go test benchmark output
func (bc *BenchmarkComparator) parseBenchmarkOutput(output string) map[string]*BenchmarkResult {
	results := make(map[string]*BenchmarkResult)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		name := fields[0]
		result := &BenchmarkResult{
			Name: name,
		}

		// Parse iterations
		if len(fields) > 1 {
			fmt.Sscanf(fields[1], "%d", &result.Runs)
		}

		// Parse ns/op
		for _, field := range fields {
			if strings.HasSuffix(field, "ns/op") {
				fmt.Sscanf(field, "%f", &result.NsPerOp)
			} else if strings.HasSuffix(field, "B/op") {
				fmt.Sscanf(field, "%d", &result.BytesPerOp)
			} else if strings.HasSuffix(field, "allocs/op") {
				fmt.Sscanf(field, "%d", &result.AllocsPerOp)
			} else if strings.HasSuffix(field, "MB/s") {
				fmt.Sscanf(field, "%f", &result.MBPerSec)
			}
		}

		results[name] = result
	}

	return results
}

// generateReport creates comparison report
func (bc *BenchmarkComparator) generateReport(
	before, after map[string]*BenchmarkResult, commitBefore, commitAfter string,
) *ComparisonReport {
	report := &ComparisonReport{
		Timestamp:    time.Now(),
		CommitBefore: commitBefore,
		CommitAfter:  commitAfter,
		Comparisons:  []Comparison{},
	}

	summary := Summary{}

	// Compare each benchmark
	for name, beforeResult := range before {
		afterResult, exists := after[name]
		if !exists {
			continue
		}

		comparison := bc.compareBenchmarks(beforeResult, afterResult)
		report.Comparisons = append(report.Comparisons, comparison)

		// Update summary
		summary.TotalBenchmarks++
		if comparison.Improved {
			summary.ImprovedCount++
		} else if comparison.SpeedupPercent < -5 { // 5% regression threshold
			summary.RegressedCount++
		} else {
			summary.UnchangedCount++
		}

		summary.AvgSpeedup += comparison.SpeedupPercent
		summary.AvgMemoryReduction += comparison.BytesChange
	}

	// Calculate averages
	if summary.TotalBenchmarks > 0 {
		summary.AvgSpeedup /= float64(summary.TotalBenchmarks)
		summary.AvgMemoryReduction /= float64(summary.TotalBenchmarks)
	}

	report.Summary = summary
	report.OverallImproved = summary.ImprovedCount > summary.RegressedCount

	return report
}

// compareBenchmarks compares two benchmark results
func (bc *BenchmarkComparator) compareBenchmarks(before, after *BenchmarkResult) Comparison {
	speedup := ((before.NsPerOp - after.NsPerOp) / before.NsPerOp) * 100
	allocChange := float64(after.AllocsPerOp-before.AllocsPerOp) / float64(before.AllocsPerOp) * 100
	bytesChange := float64(after.BytesPerOp-before.BytesPerOp) / float64(before.BytesPerOp) * 100

	return Comparison{
		Name:           before.Name,
		Before:         *before,
		After:          *after,
		SpeedupPercent: speedup,
		AllocChange:    allocChange,
		BytesChange:    bytesChange,
		Improved:       speedup > 5 || bytesChange < -5, // 5% improvement threshold
	}
}

// getCurrentCommit gets current git commit hash
func (bc *BenchmarkComparator) getCurrentCommit(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// GenerateMarkdownReport generates a markdown report
func (bc *BenchmarkComparator) GenerateMarkdownReport(report *ComparisonReport) string {
	var sb strings.Builder

	sb.WriteString("# Benchmark Comparison Report\n\n")
	sb.WriteString(fmt.Sprintf("**Date:** %s\n", report.Timestamp.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**Before:** `%s`\n", report.CommitBefore[:8]))
	sb.WriteString(fmt.Sprintf("**After:** `%s`\n\n", report.CommitAfter[:8]))

	// Summary
	sb.WriteString("## Summary\n\n")
	if report.OverallImproved {
		sb.WriteString("✅ **Overall Performance Improved**\n\n")
	} else {
		sb.WriteString("⚠️ **Performance Regression Detected**\n\n")
	}

	sb.WriteString(fmt.Sprintf("- Total Benchmarks: %d\n", report.Summary.TotalBenchmarks))
	sb.WriteString(fmt.Sprintf("- Improved: %d\n", report.Summary.ImprovedCount))
	sb.WriteString(fmt.Sprintf("- Regressed: %d\n", report.Summary.RegressedCount))
	sb.WriteString(fmt.Sprintf("- Unchanged: %d\n", report.Summary.UnchangedCount))
	sb.WriteString(fmt.Sprintf("- Average Speedup: %.2f%%\n", report.Summary.AvgSpeedup))
	sb.WriteString(fmt.Sprintf("- Average Memory Change: %.2f%%\n\n", report.Summary.AvgMemoryReduction))

	// Detailed results
	sb.WriteString("## Detailed Results\n\n")
	sb.WriteString("| Benchmark | Before (ns/op) | After (ns/op) | Speedup | Memory Change |\n")
	sb.WriteString("|-----------|---------------|--------------|---------|---------------|\n")

	for _, comp := range report.Comparisons {
		emoji := "➖"
		if comp.Improved {
			emoji = "✅"
		} else if comp.SpeedupPercent < -5 {
			emoji = "❌"
		}

		sb.WriteString(
			fmt.Sprintf(
				"| %s %s | %.2f | %.2f | %+.2f%% | %+.2f%% |\n",
				emoji,
				comp.Name,
				comp.Before.NsPerOp,
				comp.After.NsPerOp,
				comp.SpeedupPercent,
				comp.BytesChange,
			),
		)
	}

	// Memory details
	if len(report.Comparisons) > 0 {
		sb.WriteString("\n## Memory Analysis\n\n")
		sb.WriteString("| Benchmark | Allocs Before | Allocs After | Bytes Before | Bytes After |\n")
		sb.WriteString("|-----------|--------------|--------------|--------------|-------------|\n")

		for _, comp := range report.Comparisons {
			sb.WriteString(
				fmt.Sprintf(
					"| %s | %d | %d | %d B | %d B |\n",
					comp.Name,
					comp.Before.AllocsPerOp,
					comp.After.AllocsPerOp,
					comp.Before.BytesPerOp,
					comp.After.BytesPerOp,
				),
			)
		}
	}

	return sb.String()
}

// SaveReport saves the report to file
func (bc *BenchmarkComparator) SaveReport(report *ComparisonReport, format string) (string, error) {
	filename := filepath.Join(
		bc.workDir, fmt.Sprintf(
			"benchmark_report_%s.%s",
			time.Now().Format("20060102_150405"), format,
		),
	)

	var data []byte
	var err error

	switch format {
	case "json":
		data, err = json.MarshalIndent(report, "", "  ")
	case "md", "markdown":
		data = []byte(bc.GenerateMarkdownReport(report))
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return "", err
	}

	return filename, nil
}
