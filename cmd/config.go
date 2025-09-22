package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the configuration for the analyzer
type Config struct {
	// Analyzers configuration
	Analyzers struct {
		Loop                AnalyzerConfig `yaml:"loop" json:"loop"`
		StringConcat        AnalyzerConfig `yaml:"string_concat" json:"string_concat"`
		Defer               AnalyzerConfig `yaml:"defer" json:"defer"`
		DeferOptimization   AnalyzerConfig `yaml:"defer_optimization" json:"defer_optimization"`
		Slice               AnalyzerConfig `yaml:"slice" json:"slice"`
		Map                 AnalyzerConfig `yaml:"map" json:"map"`
		Reflection          AnalyzerConfig `yaml:"reflection" json:"reflection"`
		Goroutine           AnalyzerConfig `yaml:"goroutine" json:"goroutine"`
		Interface           AnalyzerConfig `yaml:"interface" json:"interface"`
		Regex               AnalyzerConfig `yaml:"regex" json:"regex"`
		Time                AnalyzerConfig `yaml:"time" json:"time"`
		Complexity          AnalyzerConfig `yaml:"complexity" json:"complexity"`
		MemoryLeak          AnalyzerConfig `yaml:"memory_leak" json:"memory_leak"`
		Database            AnalyzerConfig `yaml:"database" json:"database"`
		NilPtr              AnalyzerConfig `yaml:"nil_ptr" json:"nil_ptr"`
		CodeSmell           AnalyzerConfig `yaml:"code_smell" json:"code_smell"`
		APIMisuse           AnalyzerConfig `yaml:"api_misuse" json:"api_misuse"`
		AIBullshit          AnalyzerConfig `yaml:"ai_bullshit" json:"ai_bullshit"`
		Context             AnalyzerConfig `yaml:"context" json:"context"`
		Channel             AnalyzerConfig `yaml:"channel" json:"channel"`
		RaceCondition       AnalyzerConfig `yaml:"race_condition" json:"race_condition"`
		ErrorHandling       AnalyzerConfig `yaml:"error_handling" json:"error_handling"`
		HTTPClient          AnalyzerConfig `yaml:"http_client" json:"http_client"`
		GCPressure          AnalyzerConfig `yaml:"gc_pressure" json:"gc_pressure"`
		ConcurrencyPatterns AnalyzerConfig `yaml:"concurrency_patterns" json:"concurrency_patterns"`
		CPUOptimization     AnalyzerConfig `yaml:"cpu_optimization" json:"cpu_optimization"`
		NetworkPatterns     AnalyzerConfig `yaml:"network_patterns" json:"network_patterns"`
		SyncPool            AnalyzerConfig `yaml:"sync_pool" json:"sync_pool"`
		TestCoverage        AnalyzerConfig `yaml:"test_coverage" json:"test_coverage"`
	} `yaml:"analyzers" json:"analyzers"`

	// Thresholds for various checks
	Thresholds struct {
		MaxLoopDepth      int `yaml:"max_loop_depth" json:"max_loop_depth"`
		MaxComplexity     int `yaml:"max_complexity" json:"max_complexity"`
		MaxFunctionLength int `yaml:"max_function_length" json:"max_function_length"`
		MaxParameters     int `yaml:"max_parameters" json:"max_parameters"`
		MaxReturnValues   int `yaml:"max_return_values" json:"max_return_values"`
	} `yaml:"thresholds" json:"thresholds"`

	// Path configuration
	Paths struct {
		Exclude []string `yaml:"exclude" json:"exclude"` // Paths to exclude from analysis
		Include []string `yaml:"include" json:"include"` // Specific paths to include (if empty, all non-excluded paths)
	} `yaml:"paths" json:"paths"`

	// Output configuration
	Output struct {
		Format      string `yaml:"format" json:"format"`             // "text" or "json"
		ShowContext bool   `yaml:"show_context" json:"show_context"` // Show code context
		MaxIssues   int    `yaml:"max_issues" json:"max_issues"`     // Maximum issues to report (0 = unlimited)
	} `yaml:"output" json:"output"`
}

