// Package main implements golangci-lint plugin for PerfChecker
package main

import (
	"encoding/json"
	"fmt"
	"go/token"
	"strings"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/SergeiSkv/AiBsCleaner/analyzer"
)

func main() {
	register.Plugin("aibscleaner", New)
}

// New creates a new PerfChecker analyzer for golangci-lint
func New(settings interface{}) (register.LinterPlugin, error) {
	cfg := &Config{}
	if settings != nil {
		data, err := json.Marshal(settings)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	return &PerfCheckerPlugin{
		config: cfg,
	}, nil
}

// Config represents the plugin configuration
type Config struct {
	Complexity struct {
		MaxCyclomatic  int `json:"max-cyclomatic"`
		MaxCognitive   int `json:"max-cognitive"`
		MaxNestedLoops int `json:"max-nested-loops"`
	} `json:"complexity"`

	Memory struct {
		CheckLeaks           bool `json:"check-leaks"`
		CheckAllocations     bool `json:"check-allocations"`
		MaxAllocationPerFunc int  `json:"max-allocation-per-func"`
	} `json:"memory"`

	Database struct {
		CheckSQLInjection         bool `json:"check-sql-injection"`
		CheckNPlusOne             bool `json:"check-n-plus-one"`
		RequirePreparedStatements bool `json:"require-prepared-statements"`
		RequireContext            bool `json:"require-context"`
		MaxQueriesPerFunction     int  `json:"max-queries-per-function"`
	} `json:"database"`

	Defer struct {
		CheckUnnecessary     bool `json:"check-unnecessary"`
		CheckInLoop          bool `json:"check-in-loop"`
		CheckHotPath         bool `json:"check-hot-path"`
		MaxDefersPerFunction int  `json:"max-defers-per-function"`
	} `json:"defer"`

	NilPtr struct {
		CheckDereference     bool `json:"check-dereference"`
		CheckUncheckedErrors bool `json:"check-unchecked-errors"`
		CheckTypeAssertions  bool `json:"check-type-assertions"`
		RequireNilChecks     bool `json:"require-nil-checks"`
	} `json:"nilptr"`

	Coverage struct {
		MinCoverage       float64 `json:"min-coverage"`
		RequireBenchmarks bool    `json:"require-benchmarks"`
		RequireExamples   bool    `json:"require-examples"`
	} `json:"coverage"`

	Exclude []string `json:"exclude"`
}

// PerfCheckerPlugin implements the golangci-lint plugin interface
type PerfCheckerPlugin struct {
	config *Config
}

// BuildAnalyzers returns all PerfChecker analyzers for golangci-lint
func (p *PerfCheckerPlugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	var analyzers []*analysis.Analyzer

	// Convert our analyzers to analysis.Analyzer format
	analyzers = append(
		analyzers,
		p.createAnalyzer("loop", analyzer.NewLoopAnalyzer()),
		p.createAnalyzer("defer-optimization", analyzer.NewDeferOptimizationAnalyzer()),
		p.createAnalyzer("slice", analyzer.NewSliceAnalyzer()),
		p.createAnalyzer("map", analyzer.NewMapAnalyzer()),
		p.createAnalyzer("reflection", analyzer.NewReflectionAnalyzer()),
		p.createAnalyzer("goroutine", analyzer.NewGoroutineAnalyzer()),
		p.createAnalyzer("interface", analyzer.NewInterfaceAnalyzer()),
		p.createAnalyzer("regex", analyzer.NewRegexAnalyzer()),
		p.createAnalyzer("time", analyzer.NewTimeAnalyzer()),
		p.createAnalyzer("memory-leak", analyzer.NewMemoryLeakAnalyzer()),
		p.createAnalyzer("test-coverage", analyzer.NewTestCoverageAnalyzer()),
		p.createAnalyzer("database", analyzer.NewDatabaseAnalyzer()),
		p.createAnalyzer("nilptr", analyzer.NewNilPtrAnalyzer()),
		p.createAnalyzer("api-misuse", analyzer.NewAPIMisuseAnalyzer()),
		p.createAnalyzer("ai-bullshit", analyzer.NewAIBullshitAnalyzer()),
		p.createAnalyzer("context", analyzer.NewContextAnalyzer()),
		p.createAnalyzer("channel", analyzer.NewChannelAnalyzer()),
		p.createAnalyzer("race-condition", analyzer.NewRaceConditionAnalyzer()),
		p.createAnalyzer("http-client", analyzer.NewHTTPClientAnalyzer()),
		p.createAnalyzer("gc-pressure", analyzer.NewGCPressureAnalyzer()),
		p.createAnalyzer("concurrency-patterns", analyzer.NewConcurrencyPatternsAnalyzer()),
		p.createAnalyzer("cpu-optimization", analyzer.NewCPUOptimizationAnalyzer()),
		p.createAnalyzer("network-patterns", analyzer.NewNetworkPatternsAnalyzer()),
		p.createAnalyzer("sync-pool", analyzer.NewSyncPoolAnalyzer()),
		p.createAnalyzer("privacy", analyzer.NewPrivacyAnalyzer()),
		p.createAnalyzer("dependency", analyzer.NewDependencyAnalyzer(".")),
		p.createAnalyzer("crypto", analyzer.NewCryptoAnalyzer()),
		p.createAnalyzer("serialization", analyzer.NewSerializationAnalyzer()),
		p.createAnalyzer("io-buffer", analyzer.NewIOBufferAnalyzer()),
		p.createAnalyzer("http-reuse", analyzer.NewHTTPReuseAnalyzer()),
		p.createAnalyzer("cgo", analyzer.NewCGOAnalyzer()),
	)
	return analyzers, nil
}

// GetLoadMode returns the load mode for the analyzers
func (p *PerfCheckerPlugin) GetLoadMode() string {
	return register.LoadModeSyntax
}

func (p *PerfCheckerPlugin) createAnalyzer(name string, perfAnalyzer analyzer.Analyzer) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "perfchecker-" + name,
		Doc:  fmt.Sprintf("PerfChecker %s analyzer: checks for %s performance issues", name, name),
		Run: func(pass *analysis.Pass) (interface{}, error) {
			for _, file := range pass.Files {
				// Skip excluded paths
				if p.shouldSkip(pass.Fset.Position(file.Pos()).Filename) {
					continue
				}

				// Run our analyzer
				issues := perfAnalyzer.Analyze(file, pass.Fset)

				// Convert issues to diagnostics
				for _, issue := range issues {
					if p.shouldReportIssue(issue) {
						diagnostic := analysis.Diagnostic{
							Pos:     token.Pos(issue.Position.Offset),
							Message: fmt.Sprintf("[%s] %s", issue.Type, issue.Message),
						}

						// Add suggestion if available
						if issue.Suggestion != "" {
							diagnostic.SuggestedFixes = []analysis.SuggestedFix{
								{
									Message: issue.Suggestion,
								},
							}
						}

						pass.Report(diagnostic)
					}
				}
			}
			return nil, nil
		},
	}
}

