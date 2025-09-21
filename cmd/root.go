package cmd

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/SergeiSkv/AiBsCleaner/analyzer"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	jsonOutput bool
	configPath string
	compact    bool
	verbose    bool
	version    = "1.0.0"
)

// JSONOutput represents the JSON structure for results
type JSONOutput struct {
	Target    string           `json:"target"`
	Summary   Summary          `json:"summary"`
	Issues    []analyzer.Issue `json:"issues"`
	FileStats map[string]int   `json:"file_stats"`
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
	Example: `  aibscleaner .                        # Analyze current directory
  aibscleaner ./src                    # Analyze specific directory
  aibscleaner main.go                  # Analyze single file
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
			log.Fatalf("Path %s does not exist", target)
		}

		// Load configuration
		config, err := LoadConfig(configPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}

		if !jsonOutput && verbose {
			fmt.Printf("🔍 Analyzing Go code in %s for performance issues...\n", target)
		}

		// Set compact mode from flag
		if compact {
			os.Setenv("AIBSCLEANER_COMPACT", "1")
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
	Long:  `Creates a .perfchecker.yaml configuration file with default settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		createDefaultConfig()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("AiBsCleaner version %s\n", version)
		fmt.Println("Stop AI bullshit, write performant Go!")
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

	rootCmd.AddCommand(initConfigCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(listCmd)
}

func Execute() error {
	return rootCmd.Execute()
}

func analyzeTarget(target string, config *Config) []analyzer.Issue {
	if config == nil {
		return nil
	}
	var allIssues []analyzer.Issue
	var filesAnalyzed int
	var totalLines int

	// Run dependency analysis ONCE for the entire project
	depIssues := analyzer.AnalyzeDependencies(target)
	allIssues = append(allIssues, depIssues...)

	err := filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check exclusions for both files and directories
		for _, exclude := range config.Paths.Exclude {
			if strings.HasSuffix(exclude, ".go") {
				// File pattern (e.g., "_test.go")
				if !info.IsDir() && strings.HasSuffix(path, exclude) {
					return nil
				}
			} else {
				// Directory exclusion
				// For directories, check if this is the excluded directory
				if info.IsDir() && filepath.Base(path) == exclude {
					return filepath.SkipDir
				}
				// For files, check if they're in an excluded directory
				if !info.IsDir() && strings.Contains(path, string(filepath.Separator)+exclude+string(filepath.Separator)) {
					return nil
				}
			}
		}

		// Skip non-Go files
		if !info.IsDir() && !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip directories (we only analyze files)
		if info.IsDir() {
			return nil
		}

		// Count the file
		filesAnalyzed++

		// Count lines in file
		if content, err := os.ReadFile(path); err == nil {
			totalLines += strings.Count(string(content), "\n") + 1
		}

		issues := analyzeFile(path, config)
		allIssues = append(allIssues, issues...)

		return nil
	})

	if err != nil {
		log.Fatalf("Error analyzing target: %v", err)
	}

	// Print statistics
	fmt.Fprintf(os.Stderr, "\n📊 Analyzed %d files (%d lines of code)\n", filesAnalyzed, totalLines)

	return allIssues
}

func outputJSON(target string, issues []analyzer.Issue) {
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
		log.Fatalf("Error marshaling JSON: %v", err)
	}

	fmt.Println(string(jsonData))
}

func outputHuman(target string, issues []analyzer.Issue) {
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
			sb.WriteString(fmt.Sprintf("%s:%d:%d: %s [%s] %s - %s\n",
				issue.Position.Filename, issue.Position.Line, issue.Position.Column,
				severityIcon, issue.Type, issue.Message, issue.Suggestion))
		}
		fmt.Print(sb.String())
	} else {
		// Group issues by analyzer type
		analyzerGroups := getAnalyzerGroups()
		groupedIssues := make(map[string][]analyzer.Issue)

		for _, issue := range issues {
			group := getAnalyzerGroup(issue.Type)
			groupedIssues[group] = append(groupedIssues[group], issue)
		}

		// Build output using string builder for efficiency
		var sb strings.Builder
		sb.Grow(len(issues) * 200) // Pre-allocate space

		// Print issues grouped by analyzer category
		for _, group := range analyzerGroups {
			if groupIssues, exists := groupedIssues[group.Name]; exists && len(groupIssues) > 0 {
				sb.WriteString(fmt.Sprintf("%s %s (%d issues):\n", group.Icon, group.Name, len(groupIssues)))
				sb.WriteString(strings.Repeat("─", 50))
				sb.WriteString("\n")

				for _, issue := range groupIssues {
					severityIcon := getSeverityIcon(issue.Severity)
					// Format: file:line:column - this format is clickable in most IDEs
					sb.WriteString(fmt.Sprintf("  %s %s:%d:%d [%s]\n",
						severityIcon, issue.Position.Filename, issue.Position.Line, issue.Position.Column, issue.Type))
					sb.WriteString(fmt.Sprintf("     %s\n", issue.Message))
					if issue.Suggestion != "" {
						sb.WriteString(fmt.Sprintf("     💡 %s\n", issue.Suggestion))
					}
				}
				sb.WriteString("\n")
			}
		}

		// Print all at once
		fmt.Print(sb.String())
	}

	// Summary
	high, medium, low := countBySeverity(issues)
	fmt.Printf("📊 Summary: %d HIGH, %d MEDIUM, %d LOW\n", high, medium, low)
}