// AnalyzerConfig represents configuration for a single analyzer
type AnalyzerConfig struct {
	Enabled  bool     `yaml:"enabled" json:"enabled"`
	Severity string   `yaml:"severity,omitempty" json:"severity,omitempty"` // Override default severity
	Exclude  []string `yaml:"exclude,omitempty" json:"exclude,omitempty"`   // Exclude patterns
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	config := &Config{}

	// Enable most analyzers by default
	config.Analyzers.Loop.Enabled = true
	config.Analyzers.StringConcat.Enabled = true
	config.Analyzers.Defer.Enabled = true
	config.Analyzers.DeferOptimization.Enabled = true
	config.Analyzers.Slice.Enabled = true
	config.Analyzers.Map.Enabled = true
	config.Analyzers.Reflection.Enabled = true
	config.Analyzers.Goroutine.Enabled = true
	config.Analyzers.Interface.Enabled = true
	config.Analyzers.Regex.Enabled = true
	config.Analyzers.Time.Enabled = true
	config.Analyzers.Complexity.Enabled = true
	config.Analyzers.MemoryLeak.Enabled = true
	config.Analyzers.Database.Enabled = true
	config.Analyzers.NilPtr.Enabled = false // Too many false positives, needs rewrite
	config.Analyzers.CodeSmell.Enabled = true
	config.Analyzers.APIMisuse.Enabled = true
	config.Analyzers.AIBullshit.Enabled = true // ENABLED! This is what AiBsCleaner is about
	config.Analyzers.Context.Enabled = true
	config.Analyzers.Channel.Enabled = true
	config.Analyzers.RaceCondition.Enabled = false // Use `go test -race` instead
	config.Analyzers.ErrorHandling.Enabled = false // Too many false positives
	config.Analyzers.HTTPClient.Enabled = true
	config.Analyzers.GCPressure.Enabled = true
	config.Analyzers.ConcurrencyPatterns.Enabled = true
	config.Analyzers.CPUOptimization.Enabled = false // Too aggressive for CLI tools
	config.Analyzers.NetworkPatterns.Enabled = true
	config.Analyzers.SyncPool.Enabled = true
	config.Analyzers.TestCoverage.Enabled = false // Disabled by default - noisy

	// Set default thresholds
	config.Thresholds.MaxLoopDepth = 3
	config.Thresholds.MaxComplexity = 10
	config.Thresholds.MaxFunctionLength = 50
	config.Thresholds.MaxParameters = 5
	config.Thresholds.MaxReturnValues = 3

	// Set default paths to exclude
	config.Paths.Exclude = []string{
		"examples",
		"vendor",
		".git",
		"node_modules",
		"testdata",
		"test_data",
		"mocks",
		"_test.go",
	}

	// Set default output
	config.Output.Format = "text"
	config.Output.ShowContext = false
	config.Output.MaxIssues = 0

	return config
}

// LoadConfig loads configuration from file or returns default
func LoadConfig(path string) (*Config, error) {
	// If no path specified, try to find config in common locations
	if path == "" {
		// Try common config locations
		locations := []string{
			".aibscleaner.yaml",
			".aibscleaner.yml",
			".aibscleaner.json",
			"aibscleaner.yaml",
			"aibscleaner.yml",
			"aibscleaner.json",
		}

		// Check current directory
		for _, loc := range locations {
			if _, err := os.Stat(loc); err == nil {
				path = loc
				break
			}
		}

		// Check home directory
		if path == "" {
			home, _ := os.UserHomeDir()
			if home != "" {
				for _, loc := range locations {
					configPath := filepath.Join(home, ".config", "aibscleaner", loc)
					if _, err := os.Stat(configPath); err == nil {
						path = configPath
						break
					}
				}
			}
		}
	}

	// If still no config found, return default
	if path == "" {
		return DefaultConfig(), nil
	}

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		// If config file doesn't exist, return default
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse config based on extension
	config := &Config{}
	ext := filepath.Ext(path)

	switch ext {
	case ".json":
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	default:
		// Try YAML first, then JSON
		if err := yaml.Unmarshal(data, config); err != nil {
			if err := json.Unmarshal(data, config); err != nil {
				return nil, fmt.Errorf("failed to parse config (tried YAML and JSON): %w", err)
			}
		}
	}

	// Load .abcignore if exists
	if ignorePatterns, err := loadIgnoreFile(".abcignore"); err == nil {
		config.Paths.Exclude = append(config.Paths.Exclude, ignorePatterns...)
	}

	return config, nil
}

// loadIgnoreFile loads patterns from an ignore file like .gitignore
func loadIgnoreFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var patterns []string
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Convert glob patterns to simple excludes
		// Remove trailing slashes and wildcards for simplicity
		line = strings.TrimSuffix(line, "/")
		line = strings.TrimSuffix(line, "*")
		line = strings.TrimPrefix(line, "**/")

		patterns = append(patterns, line)
	}

	return patterns, nil
}