func (p *PerfCheckerPlugin) shouldSkip(filename string) bool {
	for _, pattern := range p.config.Exclude {
		if strings.Contains(filename, pattern) {
			return true
		}
	}
	return false
}

func (p *PerfCheckerPlugin) shouldReportIssue(issue *analyzer.Issue) bool {
	// Filter based on configuration
	switch issue.Type {
	case analyzer.IssueSQLNPlusOne:
		return p.config.Database.CheckNPlusOne
	case analyzer.IssueSQLInLoop:
		return p.config.Database.CheckSQLInjection
	case analyzer.IssueNoPreparedStmt:
		return p.config.Database.RequirePreparedStatements
	case analyzer.IssueContextBackground:
		return p.config.Database.RequireContext
	case analyzer.IssueUnnecessaryDefer:
		return p.config.Defer.CheckUnnecessary
	case analyzer.IssueDeferInLoop:
		return p.config.Defer.CheckInLoop
	case analyzer.IssueDeferInHotPath:
		return p.config.Defer.CheckHotPath
	case analyzer.IssuePotentialNilDeref, analyzer.IssueNilMethodCall:
		return p.config.NilPtr.CheckDereference
	case analyzer.IssueErrorIgnored, analyzer.IssueErrorCheckMissing:
		return p.config.NilPtr.CheckUncheckedErrors
	case analyzer.IssueNilCheck:
		return p.config.NilPtr.CheckTypeAssertions
	case analyzer.IssueMissingClose, analyzer.IssueTimeAfterLeak, analyzer.IssueGoroutineLeak:
		return p.config.Memory.CheckLeaks
	default:
		// Report all issues by default
		return true
	}
}

// GetAnalyzers is the alternative method for providing analyzers
func GetAnalyzers() []*analysis.Analyzer {
	plugin, err := New(nil)
	if err != nil {
		return nil
	}
	if plugin == nil {
		return nil
	}
	analyzers, err := plugin.BuildAnalyzers()
	if err != nil {
		return nil
	}
	return analyzers
}
