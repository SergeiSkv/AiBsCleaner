package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/SergeiSkv/AiBsCleaner/analyzer"
	"github.com/SergeiSkv/AiBsCleaner/cache"
	"github.com/SergeiSkv/AiBsCleaner/models"
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
	Target    string          `json:"target"`
	Summary   Summary         `json:"summary"`
	Issues    []*models.Issue `json:"issues"`
	FileStats []fileStat      `json:"file_stats"`
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
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("AiBsCleaner version %s\n", version.Version))
		sb.WriteString(fmt.Sprintf("Commit: %s\n", version.CommitHash))
		sb.WriteString(fmt.Sprintf("Built: %s\n", version.BuiltAt))
		sb.WriteString("\nStop AI bullshit, write performant Go!\n")
		fmt.Print(sb.String())
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

		var sb strings.Builder
		sb.WriteString("Cache Statistics:\n")
		sb.WriteString("====================\n")
		sb.WriteString(fmt.Sprintf("Total files analyzed:  %d\n", stats["total_files"]))
		sb.WriteString(fmt.Sprintf("Total issues found:    %d\n", stats["total_issues"]))
		sb.WriteString(fmt.Sprintf("Ignored issues:        %d\n", stats["ignored_issues"]))
		sb.WriteString(fmt.Sprintf("Fixed issues:          %d\n", stats["fixed_issues"]))
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("Cache location: %s\n", cacheDB.GetCacheDir()))

		if fileInfo, err := os.Stat(filepath.Join(cacheDB.GetCacheDir(), cache.CacheFile)); err == nil {
			sb.WriteString(fmt.Sprintf("Cache size:     %.2f MB\n", float64(fileInfo.Size())/(1024*1024)))
		}
		fmt.Print(sb.String())
	},
}

var analyzers = []struct {
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
	{"StructLayoutAnalyzer", "Optimizes struct field alignment and memory layout"},
	{"PrivacyAnalyzer", "Detects privacy issues and data leaks"},
	{"DependencyAnalyzer", "Checks dependency health and vulnerabilities"},
}

var listCmd = &cobra.Command{
	Use:   "list-analyzers",
	Short: "List all available analyzers",
	Long:  `Shows all available analyzers and their detection patterns.`,
	Run: func(cmd *cobra.Command, args []string) {
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

func analyzeTarget(target string, config *Config) []*models.Issue {
	if config == nil {
		return nil
	}
	var allIssues []*models.Issue
	var filesAnalyzed int
	var totalLines int

	// Run dependency analysis ONCE for the entire project
	depIssues := analyzer.AnalyzeDependencies(target)
	allIssues = append(allIssues, depIssues...)

	// Collect all Go files first
	var filesToAnalyze []string
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

			// Collect only Go files
			if isGoFile(path, info) {
				filesToAnalyze = append(filesToAnalyze, path)
			}

			return nil
		},
	)

	if err != nil {
		slog.Error("Error scanning target", "error", err)
		os.Exit(1)
	}

	// Analyze files in parallel
	numWorkers := runtime.NumCPU()
	var wg sync.WaitGroup
	var mu sync.Mutex
	fileChan := make(chan string, len(filesToAnalyze))

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range fileChan {
				issues := analyzeFile(path, config)
				lines := countLines(path)

				mu.Lock()
				allIssues = append(allIssues, issues...)
				filesAnalyzed++
				totalLines += lines
				mu.Unlock()
			}
		}()
	}

	// Send files to workers
	for _, path := range filesToAnalyze {
		fileChan <- path
	}
	close(fileChan)

	// Wait for all workers to finish
	wg.Wait()

	// Print statistics
	if !jsonOutput {
		fmt.Fprintf(os.Stderr, "\nAnalyzed %d files (%d lines of code)\n", filesAnalyzed, totalLines)
	} else {
		slog.Debug("Analysis complete", "files", filesAnalyzed, "lines", totalLines)
	}

	return allIssues
}

type fileStat struct {
	Filename string
	Count    int
}

func outputJSON(target string, issues []*models.Issue) {
	// Используем срез для подсчёта по файлам
	var fileStats []fileStat

	// Функция для поиска индекса файла в срезе
	findFileIndex := func(filename string) int {
		for i, f := range fileStats {
			if f.Filename == filename {
				return i
			}
		}
		return -1
	}

	for _, issue := range issues {
		idx := findFileIndex(issue.Position.Filename)
		if idx == -1 {
			fileStats = append(
				fileStats, fileStat{
					Filename: issue.Position.Filename,
					Count:    1,
				},
			)
		} else {
			fileStats[idx].Count++
		}
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
		FileStats: fileStats, // тут уже срез вместо map
	}

	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(output); err != nil {
		slog.Error("Error encoding JSON", "error", err)
		os.Exit(1)
	}
}

