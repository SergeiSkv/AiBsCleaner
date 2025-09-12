package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/SergeiSkv/AiBsCleaner/analyzer"
	"github.com/SergeiSkv/AiBsCleaner/cache"
	"github.com/SergeiSkv/AiBsCleaner/version"
)

var (
	jsonOutput bool
	configPath string
	compact    bool
	verbose    bool
	reportType string
	logLevel   string
	noCache    = true // Disabled by default for better accuracy
	clearCache bool
	ignoreFile string
	logger     *slog.Logger
	cacheDB    *cache.FileCache
)

// JSONOutput represents the JSON structure for results
type JSONOutput struct {
	Target    string            `json:"target"`
	Summary   Summary           `json:"summary"`
	Issues    []*analyzer.Issue `json:"issues"`
	FileStats map[string]int    `json:"file_stats"`
}

// Summary contains overall statistics
type Summary struct {
	TotalIssues int `json:"total_issues"`
	High        int `json:"high"`
	Medium      int `json:"medium"`
	Low         int `json:"low"`
}

var rootCmd = &cobra.Command{
	Use:   "aibscleaner [path]",
	Short: "AiBsCleaner - Stop AI bullshit, write performant Go",
	Long: `AiBsCleaner is a high-performance static analyzer for Go code.
It detects performance issues, anti-patterns, and AI-generated bullshit code.

Perfect for CI/CD pipelines, code reviews, and keeping your codebase clean.`,
	Example: `  
  aibscleaner .                        # AnalyzeAll current directory
  aibscleaner ./src                    # AnalyzeAll specific directory
  aibscleaner main.go                  # AnalyzeAll single file
  aibscleaner --json .                 # JSON output for CI/CD
  aibscleaner --compact .              # Compact IDE-friendly output`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			args = append(args, ".")
		}

		target := args[0]

		// Check if target exists
		if _, err := os.Stat(target); os.IsNotExist(err) {
			slog.Error("Path does not exist", "path", target)
			os.Exit(1)
		}

		// Initialize file cache unless --no-cache is specified
		if !noCache {
			var err error
			cacheDB, err = cache.New(getProjectRoot(target))
			if err != nil {
				slog.Warn("Failed to open cache database", "error", err)
				// Continue without cache
			}
			defer func() {
				if cacheDB != nil {
					_ = cacheDB.Close()
				}
			}()

			// Clear cache if requested
			if clearCache && cacheDB != nil {
				if err := cacheDB.ClearCache(); err != nil {
					slog.Warn("Failed to clear cache", "error", err)
				} else {
					slog.Info("Cache cleared")
				}
			}
		}

		// Load configuration
		config, err := LoadConfig(configPath)
		if err != nil {
			slog.Error("Failed to load config", "error", err)
			os.Exit(1)
		}

		// Remove noisy logging during analysis

		// Set compact mode from flag
		if compact {
			_ = os.Setenv("AIBSCLEANER_COMPACT", "1")
		}

		issues := analyzeTarget(target, config)

		if jsonOutput {
			outputJSON(target, issues)
		} else {
			outputHuman(target, issues)
		}

		// Exit with error code if high severity issues found
		high, _, _ := countBySeverity(issues)
		if high > 0 {
			os.Exit(1)
		}
	},
}