func analyzeFile(filename string, config *Config) []analyzer.Issue {
	if config == nil {
		return nil
	}
	fset := token.NewFileSet()

	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		log.Printf("Error parsing %s: %v", filename, err)
		return nil
	}

	// Get project path for dependency analyzer
	projectPath := filepath.Dir(filename)
	for {
		goModPath := filepath.Join(projectPath, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			break
		}
		parent := filepath.Dir(projectPath)
		if parent == projectPath {
			// Reached root, use original directory
			projectPath = filepath.Dir(filename)
			break
		}
		projectPath = parent
	}

	// Use centralized Analyze function
	issues := analyzer.Analyze(filename, node, fset, projectPath)

	// Filter issues based on configuration
	var allIssues []analyzer.Issue
	for _, issue := range issues {
		if config.ShouldAnalyze(issue.Type) {
			allIssues = append(allIssues, issue)
		}
	}

	return allIssues
}

func getSeverityIcon(severity analyzer.Severity) string {
	switch severity {
	case analyzer.SeverityHigh:
		return "🔴"
	case analyzer.SeverityMedium:
		return "🟡"
	case analyzer.SeverityLow:
		return "🟢"
	default:
		return "⚪"
	}
}

type analyzerGroup struct {
	Name  string
	Icon  string
	Types []string
}