// ShouldAnalyze checks if an issue type should be analyzed based on config
func (c *Config) ShouldAnalyze(issueType string) bool {
	// Map issue types to analyzer configs
	switch issueType {
	// Loop analyzer
	case "ALLOC_IN_LOOP", "NESTED_LOOP", "STRING_CONCAT_IN_LOOP", "APPEND_IN_LOOP":
		return c.Analyzers.Loop.Enabled

	// String concat analyzer
	case "STRING_CONCAT", "STRING_BUILDER":
		return c.Analyzers.StringConcat.Enabled

	// Defer analyzer
	case "DEFER_IN_LOOP", "DEFER_IN_SHORT_FUNC", "DEFER_OVERHEAD":
		return c.Analyzers.Defer.Enabled

	// Defer optimization analyzer
	case "UNNECESSARY_DEFER", "DEFER_AT_END", "MULTIPLE_DEFERS", "DEFER_IN_HOT_PATH",
		"DEFER_LARGE_CAPTURE", "UNNECESSARY_MUTEX_DEFER", "MISSING_DEFER_UNLOCK",
		"MISSING_DEFER_CLOSE":
		return c.Analyzers.DeferOptimization.Enabled

	// Slice analyzer
	case "SLICE_CAPACITY", "SLICE_COPY", "SLICE_APPEND", "SLICE_RANGE_COPY":
		return c.Analyzers.Slice.Enabled

	// Map analyzer
	case "MAP_CAPACITY", "MAP_CLEAR", "MAP_WITHOUT_SIZE_HINT":
		return c.Analyzers.Map.Enabled

	// Reflection analyzer
	case "REFLECTION", "REFLECTION_IN_LOOP":
		return c.Analyzers.Reflection.Enabled

	// Goroutine analyzer
	case "GOROUTINE_LEAK", "UNBUFFERED_CHANNEL", "GOROUTINE_OVERHEAD",
		"UNBUFFERED_CHANNEL_IN_GOROUTINE", "GOROUTINE_WITHOUT_RECOVER":
		return c.Analyzers.Goroutine.Enabled

	// Interface analyzer
	case "INTERFACE_ALLOCATION", "EMPTY_INTERFACE", "INTERFACE_POLLUTION":
		return c.Analyzers.Interface.Enabled

	// Regex analyzer
	case "REGEX_IN_LOOP", "REGEX_COMPILE":
		return c.Analyzers.Regex.Enabled

	// Time analyzer
	case "TIME_AFTER_LEAK", "TIME_FORMAT", "TIME_IN_LOOP":
		return c.Analyzers.Time.Enabled

	// Complexity analyzer
	case "HIGH_COMPLEXITY":
		return c.Analyzers.Complexity.Enabled

	// Memory leak analyzer
	case "MEMORY_LEAK", "GLOBAL_VAR", "LARGE_ALLOCATION":
		return c.Analyzers.MemoryLeak.Enabled

	// Database analyzer
	case "SQL_IN_LOOP", "NO_PREPARED_STMT", "MISSING_DB_CLOSE":
		return c.Analyzers.Database.Enabled

	// NilPtr analyzer
	case "NIL_CHECK", "PANIC_RISK", "NIL_RETURN",
		"POTENTIAL_NIL_DEREF", "POTENTIAL_NIL_INDEX", "RANGE_OVER_NIL", "NIL_METHOD_CALL",
		"UNCHECKED_PARAM":
		return c.Analyzers.NilPtr.Enabled

	// Code smell analyzer
	case "LONG_FUNCTION", "TOO_MANY_PARAMS", "DUPLICATE_CODE", "UNUSED_PARAM", "TODO_FIXME",
		"SINGLE_LETTER_VAR", "ARROW_ANTIPATTERN", "HARDCODED_CONFIG",
		"CONSOLE_LOG_DEBUGGING", "UNTESTED_COMPLEX_FUNCTION", "PANIC_IN_LIBRARY":
		return c.Analyzers.CodeSmell.Enabled

	// API misuse analyzer
	case "SYNC_POOL_MISUSE", "CONTEXT_MISUSE", "WG_MISUSE":
		return c.Analyzers.APIMisuse.Enabled

	// AI bullshit detector (specific patterns - enabled by default)
	case "AI_BULLSHIT_CONCURRENCY", "AI_REFLECTION_OVERKILL", "AI_PATTERN_ABUSE",
		"AI_ENTERPRISE_HELLO_WORLD", "AI_CAPTAIN_OBVIOUS", "AI_OVERENGINEERED_SIMPLE":
		return c.Analyzers.AIBullshit.Enabled

	// Context analyzer
	case "CONTEXT_BACKGROUND", "CONTEXT_VALUE", "MISSING_CONTEXT_CANCEL", "CONTEXT_LEAK",
		"CONTEXT_IN_STRUCT", "CONTEXT_NOT_FIRST":
		return c.Analyzers.Context.Enabled

	// Channel analyzer
	case "UNBUFFERED_SIGNAL_CHAN", "SELECT_DEFAULT", "CHANNEL_SIZE", "RANGE_OVER_CHANNEL":
		return c.Analyzers.Channel.Enabled

	// Race condition analyzer
	case "RACE_CONDITION", "RACE_CONDITION_GLOBAL", "UNSYNC_MAP_ACCESS", "RACE_CLOSURE":
		return c.Analyzers.RaceCondition.Enabled

	// Error handling analyzer
	case "ERROR_IGNORED", "ERROR_CHECK_MISSING", "PANIC_RECOVER", "ERROR_STRING_FORMAT":
		return c.Analyzers.ErrorHandling.Enabled

	// HTTP client analyzer
	case "HTTP_NO_TIMEOUT", "HTTP_NO_CLOSE", "HTTP_DEFAULT_CLIENT", "HTTP_NO_CONTEXT":
		return c.Analyzers.HTTPClient.Enabled

	// GC pressure analyzer
	case "HIGH_GC_PRESSURE", "FREQUENT_ALLOCATION", "LARGE_HEAP_ALLOC", "POINTER_HEAVY_STRUCT":
		return c.Analyzers.GCPressure.Enabled

	// Concurrency patterns analyzer
	case "SYNC_MUTEX_VALUE", "WAITGROUP_MISUSE", "RACE_IN_DEFER", "ATOMIC_MISUSE",
		"GOROUTINE_PER_REQUEST", "NO_WORKER_POOL":
		return c.Analyzers.ConcurrencyPatterns.Enabled

	// CPU optimization analyzer
	case "CPU_INTENSIVE_LOOP", "UNNECESSARY_COPY", "BOUNDS_CHECK_ELIMINATION",
		"INEFFICIENT_ALGORITHM", "CACHE_UNFRIENDLY", "HIGH_COMPLEXITY_O2_EXPENSIVE",
		"PREVENTS_INLINING", "EXPENSIVE_OP_IN_HOT_PATH", "MODULO_POWER_OF_TWO":
		return c.Analyzers.CPUOptimization.Enabled

	// Network patterns analyzer
	case "KEEPALIVE_MISSING", "CONNECTION_POOL", "DNS_IN_LOOP", "NO_REUSE_CONNECTION":
		return c.Analyzers.NetworkPatterns.Enabled

	// SyncPool analyzer
	case "SYNCPOOL_OPPORTUNITY", "SYNCPOOL_PUT_MISSING", "SYNCPOOL_TYPE_ASSERT":
		return c.Analyzers.SyncPool.Enabled

	// Test coverage analyzer - usually noisy, disabled by default
	case "MISSING_TEST", "MISSING_EXAMPLE", "MISSING_BENCHMARK", "UNTESTED_EXPORT",
		"UNTESTED_TYPE", "UNTESTED_ERROR", "UNTESTED_CONCURRENCY", "UNTESTED_IO_FUNCTION":
		return c.Analyzers.TestCoverage.Enabled

	// Too noisy or not critical - always disabled
	case "MAGIC_NUMBER":
		return false // Too noisy and not critical for production code

	default:
		// Unknown issue type - allow by default
		return true
	}
}

