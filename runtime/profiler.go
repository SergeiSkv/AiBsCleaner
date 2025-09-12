// Package runtime provides runtime performance profiling capabilities
package runtime

import (
	"fmt"
	"io"
	gort "runtime"
	"runtime/pprof"
	"time"

	"github.com/SergeiSkv/AiBsCleaner/analyzer"
)

// Profiler provides runtime performance analysis
type Profiler struct {
	duration   time.Duration
	sampleRate int
}

// NewProfiler creates a new runtime profiler
func NewProfiler(duration time.Duration) *Profiler {
	return &Profiler{
		duration:   duration,
		sampleRate: 100, // Default sample rate
	}
}

// ProfileFunc profiles a function's runtime performance
func (p *Profiler) ProfileFunc(name string, fn func()) *RuntimeMetrics {
	metrics := &RuntimeMetrics{
		FunctionName: name,
		StartTime:    time.Now(),
	}

	// Capture initial memory stats
	var memStatsBefore gort.MemStats
	gort.ReadMemStats(&memStatsBefore)

	// Capture initial GC stats
	gcStatsBefore := memStatsBefore.NumGC

	// Run the function
	start := time.Now()
	fn()
	elapsed := time.Since(start)

	// Capture final memory stats
	var memStatsAfter gort.MemStats
	gort.ReadMemStats(&memStatsAfter)

	// Capture final GC stats
	gcStatsAfter := memStatsAfter.NumGC

	// Calculate metrics
	metrics.Duration = elapsed
	metrics.MemoryAllocated = memStatsAfter.Alloc - memStatsBefore.Alloc
	metrics.MemoryFreed = memStatsAfter.Frees - memStatsBefore.Frees
	metrics.GCRuns = uint32(gcStatsAfter - gcStatsBefore)
	metrics.HeapAlloc = memStatsAfter.HeapAlloc
	metrics.HeapObjects = memStatsAfter.HeapObjects
	metrics.Goroutines = gort.NumGoroutine()

	return metrics
}

// AnalyzeRuntime performs runtime analysis and returns issues
func (p *Profiler) AnalyzeRuntime() []analyzer.Issue {
	if p == nil {
		return nil
	}
	var issues []analyzer.Issue

	// Check memory usage
	var memStats gort.MemStats
	gort.ReadMemStats(&memStats)

	// High memory allocation rate
	if memStats.Mallocs > 1000000 {
		issues = append(
			issues, analyzer.Issue{
				Type:       "HIGH_ALLOCATION_RATE",
				Severity:   analyzer.SeverityHigh,
				Message:    fmt.Sprintf("High memory allocation rate: %d allocations", memStats.Mallocs),
				Suggestion: "Consider object pooling or reducing allocations",
			},
		)
	}

	// High heap usage
	heapUsagePercent := float64(memStats.HeapAlloc) / float64(memStats.HeapSys) * 100
	if heapUsagePercent > 80 {
		issues = append(
			issues, analyzer.Issue{
				Type:       "HIGH_HEAP_USAGE",
				Severity:   analyzer.SeverityHigh,
				Message:    fmt.Sprintf("Heap usage at %.1f%%", heapUsagePercent),
				Suggestion: "Memory usage is high, consider optimizing memory consumption",
			},
		)
	}

	// Too many goroutines
	numGoroutines := gort.NumGoroutine()
	if numGoroutines > 10000 {
		issues = append(
			issues, analyzer.Issue{
				Type:       "EXCESSIVE_GOROUTINES",
				Severity:   analyzer.SeverityHigh,
				Message:    fmt.Sprintf("Excessive goroutines: %d", numGoroutines),
				Suggestion: "Use worker pools to limit goroutine creation",
			},
		)
	}

	// GC pressure
	if memStats.NumGC > 100 && memStats.PauseTotalNs/uint64(memStats.NumGC) > 1000000 {
		avgPause := memStats.PauseTotalNs / uint64(memStats.NumGC) / 1000000 // Convert to ms
		issues = append(
			issues, analyzer.Issue{
				Type:       "HIGH_GC_PRESSURE",
				Severity:   analyzer.SeverityMedium,
				Message:    fmt.Sprintf("High GC pause time: %dms average", avgPause),
				Suggestion: "Reduce allocations and consider GOGC tuning",
			},
		)
	}

	return issues
}

// StartCPUProfile starts CPU profiling
func (p *Profiler) StartCPUProfile(w io.Writer) error {
	return pprof.StartCPUProfile(w)
}

// StopCPUProfile stops CPU profiling
func (p *Profiler) StopCPUProfile() {
	pprof.StopCPUProfile()
}

// RuntimeMetrics contains runtime performance metrics
type RuntimeMetrics struct {
	FunctionName    string
	StartTime       time.Time
	Duration        time.Duration
	MemoryAllocated uint64
	MemoryFreed     uint64
	GCRuns          uint32
	HeapAlloc       uint64
	HeapObjects     uint64
	Goroutines      int
}

// GetIssues converts runtime metrics to analyzer issues
func (m *RuntimeMetrics) GetIssues() []analyzer.Issue {
	if m == nil {
		return nil
	}
	var issues []analyzer.Issue

	// Check for slow execution
	if m.Duration > 100*time.Millisecond {
		issues = append(
			issues, analyzer.Issue{
				Type:       "SLOW_FUNCTION",
				Severity:   analyzer.SeverityMedium,
				Message:    fmt.Sprintf("Function %s took %v to execute", m.FunctionName, m.Duration),
				Suggestion: "Profile the function to identify bottlenecks",
			},
		)
	}

	// Check for high memory allocation
	if m.MemoryAllocated > 10*1024*1024 { // 10MB
		issues = append(
			issues, analyzer.Issue{
				Type:       "HIGH_MEMORY_ALLOCATION",
				Severity:   analyzer.SeverityMedium,
				Message:    fmt.Sprintf("Function %s allocated %d bytes", m.FunctionName, m.MemoryAllocated),
				Suggestion: "Consider pre-allocating memory or using object pools",
			},
		)
	}

	// Check for GC pressure during function execution
	if m.GCRuns > 0 {
		issues = append(
			issues, analyzer.Issue{
				Type:       "GC_DURING_EXECUTION",
				Severity:   analyzer.SeverityLow,
				Message:    fmt.Sprintf("Function %s triggered %d GC runs", m.FunctionName, m.GCRuns),
				Suggestion: "Reduce allocations to minimize GC overhead",
			},
		)
	}

	return issues
}

// MonitorRuntime continuously monitors runtime metrics
func MonitorRuntime(interval time.Duration, callback func([]analyzer.Issue)) {
	if callback == nil {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	profiler := NewProfiler(interval)
	if profiler == nil {
		return
	}

	for range ticker.C {
		issues := profiler.AnalyzeRuntime()
		if len(issues) > 0 {
			callback(issues)
		}
	}
}