func getAnalyzerGroups() []analyzerGroup {
	return []analyzerGroup{
		{
			Name: "AI Bullshit Detection",
			Icon: "🤖",
			Types: []string{"AI_BULLSHIT_CONCURRENCY", "AI_REFLECTION_OVERKILL", "AI_PATTERN_ABUSE",
				"AI_ENTERPRISE_HELLO_WORLD", "AI_CAPTAIN_OBVIOUS", "AI_OVERENGINEERED_SIMPLE",
				"AI_COMMENT", "AI_COMPLEXITY", "AI_VARIABLE", "AI_ERROR_HANDLING",
				"AI_STRUCTURE", "AI_REPETITION", "AI_FACTORY_SIMPLE", "AI_REDUNDANT_ELSE"},
		},
		{
			Name: "Memory & GC",
			Icon: "💾",
			Types: []string{"MEMORY_LEAK", "GLOBAL_VAR", "LARGE_ALLOCATION", "HIGH_GC_PRESSURE",
				"FREQUENT_ALLOCATION", "LARGE_HEAP_ALLOC", "POINTER_HEAVY_STRUCT",
				"SLICE_CAPACITY", "SLICE_COPY", "SLICE_APPEND", "SLICE_RANGE_COPY",
				"MAP_CAPACITY", "MAP_CLEAR", "INTERFACE_ALLOCATION", "EMPTY_INTERFACE"},
		},
		{
			Name: "Concurrency & Race Conditions",
			Icon: "🔄",
			Types: []string{"RACE_CONDITION", "RACE_CONDITION_GLOBAL", "UNSYNC_MAP_ACCESS", "RACE_CLOSURE",
				"GOROUTINE_LEAK", "UNBUFFERED_CHANNEL", "GOROUTINE_OVERHEAD", "SYNC_MUTEX_VALUE",
				"WAITGROUP_MISUSE", "RACE_IN_DEFER", "ATOMIC_MISUSE", "GOROUTINE_PER_REQUEST",
				"NO_WORKER_POOL", "UNBUFFERED_SIGNAL_CHAN", "SELECT_DEFAULT", "CHANNEL_SIZE",
				"RANGE_OVER_CHANNEL"},
		},
		{
			Name: "Performance Hotspots",
			Icon: "🔥",
			Types: []string{"ALLOC_IN_LOOP", "NESTED_LOOP", "STRING_CONCAT_IN_LOOP", "APPEND_IN_LOOP",
				"DEFER_IN_LOOP", "REGEX_IN_LOOP", "TIME_IN_LOOP", "SQL_IN_LOOP", "DNS_IN_LOOP",
				"REFLECTION_IN_LOOP", "CPU_INTENSIVE_LOOP", "UNNECESSARY_COPY", "BOUNDS_CHECK_ELIMINATION",
				"INEFFICIENT_ALGORITHM", "CACHE_UNFRIENDLY"},
		},
		{
			Name: "Defer Optimization",
			Icon: "⏰",
			Types: []string{"DEFER_IN_SHORT_FUNC", "DEFER_OVERHEAD", "UNNECESSARY_DEFER", "DEFER_AT_END",
				"MULTIPLE_DEFERS", "DEFER_IN_HOT_PATH", "DEFER_LARGE_CAPTURE", "UNNECESSARY_MUTEX_DEFER",
				"MISSING_DEFER_UNLOCK", "MISSING_DEFER_CLOSE"},
		},
		{
			Name:  "String Operations",
			Icon:  "📝",
			Types: []string{"STRING_CONCAT", "STRING_BUILDER"},
		},
		{
			Name:  "Reflection & Interfaces",
			Icon:  "🔍",
			Types: []string{"REFLECTION", "INTERFACE_POLLUTION"},
		},
		{
			Name:  "Time & Regex",
			Icon:  "⏱️",
			Types: []string{"TIME_AFTER_LEAK", "TIME_FORMAT", "REGEX_COMPILE"},
		},
		{
			Name: "Network & HTTP",
			Icon: "🌐",
			Types: []string{"HTTP_NO_TIMEOUT", "HTTP_NO_CLOSE", "HTTP_DEFAULT_CLIENT", "HTTP_NO_CONTEXT",
				"KEEPALIVE_MISSING", "CONNECTION_POOL", "NO_REUSE_CONNECTION"},
		},
		{
			Name:  "Database",
			Icon:  "🗄️",
			Types: []string{"NO_PREPARED_STMT", "MISSING_DB_CLOSE"},
		},
		{
			Name: "Error Handling",
			Icon: "⚠️",
			Types: []string{"ERROR_IGNORED", "ERROR_CHECK_MISSING", "PANIC_RECOVER", "ERROR_STRING_FORMAT",
				"NIL_CHECK", "PANIC_RISK", "NIL_RETURN", "PANIC_IN_LIBRARY"},
		},
		{
			Name: "Code Quality",
			Icon: "🎯",
			Types: []string{"HIGH_COMPLEXITY", "LONG_FUNCTION", "TOO_MANY_PARAMS", "DUPLICATE_CODE",
				"UNUSED_PARAM", "TODO_FIXME", "SINGLE_LETTER_VAR", "MAGIC_NUMBER"},
		},
		{
			Name: "Context & API",
			Icon: "⚡",
			Types: []string{"CONTEXT_BACKGROUND", "CONTEXT_VALUE", "MISSING_CONTEXT_CANCEL", "CONTEXT_LEAK",
				"CONTEXT_IN_STRUCT", "CONTEXT_NOT_FIRST", "SYNC_POOL_MISUSE", "CONTEXT_MISUSE", "WG_MISUSE"},
		},
		{
			Name:  "Optimization Opportunities",
			Icon:  "💡",
			Types: []string{"SYNCPOOL_OPPORTUNITY", "SYNCPOOL_PUT_MISSING", "SYNCPOOL_TYPE_ASSERT"},
		},
		{
			Name: "Test Coverage",
			Icon: "🧪",
			Types: []string{"MISSING_TEST", "MISSING_EXAMPLE", "MISSING_BENCHMARK", "UNTESTED_EXPORT",
				"UNTESTED_TYPE", "UNTESTED_ERROR", "UNTESTED_CONCURRENCY", "UNTESTED_IO_FUNCTION"},
		},
		{
			Name: "Privacy & Security",
			Icon: "🔒",
			Types: []string{"PRIVACY_HARDCODED_SECRET", "PRIVACY_AWS_KEY", "PRIVACY_JWT_TOKEN",
				"PRIVACY_EMAIL_PII", "PRIVACY_SSN_PII", "PRIVACY_CREDIT_CARD_PII",
				"PRIVACY_LOGGING_SENSITIVE", "PRIVACY_PRINTING_SENSITIVE", "PRIVACY_EXPOSED_FIELD",
				"PRIVACY_UNENCRYPTED_DB_WRITE", "PRIVACY_DIRECT_INPUT_TO_DB"},
		},
		{
			Name: "Dependencies",
			Icon: "📦",
			Types: []string{"DEPENDENCY_DEPRECATED", "DEPENDENCY_VULNERABLE", "DEPENDENCY_OUTDATED",
				"DEPENDENCY_CGO", "DEPENDENCY_UNSAFE", "DEPENDENCY_INTERNAL", "DEPENDENCY_INDIRECT",
				"DEPENDENCY_LOCAL_REPLACE", "DEPENDENCY_NO_CHECKSUM", "DEPENDENCY_EMPTY_CHECKSUM",
				"DEPENDENCY_VERSION_CONFLICT"},
		},
		{
			Name:  "Other",
			Icon:  "📌",
			Types: []string{}, // Catches any uncategorized issues
		},
	}
}

func getAnalyzerGroup(issueType string) string {
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

func countBySeverity(issues []analyzer.Issue) (high, medium, low int) {
	for _, issue := range issues {
		switch issue.Severity {
		case analyzer.SeverityHigh:
			high++
		case analyzer.SeverityMedium:
			medium++
		case analyzer.SeverityLow:
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
		log.Fatalf("Failed to marshal config: %v", err)
	}

	const configFile = ".aibscleaner.yaml"
	const configFileMode = 0644
	err = os.WriteFile(configFile, yamlData, configFileMode)
	if err != nil {
		log.Fatalf("Failed to write config file: %v", err)
	}

	fmt.Printf("✅ Created default configuration file: %s\n", configFile)
	fmt.Println("📝 Edit this file to customize your analysis settings")
	fmt.Println("")
	fmt.Println("Example usage:")
	fmt.Println("  aibscleaner --config=.aibscleaner.yaml .")
}