func outputHuman(_ string, issues []*models.Issue) {
	if len(issues) == 0 {
		fmt.Println("✅ No performance issues found!")
		return
	}

	compactMode := os.Getenv("AIBSCLEANER_COMPACT") == "1"
	if compactMode {
		printCompactIssues(issues)
	} else {
		printGroupedIssues(issues)
	}

	printSummary(issues)
}

func printSummary(issues []*models.Issue) {
	high, medium, low := countBySeverity(issues)
	fmt.Printf("Summary: %d HIGH, %d MEDIUM, %d LOW\n", high, medium, low)
}

type groupWithIssues struct {
	group  analyzerGroup
	issues []*models.Issue
}

func printGroupedIssues(issues []*models.Issue) {
	grouped := groupIssuesByAnalyzer(issues)
	output := buildGroupedOutput(grouped)
	fmt.Print(output)
}

func groupIssuesByAnalyzer(issues []*models.Issue) []groupWithIssues {
	analyzerGroups := getAnalyzerGroups()
	grouped := make([]groupWithIssues, len(analyzerGroups))

	// Use more conservative capacity estimation to avoid reallocations
	estimatedCapacityPerGroup := (len(issues)/len(analyzerGroups) + 1) * 2
	if estimatedCapacityPerGroup < 20 {
		estimatedCapacityPerGroup = 20
	}
	for i, g := range analyzerGroups {
		grouped[i] = groupWithIssues{
			group:  g,
			issues: make([]*models.Issue, 0, estimatedCapacityPerGroup),
		}
	}

	// Group issues by analyzer type
	for _, issue := range issues {
		groupName := getAnalyzerGroup(issue.Type)
		for i := range grouped {
			if grouped[i].group.Name == groupName {
				grouped[i].issues = append(grouped[i].issues, issue)
				break
			}
		}
	}

	// Sort issues within each group: first by severity (HIGH, MEDIUM, LOW), then by PVE code
	for i := range grouped {
		sort.Slice(grouped[i].issues, func(a, b int) bool {
			issueA := grouped[i].issues[a]
			issueB := grouped[i].issues[b]

			// First sort by severity: HIGH first, then MEDIUM, then LOW
			if issueA.Severity != issueB.Severity {
				return issueA.Severity > issueB.Severity
			}

			// Then sort by PVE code (issue type number)
			return issueA.Type < issueB.Type
		})
	}

	return grouped
}

func buildGroupedOutput(grouped []groupWithIssues) string {
	var sb strings.Builder
	// Estimate size based on number of issues
	totalIssues := 0
	for _, g := range grouped {
		totalIssues += len(g.issues)
	}
	sb.Grow(totalIssues * 200)

	for _, g := range grouped {
		if len(g.issues) == 0 {
			continue
		}
		addGroupHeader(&sb, &g)
		addGroupIssues(&sb, g.issues)
		sb.WriteString("\n")
	}
	return sb.String()
}

func addGroupHeader(sb *strings.Builder, g *groupWithIssues) {
	sb.WriteString(g.group.Icon)
	sb.WriteString(" ")
	sb.WriteString(g.group.Name)
	sb.WriteString(" (")
	sb.WriteString(strconv.Itoa(len(g.issues)))
	sb.WriteString(" issues):\n")
	sb.WriteString(strings.Repeat("─", 50) + "\n")
}

func addGroupIssues(sb *strings.Builder, issues []*models.Issue) {
	for _, issue := range issues {
		severityIcon := getSeverityIcon(issue.Severity)
		sb.WriteString("\t")
		sb.WriteString(severityIcon)
		sb.WriteString(" ")
		sb.WriteString(issue.Position.Filename)
		sb.WriteString(":")
		sb.WriteString(strconv.Itoa(issue.Position.Line))
		sb.WriteString(":")
		sb.WriteString(strconv.Itoa(issue.Position.Column))
		sb.WriteString(" [")
		sb.WriteString(issue.Type.GetPVEID())
		sb.WriteString("]\n")
		sb.WriteString("\t\t")
		sb.WriteString(issue.Message)
		sb.WriteString("\n")
		if issue.Suggestion != "" {
			sb.WriteString("\t\t")
			sb.WriteString(issue.Suggestion)
			sb.WriteString("\n")
		}
	}
}

func printCompactIssues(issues []*models.Issue) {
	// Sort issues: first by severity (HIGH, MEDIUM, LOW), then by PVE code
	sort.Slice(issues, func(i, j int) bool {
		// First sort by severity: HIGH first, then MEDIUM, then LOW
		if issues[i].Severity != issues[j].Severity {
			return issues[i].Severity > issues[j].Severity
		}

		// Then sort by PVE code (issue type number)
		return issues[i].Type < issues[j].Type
	})

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
}