var initConfigCmd = &cobra.Command{
	Use:   "init",
	Short: "Create default configuration file",
	Long:  `Creates a .aibscleaner.yaml configuration file with default settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		createDefaultConfig()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("AiBsCleaner version %s\n", version.Version)
		fmt.Printf("Commit: %s\n", version.CommitHash)
		fmt.Printf("Built: %s\n", version.BuiltAt)
		fmt.Println("\nStop AI bullshit, write performant Go!")
	},
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show cache statistics",
	Long:  `Shows statistics about the cached analysis results.`,
	Run: func(cmd *cobra.Command, args []string) {
		target := "."
		if len(args) > 0 {
			target = args[0]
		}

		cacheDB, err := cache.New(getProjectRoot(target))
		if err != nil {
			slog.Error("Failed to open cache database", "error", err)
			os.Exit(1)
		}
		defer func() { _ = cacheDB.Close() }()

		stats := cacheDB.GetStats()

		fmt.Println("Cache Statistics:")
		fmt.Println("====================")
		fmt.Printf("Total files analyzed:  %d\n", stats["total_files"])
		fmt.Printf("Total issues found:    %d\n", stats["total_issues"])
		fmt.Printf("Ignored issues:        %d\n", stats["ignored_issues"])
		fmt.Printf("Fixed issues:          %d\n", stats["fixed_issues"])
		fmt.Printf("\n")
		fmt.Printf("Cache location: %s\n", cacheDB.GetCacheDir())

		if fileInfo, err := os.Stat(filepath.Join(cacheDB.GetCacheDir(), cache.CacheFile)); err == nil {
			fmt.Printf("Cache size:     %.2f MB\n", float64(fileInfo.Size())/(1024*1024))
		}
	},
}

var listCmd = &cobra.Command{
	Use:   "list-analyzers",
	Short: "List all available analyzers",
	Long:  `Shows all available analyzers and their detection patterns.`,
	Run: func(cmd *cobra.Command, args []string) {
		analyzers := []struct {
			Name        string
			Description string
		}{
			{"LoopAnalyzer", "Detects inefficient loops and allocations"},
			{"StringConcatAnalyzer", "Finds inefficient string concatenations"},
			{"DeferAnalyzer", "Identifies defer misuse and overhead"},
			{"SliceAnalyzer", "Detects slice capacity and append issues"},
			{"MapAnalyzer", "Finds map initialization problems"},
			{"ReflectionAnalyzer", "Warns about reflection performance impact"},
			{"GoroutineAnalyzer", "Detects goroutine leaks and misuse"},
			{"InterfaceAnalyzer", "Finds unnecessary interface allocations"},
			{"RegexAnalyzer", "Identifies regex compilation in hot paths"},
			{"TimeAnalyzer", "Detects time.After leaks and inefficiencies"},
			{"ComplexityAnalyzer", "Measures cyclomatic complexity"},
			{"MemoryLeakAnalyzer", "Finds potential memory leaks"},
			{"DatabaseAnalyzer", "Detects database performance issues"},
			{"AIBullshitDetector", "Identifies AI-generated anti-patterns"},
			{"ContextAnalyzer", "Finds context misuse and leaks"},
			{"ChannelAnalyzer", "Detects channel deadlocks and inefficiencies"},
			{"RaceConditionAnalyzer", "Identifies potential race conditions"},
			{"ErrorHandlingAnalyzer", "Finds error handling issues"},
			{"HTTPClientAnalyzer", "Detects HTTP client problems"},
			{"GCPressureAnalyzer", "Identifies high GC pressure patterns"},
			{"ConcurrencyPatternsAnalyzer", "Finds concurrency anti-patterns"},
			{"CPUOptimizationAnalyzer", "Detects CPU-intensive operations"},
			{"NetworkPatternsAnalyzer", "Finds network performance issues"},
			{"SyncPoolAnalyzer", "Suggests sync.Pool optimizations"},
			{"PrivacyAnalyzer", "Detects privacy issues and data leaks"},
			{"DependencyAnalyzer", "Checks dependency health and vulnerabilities"},
		}

		fmt.Println("Available Analyzers:")
		fmt.Println("====================")
		for _, a := range analyzers {
			fmt.Printf("• %-30s %s\n", a.Name, a.Description)
		}
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&jsonOutput, "json", "j", false, "Output results in JSON format")
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Path to configuration file")
	rootCmd.PersistentFlags().BoolVarP(&compact, "compact", "", false, "Compact IDE-friendly output")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().StringVarP(&reportType, "report", "r", "terminal", "Report format: terminal, html, markdown, json, all")
	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "", "info", "Log level: debug, info, warn, error")
	rootCmd.PersistentFlags().BoolVar(&noCache, "no-cache", true, "Disable cache and re-analyze all files (default: true)")
	rootCmd.PersistentFlags().BoolVar(&clearCache, "clear-cache", false, "Clear the cache before analyzing")

	// Add flag to enable cache
	enableCache := rootCmd.PersistentFlags().Bool("enable-cache", false, "Enable file cache for faster subsequent runs")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if *enableCache {
			noCache = false
		}
	}
	rootCmd.PersistentFlags().StringVar(&ignoreFile, "ignore-file", ".abcignore", "Path to ignore file")

	rootCmd.AddCommand(initConfigCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(statsCmd)

	// Setup logger
	cobra.OnInitialize(initLogger)
}

func initLogger() {
	// Parse log level
	var level slog.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Configure handler based on output format
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: verbose,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Remove time, level, source - only show message and custom attrs
			if a.Key == slog.TimeKey || a.Key == slog.LevelKey || a.Key == slog.SourceKey {
				return slog.Attr{}
			}
			return a
		},
	}

	if jsonOutput {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	logger = slog.New(handler)
	slog.SetDefault(logger)
}

func Execute() error {
	return rootCmd.Execute()
}

func analyzeTarget(target string, config *Config) []*analyzer.Issue {
	if config == nil {
		return nil
	}
	var allIssues []*analyzer.Issue
	var filesAnalyzed int
	var totalLines int

	// Run dependency analysis ONCE for the entire project
	depIssues := analyzer.AnalyzeDependencies(target)
	allIssues = append(allIssues, depIssues...)

	err := filepath.Walk(
		target, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Check if path should be skipped
			skip, skipDir := shouldSkipPath(path, info, config.Paths.Exclude)
			if skipDir {
				return filepath.SkipDir
			}
			if skip {
				return nil
			}

			// Process only Go files
			if !isGoFile(path, info) {
				return nil
			}

			// Count the file
			filesAnalyzed++
			totalLines += countLines(path)

			issues := analyzeFile(path, config)
			allIssues = append(allIssues, issues...)

			return nil
		},
	)

	if err != nil {
		slog.Error("Error analyzing target", "error", err)
		os.Exit(1)
	}

	// Print statistics
	if !jsonOutput {
		fmt.Fprintf(os.Stderr, "\nAnalyzed %d files (%d lines of code)\n", filesAnalyzed, totalLines)
	} else {
		slog.Debug("Analysis complete", "files", filesAnalyzed, "lines", totalLines)
	}

	return allIssues
}

func outputJSON(target string, issues []*analyzer.Issue) {
	// Group issues by file for file stats
	fileStats := make(map[string]int)
	for _, issue := range issues {
		fileStats[issue.Position.Filename]++
	}

	// Calculate summary
	high, medium, low := countBySeverity(issues)
	summary := Summary{
		TotalIssues: len(issues),
		High:        high,
		Medium:      medium,
		Low:         low,
	}

	// Create JSON output
	output := JSONOutput{
		Target:    target,
		Summary:   summary,
		Issues:    issues,
		FileStats: fileStats,
	}

	// Marshal and print JSON
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		slog.Error("Error marshaling JSON", "error", err)
		os.Exit(1)
	}

	fmt.Println(string(jsonData))
}

func outputHuman(_ string, issues []*analyzer.Issue) {
	if len(issues) == 0 {
		fmt.Println("✅ No performance issues found!")
		return
	}

	fmt.Printf("\n🚨 Found %d performance issues:\n\n", len(issues))

	// Check if compact mode (IDE-friendly output)
	compactMode := os.Getenv("AIBSCLEANER_COMPACT") == "1"

	if compactMode {
		// Compact mode: one line per issue, IDE-clickable format
		var sb strings.Builder
		sb.Grow(len(issues) * 150) // Pre-allocate

		for _, issue := range issues {
			severityIcon := getSeverityIcon(issue.Severity)
			// Standard compiler error format that all IDEs understand
			sb.WriteString(
				fmt.Sprintf(
					"%s:%d:%d: %s [%s] %s - %s\n",
					issue.Position.Filename, issue.Position.Line, issue.Position.Column,
					severityIcon, issue.Type, issue.Message, issue.Suggestion,
				),
			)
		}
		fmt.Print(sb.String())
	} else {
		// Group issues by analyzer type
		analyzerGroups := getAnalyzerGroups()
		groupedIssues := make(map[string][]*analyzer.Issue)

		for _, issue := range issues {
			group := getAnalyzerGroup(issue.Type)
			groupedIssues[group] = append(groupedIssues[group], issue)
		}

		// Build output using string builder for efficiency
		var sb strings.Builder
		sb.Grow(len(issues) * 200) // Pre-allocate space

		// Print issues grouped by analyzer category
		for _, group := range analyzerGroups {
			groupIssues, exists := groupedIssues[group.Name]
			if !exists || len(groupIssues) == 0 {
				continue
			}
			sb.WriteString(fmt.Sprintf("%s %s (%d issues):\n", group.Icon, group.Name, len(groupIssues)))
			sb.WriteString(strings.Repeat("─", 50))
			sb.WriteString("\n")

			for _, issue := range groupIssues {
				severityIcon := getSeverityIcon(issue.Severity)
				// Format: file:line:column - this format is clickable in most IDEs
				sb.WriteString(
					fmt.Sprintf(
						"  %s %s:%d:%d [%s]\n",
						severityIcon, issue.Position.Filename, issue.Position.Line, issue.Position.Column, issue.Type,
					),
				)
				sb.WriteString(fmt.Sprintf("     %s\n", issue.Message))
				if issue.Suggestion != "" {
					sb.WriteString(fmt.Sprintf("     💡 %s\n", issue.Suggestion))
				}
			}
			sb.WriteString("\n")
		}

		// Print all at once
		fmt.Print(sb.String())
	}

	// Summary
	high, medium, low := countBySeverity(issues)
	fmt.Printf("📊 Summary: %d HIGH, %d MEDIUM, %d LOW\n", high, medium, low)
}

func analyzeFile(filename string, config *Config) []*analyzer.Issue {
	if config == nil {
		return nil
	}

	// Try to load from cache first
	if cachedIssues, found := loadCachedIssues(filename, cacheDB); found {
		return cachedIssues
	}

	// Parse the Go file
	fset, node, err := parseGoFile(filename)
	if err != nil {
		return nil
	}

	// Build enabled analyzers map from config
	enabledAnalyzers := buildEnabledAnalyzers(config)

	// Use centralized AnalyzeAll function with config
	issues := analyzer.Analyze(filename, node, fset, enabledAnalyzers)

	// Filter out issues that have ignore comments
	allIssues := analyzer.FilterIssuesByComments(issues, fset, node)

	// Save to cache
	saveToCacheDB(filename, allIssues, cacheDB)

	return allIssues
}

func getSeverityIcon(severity analyzer.SeverityLevel) string {
	switch severity {
	case analyzer.SeverityLevelCritical:
		return "🚨"
	case analyzer.SeverityLevelHigh:
		return "🔴"
	case analyzer.SeverityLevelMedium:
		return "🟡"
	case analyzer.SeverityLevelLow:
		return "🟢"
	default:
		return "⚪"
	}
}

type analyzerGroup struct {
	Name  string
	Icon  string
	Types []analyzer.IssueType
}

func getAnalyzerGroups() []analyzerGroup {
	groups := []analyzerGroup{
		getAIGroup(),
		getMemoryGroup(),
		getConcurrencyGroup(),
		getPerformanceGroup(),
		getDeferGroup(),
		getStringGroup(),
		getReflectionGroup(),
		getTimeRegexGroup(),
		getNetworkGroup(),
		getDatabaseGroup(),
		getErrorGroup(),
		getQualityGroup(),
		getContextGroup(),
		getOptimizationGroup(),
		getTestGroup(),
		getPrivacyGroup(),
		getDependenciesGroup(),
		getOtherGroup(),
	}
	return groups
}

func getMemoryGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Memory & GC",
		Icon: "💾",
		Types: []analyzer.IssueType{
			analyzer.IssueMemoryLeak, analyzer.IssueGlobalVar, analyzer.IssueLargeAllocation, analyzer.IssueHighGCPressure,
			analyzer.IssueFrequentAllocation, analyzer.IssueLargeHeapAlloc, analyzer.IssuePointerHeavyStruct,
			analyzer.IssueSliceCapacity, analyzer.IssueSliceCopy, analyzer.IssueSliceAppend, analyzer.IssueSliceRangeCopy,
			analyzer.IssueMapCapacity, analyzer.IssueMapClear, analyzer.IssueInterfaceAllocation, analyzer.IssueEmptyInterface,
		},
	}
}

func getConcurrencyGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Concurrency & Race Conditions",
		Icon: "🔄",
		Types: []analyzer.IssueType{
			analyzer.IssueRaceCondition, analyzer.IssueRaceConditionGlobal, analyzer.IssueUnsyncMapAccess, analyzer.IssueRaceClosure,
			analyzer.IssueGoroutineLeak, analyzer.IssueUnbufferedChannel, analyzer.IssueGoroutineOverhead, analyzer.IssueSyncMutexValue,
			analyzer.IssueWaitgroupMisuse, analyzer.IssueRaceInDefer, analyzer.IssueAtomicMisuse, analyzer.IssueGoroutinePerRequest,
			analyzer.IssueNoWorkerPool, analyzer.IssueUnbufferedSignalChan, analyzer.IssueSelectDefault, analyzer.IssueChannelSize,
			analyzer.IssueRangeOverChannel,
		},
	}
}

func getPerformanceGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Performance Hotspots",
		Icon: "🔥",
		Types: []analyzer.IssueType{
			analyzer.IssueAllocInLoop, analyzer.IssueNestedLoop, analyzer.IssueStringConcatInLoop, analyzer.IssueAppendInLoop,
			analyzer.IssueDeferInLoop, analyzer.IssueRegexInLoop, analyzer.IssueTimeInLoop, analyzer.IssueSQLInLoop,
			analyzer.IssueDNSInLoop,
			analyzer.IssueReflectionInLoop, analyzer.IssueCPUIntensiveLoop, analyzer.IssueUnnecessaryCopy,
			analyzer.IssueBoundsCheckElimination,
			analyzer.IssueInefficientAlgorithm, analyzer.IssueCacheUnfriendly,
		},
	}
}

func getDeferGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Defer Optimization",
		Icon: "⏰",
		Types: []analyzer.IssueType{
			analyzer.IssueDeferInShortFunc, analyzer.IssueDeferOverhead, analyzer.IssueUnnecessaryDefer, analyzer.IssueDeferAtEnd,
			analyzer.IssueMultipleDefers, analyzer.IssueDeferInHotPath, analyzer.IssueDeferLargeCapture,
			analyzer.IssueUnnecessaryMutexDefer,
			analyzer.IssueMissingDeferUnlock, analyzer.IssueMissingDeferClose,
		},
	}
}

func getStringGroup() analyzerGroup {
	return analyzerGroup{
		Name:  "String Operations",
		Icon:  "📝",
		Types: []analyzer.IssueType{analyzer.IssueStringConcat, analyzer.IssueStringBuilder},
	}
}

func getReflectionGroup() analyzerGroup {
	return analyzerGroup{
		Name:  "Reflection & Interfaces",
		Icon:  "🔍",
		Types: []analyzer.IssueType{analyzer.IssueReflection, analyzer.IssueInterfacePollution},
	}
}

func getTimeRegexGroup() analyzerGroup {
	return analyzerGroup{
		Name:  "Time & Regex",
		Icon:  "⏱️",
		Types: []analyzer.IssueType{analyzer.IssueTimeAfterLeak, analyzer.IssueTimeFormat, analyzer.IssueRegexCompile},
	}
}

func getNetworkGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Network & HTTP",
		Icon: "🌐",
		Types: []analyzer.IssueType{
			analyzer.IssueHTTPNoTimeout, analyzer.IssueHTTPNoClose, analyzer.IssueHTTPDefaultClient, analyzer.IssueHTTPNoContext,
			analyzer.IssueKeepaliveMissing, analyzer.IssueConnectionPool, analyzer.IssueNoReuseConnection,
		},
	}
}

func getDatabaseGroup() analyzerGroup {
	return analyzerGroup{
		Name:  "Database",
		Icon:  "🗄️",
		Types: []analyzer.IssueType{analyzer.IssueNoPreparedStmt, analyzer.IssueMissingDBClose},
	}
}

func getErrorGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Error Handling",
		Icon: "⚠️",
		Types: []analyzer.IssueType{
			analyzer.IssueErrorIgnored, analyzer.IssueErrorCheckMissing, analyzer.IssuePanicRecover, analyzer.IssueErrorStringFormat,
			analyzer.IssueNilCheck, analyzer.IssuePanicRisk, analyzer.IssueNilReturn, analyzer.IssuePanicInLibrary,
		},
	}
}

func getQualityGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Code Quality",
		Icon: "🎯",
		Types: []analyzer.IssueType{
			analyzer.IssueHighComplexityO3, analyzer.IssueHighComplexityO2,
			analyzer.IssuePointerToSlice, analyzer.IssueUselessCondition, analyzer.IssueEmptyElse,
			analyzer.IssueSleepInsteadOfSync, analyzer.IssueConsoleLogDebugging,
			analyzer.IssueHardcodedConfig, analyzer.IssuePanicInLibrary, analyzer.IssueGlobalVariable,
		},
	}
}

func getContextGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Context & API",
		Icon: "⚡",
		Types: []analyzer.IssueType{
			analyzer.IssueContextBackground, analyzer.IssueContextValue, analyzer.IssueMissingContextCancel, analyzer.IssueContextLeak,
			analyzer.IssueContextInStruct, analyzer.IssueContextNotFirst, analyzer.IssueSyncPoolMisuse, analyzer.IssueContextMisuse,
			analyzer.IssueWGMisuse,
		},
	}
}

func getOptimizationGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Optimization Opportunities",
		Icon: "💡",
		Types: []analyzer.IssueType{
			analyzer.IssueSyncPoolOpportunity, analyzer.IssueSyncPoolPutMissing, analyzer.IssueSyncPoolTypeAssert,
		},
	}
}

func getTestGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Test Coverage",
		Icon: "🧪",
		Types: []analyzer.IssueType{
			analyzer.IssueMissingTest, analyzer.IssueMissingExample, analyzer.IssueMissingBenchmark, analyzer.IssueUntestedExport,
			analyzer.IssueUntestedType, analyzer.IssueUntestedError, analyzer.IssueUntestedConcurrency,
			analyzer.IssueUntestedIOFunction,
		},
	}
}

func getPrivacyGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Privacy & Security",
		Icon: "🔒",
		Types: []analyzer.IssueType{
			analyzer.IssuePrivacyHardcodedSecret, analyzer.IssuePrivacyAWSKey, analyzer.IssuePrivacyJWTToken,
			analyzer.IssuePrivacyEmailPII, analyzer.IssuePrivacySSNPII, analyzer.IssuePrivacyCreditCardPII,
			analyzer.IssuePrivacyLoggingSensitive, analyzer.IssuePrivacyPrintingSensitive, analyzer.IssuePrivacyExposedField,
			analyzer.IssuePrivacyUnencryptedDBWrite, analyzer.IssuePrivacyDirectInputToDB,
		},
	}
}

func getDependenciesGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Dependencies",
		Icon: "📦",
		Types: []analyzer.IssueType{
			analyzer.IssueDependencyDeprecated, analyzer.IssueDependencyVulnerable, analyzer.IssueDependencyOutdated,
			analyzer.IssueDependencyCGO, analyzer.IssueDependencyUnsafe, analyzer.IssueDependencyInternal,
			analyzer.IssueDependencyIndirect,
			analyzer.IssueDependencyLocalReplace, analyzer.IssueDependencyNoChecksum, analyzer.IssueDependencyEmptyChecksum,
			analyzer.IssueDependencyVersionConflict,
		},
	}
}

func getOtherGroup() analyzerGroup {
	return analyzerGroup{
		Name:  "Other",
		Icon:  "📌",
		Types: []analyzer.IssueType{}, // Catches any uncategorized issues
	}
}

func getAIGroup() analyzerGroup {
	return analyzerGroup{
		Name: "AI Bullshit Detection",
		Icon: "🤖",
		Types: []analyzer.IssueType{
			analyzer.IssueAIBullshitConcurrency, analyzer.IssueAIReflectionOverkill, analyzer.IssueAIPatternAbuse,
			analyzer.IssueAIEnterpriseHelloWorld, analyzer.IssueAICaptainObvious, analyzer.IssueAIOverengineeredSimple,
			analyzer.IssueAIGeneratedComment, analyzer.IssueAIUnnecessaryComplexity, analyzer.IssueAIVariable,
			analyzer.IssueAIErrorHandling,
			analyzer.IssueAIStructure, analyzer.IssueAIRepetition, analyzer.IssueAIFactorySimple, analyzer.IssueAIRedundantElse,
		},
	}
}

func getAnalyzerGroup(issueType analyzer.IssueType) string {
	groups := getAnalyzerGroups()
	for _, group := range groups {
		for _, t := range group.Types {
			if t == issueType {
				return group.Name
			}
		}
	}
	return "Other" // Default group for uncategorized issues
}

func countBySeverity(issues []*analyzer.Issue) (high, medium, low int) {
	for _, issue := range issues {
		switch issue.Severity {
		case analyzer.SeverityLevelCritical, analyzer.SeverityLevelHigh:
			high++
		case analyzer.SeverityLevelMedium:
			medium++
		case analyzer.SeverityLevelLow:
			low++
		}
	}
	return
}

func createDefaultConfig() {
	config := DefaultConfig()

	// Create YAML config
	yamlData, err := yaml.Marshal(config)
	if err != nil {
		slog.Error("Failed to marshal config", "error", err)
	}

	const configFile = ".aibscleaner.yaml"
	const configFileMode = 0644
	err = os.WriteFile(configFile, yamlData, configFileMode)
	if err != nil {
		slog.Error("Failed to write config file", "error", err)
	}

	fmt.Printf("✅ Created default configuration file: %s\n", configFile)
	fmt.Println("📝 Edit this file to customize your analysis settings")
	fmt.Println("")
	fmt.Println("Example usage:")
	fmt.Println("  aibscleaner --config=.aibscleaner.yaml .")
}

// getProjectRoot finds the project root (directory with go.mod)
func getProjectRoot(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	// If it's a file, start from its directory
	if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
		absPath = filepath.Dir(absPath)
	}

	// Walk up to find go.mod
	current := absPath
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			// Reached root, return original path
			return absPath
		}
		current = parent
	}
}