// GetAnalyzerConfig returns config for a specific analyzer
func (c *Config) GetAnalyzerConfig(analyzerName string) AnalyzerConfig {
	switch strings.ToLower(analyzerName) {
	case "loop":
		return c.Analyzers.Loop
	case "stringconcat":
		return c.Analyzers.StringConcat
	case "defer":
		return c.Analyzers.Defer
	case "deferoptimization":
		return c.Analyzers.DeferOptimization
	case "slice":
		return c.Analyzers.Slice
	case "map":
		return c.Analyzers.Map
	case "reflection":
		return c.Analyzers.Reflection
	case "goroutine":
		return c.Analyzers.Goroutine
	case "interface":
		return c.Analyzers.Interface
	case "regex":
		return c.Analyzers.Regex
	case "time":
		return c.Analyzers.Time
	case "complexity":
		return c.Analyzers.Complexity
	case "memoryleak":
		return c.Analyzers.MemoryLeak
	case "database":
		return c.Analyzers.Database
	case "nilptr":
		return c.Analyzers.NilPtr
	case "codesmell":
		return c.Analyzers.CodeSmell
	case "apimisuse":
		return c.Analyzers.APIMisuse
	case "aibullshit":
		return c.Analyzers.AIBullshit
	case "context":
		return c.Analyzers.Context
	case "channel":
		return c.Analyzers.Channel
	case "racecondition":
		return c.Analyzers.RaceCondition
	case "errorhandling":
		return c.Analyzers.ErrorHandling
	case "httpclient":
		return c.Analyzers.HTTPClient
	case "gcpressure":
		return c.Analyzers.GCPressure
	case "concurrencypatterns":
		return c.Analyzers.ConcurrencyPatterns
	case "cpuoptimization":
		return c.Analyzers.CPUOptimization
	case "networkpatterns":
		return c.Analyzers.NetworkPatterns
	case "syncpool":
		return c.Analyzers.SyncPool
	case "testcoverage":
		return c.Analyzers.TestCoverage
	default:
		// Return enabled by default
		return AnalyzerConfig{Enabled: true}
	}
}