func analyzeFile(filename string, config *Config) []*models.Issue {
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

func getSeverityIcon(severity models.SeverityLevel) string {
	switch severity {
	case models.SeverityLevelHigh:
		return "🔴"
	case models.SeverityLevelMedium:
		return "🟡"
	case models.SeverityLevelLow:
		return "🟢"
	default:
		return "⚪"
	}
}

type analyzerGroup struct {
	Name  string
	Icon  string
	Types []models.IssueType
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
		Types: []models.IssueType{
			models.IssueMemoryLeak, models.IssueGlobalVar, models.IssueLargeAllocation, models.IssueHighGCPressure,
			models.IssueFrequentAllocation, models.IssueLargeHeapAlloc, models.IssuePointerHeavyStruct,
			models.IssueSliceCapacity, models.IssueSliceCopy, models.IssueSliceAppend, models.IssueSliceRangeCopy,
			models.IssueMapCapacity, models.IssueMapClear, models.IssueInterfaceAllocation, models.IssueEmptyInterface,
			models.IssueStructLayoutUnoptimized, models.IssueStructLargePadding, models.IssueStructFieldAlignment,
			models.IssueCacheFalseSharing, models.IssueCacheLineWaste, models.IssueCacheLineAlignment,
			models.IssueOversizedType, models.IssueUnspecificIntType, models.IssueSoAPattern,
			models.IssueNestedRangeCache, models.IssueMapRangeCache,
		},
	}
}

func getConcurrencyGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Concurrency & Race Conditions",
		Icon: "🔄",
		Types: []models.IssueType{
			models.IssueRaceCondition, models.IssueRaceConditionGlobal, models.IssueUnsyncMapAccess, models.IssueRaceClosure,
			models.IssueGoroutineLeak, models.IssueUnbufferedChannel, models.IssueGoroutineOverhead, models.IssueSyncMutexValue,
			models.IssueWaitgroupMisuse, models.IssueRaceInDefer, models.IssueAtomicMisuse, models.IssueGoroutinePerRequest,
			models.IssueNoWorkerPool, models.IssueUnbufferedSignalChan, models.IssueSelectDefault, models.IssueChannelSize,
			models.IssueRangeOverChannel,
		},
	}
}

func getPerformanceGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Performance Hotspots",
		Icon: "🔥",
		Types: []models.IssueType{
			models.IssueAllocInLoop, models.IssueNestedLoop, models.IssueAppendInLoop,
			models.IssueDeferInLoop, models.IssueRegexInLoop, models.IssueTimeInLoop, models.IssueSQLInLoop,
			models.IssueDNSInLoop,
			models.IssueReflectionInLoop, models.IssueCPUIntensiveLoop, models.IssueUnnecessaryCopy,
			models.IssueBoundsCheckElimination,
			models.IssueInefficientAlgorithm, models.IssueCacheUnfriendly,
		},
	}
}

func getDeferGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Defer Optimization",
		Icon: "⏰",
		Types: []models.IssueType{
			models.IssueDeferInShortFunc, models.IssueDeferOverhead, models.IssueUnnecessaryDefer, models.IssueDeferAtEnd,
			models.IssueMultipleDefers, models.IssueDeferInHotPath, models.IssueDeferLargeCapture,
			models.IssueUnnecessaryMutexDefer,
			models.IssueMissingDeferUnlock, models.IssueMissingDeferClose,
		},
	}
}

func getStringGroup() analyzerGroup {
	return analyzerGroup{
		Name:  "String Operations",
		Icon:  "📝",
		Types: []models.IssueType{models.IssueStringConcat, models.IssueStringBuilder},
	}
}

func getReflectionGroup() analyzerGroup {
	return analyzerGroup{
		Name:  "Reflection & Interfaces",
		Icon:  "🔍",
		Types: []models.IssueType{models.IssueReflection, models.IssueInterfacePollution},
	}
}

func getTimeRegexGroup() analyzerGroup {
	return analyzerGroup{
		Name:  "Time & Regex",
		Icon:  "⏱️",
		Types: []models.IssueType{models.IssueTimeAfterLeak, models.IssueTimeFormat, models.IssueRegexCompile},
	}
}

func getNetworkGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Network & HTTP",
		Icon: "🌐",
		Types: []models.IssueType{
			models.IssueHTTPNoTimeout, models.IssueHTTPNoClose, models.IssueHTTPDefaultClient, models.IssueHTTPNoContext,
			models.IssueKeepaliveMissing, models.IssueConnectionPool, models.IssueNoReuseConnection,
		},
	}
}

func getDatabaseGroup() analyzerGroup {
	return analyzerGroup{
		Name:  "Database",
		Icon:  "🗄️",
		Types: []models.IssueType{models.IssueNoPreparedStmt, models.IssueMissingDBClose},
	}
}

func getErrorGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Error Handling",
		Icon: "⚠️",
		Types: []models.IssueType{
			models.IssueErrorIgnored, models.IssueErrorCheckMissing, models.IssuePanicRecover, models.IssueErrorStringFormat,
			models.IssuePanicRisk, models.IssuePanicInLibrary,
		},
	}
}

func getQualityGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Code Quality",
		Icon: "🎯",
		Types: []models.IssueType{
			models.IssueHighComplexityO3, models.IssueHighComplexityO2,
			models.IssuePointerToSlice, models.IssueUselessCondition, models.IssueEmptyElse,
			models.IssueSleepInsteadOfSync, models.IssueConsoleLogDebugging,
			models.IssueHardcodedConfig, models.IssuePanicInLibrary, models.IssueGlobalVariable,
		},
	}
}

func getContextGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Context & API",
		Icon: "⚡",
		Types: []models.IssueType{
			models.IssueContextBackground, models.IssueContextValue, models.IssueMissingContextCancel, models.IssueContextLeak,
			models.IssueContextInStruct, models.IssueContextNotFirst, models.IssueSyncPoolMisuse, models.IssueContextMisuse,
			models.IssueWGMisuse,
		},
	}
}

func getOptimizationGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Optimization Opportunities",
		Icon: "💡",
		Types: []models.IssueType{
			models.IssueSyncPoolOpportunity, models.IssueSyncPoolPutMissing, models.IssueSyncPoolTypeAssert,
		},
	}
}

func getTestGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Test Coverage",
		Icon: "🧪",
		Types: []models.IssueType{
			models.IssueMissingTest, models.IssueMissingExample, models.IssueMissingBenchmark, models.IssueUntestedExport,
			models.IssueUntestedType, models.IssueUntestedError, models.IssueUntestedConcurrency,
			models.IssueUntestedIOFunction,
		},
	}
}

func getPrivacyGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Privacy & Security",
		Icon: "🔒",
		Types: []models.IssueType{
			models.IssuePrivacyHardcodedSecret, models.IssuePrivacyAWSKey, models.IssuePrivacyJWTToken,
			models.IssuePrivacyEmailPII, models.IssuePrivacySSNPII, models.IssuePrivacyCreditCardPII,
			models.IssuePrivacyLoggingSensitive, models.IssuePrivacyPrintingSensitive, models.IssuePrivacyExposedField,
			models.IssuePrivacyUnencryptedDBWrite, models.IssuePrivacyDirectInputToDB,
		},
	}
}

func getDependenciesGroup() analyzerGroup {
	return analyzerGroup{
		Name: "Dependencies",
		Icon: "📦",
		Types: []models.IssueType{
			models.IssueDependencyDeprecated, models.IssueDependencyVulnerable, models.IssueDependencyOutdated,
			models.IssueDependencyCGO, models.IssueDependencyUnsafe, models.IssueDependencyInternal,
			models.IssueDependencyIndirect,
			models.IssueDependencyLocalReplace, models.IssueDependencyNoChecksum, models.IssueDependencyEmptyChecksum,
			models.IssueDependencyVersionConflict,
		},
	}
}

func getOtherGroup() analyzerGroup {
	return analyzerGroup{
		Name:  "Other",
		Icon:  "📌",
		Types: []models.IssueType{}, // Catches any uncategorized issues
	}
}

func getAIGroup() analyzerGroup {
	return analyzerGroup{
		Name: "AI Bullshit Detection",
		Icon: "🤖",
		Types: []models.IssueType{
			models.IssueAIBullshitConcurrency, models.IssueAIReflectionOverkill, models.IssueAIPatternAbuse,
			models.IssueAIEnterpriseHelloWorld, models.IssueAICaptainObvious, models.IssueAIOverengineeredSimple,
			models.IssueAIGeneratedComment, models.IssueAIUnnecessaryComplexity, models.IssueAIVariable,
			models.IssueAIErrorHandling,
			models.IssueAIStructure, models.IssueAIRepetition, models.IssueAIFactorySimple, models.IssueAIRedundantElse,
		},
	}
}

func getAnalyzerGroup(issueType models.IssueType) string {
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

func countBySeverity(issues []*models.Issue) (high, medium, low int) {
	for _, issue := range issues {
		switch issue.Severity {
		case models.SeverityLevelHigh:
			high++
		case models.SeverityLevelMedium:
			medium++
		case models.SeverityLevelLow:
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

	fmt.Printf("Created default configuration file: %s\n", configFile)
	fmt.Println("\tEdit this file to customize your analysis settings")
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
