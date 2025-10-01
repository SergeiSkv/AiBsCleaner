package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/SergeiSkv/AiBsCleaner/analyzer"
)

// Config represents the configuration for the analyzer
type Config struct {
	// Analyzer configuration
	Analyzers struct {
		Loop                AnalyzerConfig `yaml:"loop" json:"loop"`
		DeferOptimization   AnalyzerConfig `yaml:"defer_optimization" json:"defer_optimization"`
		Slice               AnalyzerConfig `yaml:"slice" json:"slice"`
		Map                 AnalyzerConfig `yaml:"map" json:"map"`
		Reflection          AnalyzerConfig `yaml:"reflection" json:"reflection"`
		Goroutine           AnalyzerConfig `yaml:"goroutine" json:"goroutine"`
		Interface           AnalyzerConfig `yaml:"interface" json:"interface"`
		Regex               AnalyzerConfig `yaml:"regex" json:"regex"`
		Time                AnalyzerConfig `yaml:"time" json:"time"`
		MemoryLeak          AnalyzerConfig `yaml:"memory_leak" json:"memory_leak"`
		Database            AnalyzerConfig `yaml:"database" json:"database"`
		NilPtr              AnalyzerConfig `yaml:"nil_ptr" json:"nil_ptr"`
		APIMisuse           AnalyzerConfig `yaml:"api_misuse" json:"api_misuse"`
		AIBullshit          AnalyzerConfig `yaml:"ai_bullshit" json:"ai_bullshit"`
		Channel             AnalyzerConfig `yaml:"channel" json:"channel"`
		HTTPClient          AnalyzerConfig `yaml:"http_client" json:"http_client"`
		Privacy             AnalyzerConfig `yaml:"privacy" json:"privacy"`
		Context             AnalyzerConfig `yaml:"context" json:"context"`
		RaceCondition       AnalyzerConfig `yaml:"race_condition" json:"race_condition"`
		ErrorHandling       AnalyzerConfig `yaml:"error_handling" json:"error_handling"`
		GCPressure          AnalyzerConfig `yaml:"gc_pressure" json:"gc_pressure"`
		ConcurrencyPatterns AnalyzerConfig `yaml:"concurrency_patterns" json:"concurrency_patterns"`
		CPUOptimization     AnalyzerConfig `yaml:"cpu_optimization" json:"cpu_optimization"`
		NetworkPatterns     AnalyzerConfig `yaml:"network_patterns" json:"network_patterns"`
		SyncPool            AnalyzerConfig `yaml:"sync_pool" json:"sync_pool"`
		TestCoverage        AnalyzerConfig `yaml:"test_coverage" json:"test_coverage"`
		Crypto              AnalyzerConfig `yaml:"crypto" json:"crypto"`
		Serialization       AnalyzerConfig `yaml:"serialization" json:"serialization"`
		IOBuffer            AnalyzerConfig `yaml:"io_buffer" json:"io_buffer"`
		HTTPReuse           AnalyzerConfig `yaml:"http_reuse" json:"http_reuse"`
		CGO                 AnalyzerConfig `yaml:"cgo" json:"cgo"`
		String              AnalyzerConfig `yaml:"string" json:"string"`
		Dependency          AnalyzerConfig `yaml:"dependency" json:"dependency"`
		StructLayout        AnalyzerConfig `yaml:"struct_layout" json:"struct_layout"`
		CPUCache            AnalyzerConfig `yaml:"cpu_cache" json:"cpu_cache"`
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

	// Enable performance-focused analyzers by default
	config.Analyzers.Loop.Enabled = true
	config.Analyzers.DeferOptimization.Enabled = true
	config.Analyzers.Slice.Enabled = true
	config.Analyzers.Map.Enabled = true
	config.Analyzers.Reflection.Enabled = true
	config.Analyzers.Goroutine.Enabled = true
	config.Analyzers.Interface.Enabled = true
	config.Analyzers.Regex.Enabled = true
	config.Analyzers.Time.Enabled = true
	config.Analyzers.MemoryLeak.Enabled = true
	config.Analyzers.Database.Enabled = true
	config.Analyzers.NilPtr.Enabled = true
	config.Analyzers.APIMisuse.Enabled = true
	config.Analyzers.AIBullshit.Enabled = true
	config.Analyzers.Channel.Enabled = true
	config.Analyzers.HTTPClient.Enabled = true
	config.Analyzers.Privacy.Enabled = true
	config.Analyzers.Context.Enabled = true
	config.Analyzers.RaceCondition.Enabled = true
	config.Analyzers.ErrorHandling.Enabled = true
	config.Analyzers.GCPressure.Enabled = true
	config.Analyzers.ConcurrencyPatterns.Enabled = true
	config.Analyzers.CPUOptimization.Enabled = true
	config.Analyzers.NetworkPatterns.Enabled = true
	config.Analyzers.SyncPool.Enabled = true
	config.Analyzers.TestCoverage.Enabled = false // Usually noisy
	config.Analyzers.Crypto.Enabled = true
	config.Analyzers.Serialization.Enabled = true
	config.Analyzers.IOBuffer.Enabled = true
	config.Analyzers.HTTPReuse.Enabled = true
	config.Analyzers.CGO.Enabled = false // Disabled by default
	config.Analyzers.String.Enabled = true
	config.Analyzers.Dependency.Enabled = true
	config.Analyzers.StructLayout.Enabled = true
	config.Analyzers.CPUCache.Enabled = true

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

// findConfigPath searches for config file in common locations
func findConfigPath() string {
	locations := []string{
		".aibscleaner.yaml",
		".aibscleaner.yml",
		".aibscleaner.json",
		"aibscleaner.yaml",
		"aibscleaner.yml",
		"aibscleaner.json",
	}

	// Check the current directory
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	// Check home directory
	home, _ := os.UserHomeDir()
	if home == "" {
		return ""
	}

	for _, loc := range locations {
		configPath := filepath.Join(home, ".config", "aibscleaner", loc)
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}
	return ""
}

// LoadConfig loads configuration from a file or returns default
func LoadConfig(path string) (*Config, error) {
	// If no path specified, try to find config in common locations
	if path == "" {
		path = findConfigPath()
	}

	// If still no config found, return default
	if path == "" {
		return DefaultConfig(), nil
	}

	// Read a config file
	data, err := os.ReadFile(path)
	if err != nil {
		// If a config file doesn't exist, return default
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

// loadIgnoreFile loads patterns from an ignored file like .gitignore
func loadIgnoreFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	patterns := make([]string, 0, len(lines))

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

// getAnalyzerEnabled returns if a specific analyzer is enabled
// Currently unused but kept for future API
func (c *Config) getAnalyzerEnabled(analyzerType analyzer.AnalyzerType) bool {
	analyzerConfigs := map[analyzer.AnalyzerType]*AnalyzerConfig{
		analyzer.AnalyzerLoop:                &c.Analyzers.Loop,
		analyzer.AnalyzerDeferOptimization:   &c.Analyzers.DeferOptimization,
		analyzer.AnalyzerSlice:               &c.Analyzers.Slice,
		analyzer.AnalyzerMap:                 &c.Analyzers.Map,
		analyzer.AnalyzerReflection:          &c.Analyzers.Reflection,
		analyzer.AnalyzerGoroutine:           &c.Analyzers.Goroutine,
		analyzer.AnalyzerInterface:           &c.Analyzers.Interface,
		analyzer.AnalyzerRegex:               &c.Analyzers.Regex,
		analyzer.AnalyzerTime:                &c.Analyzers.Time,
		analyzer.AnalyzerMemoryLeak:          &c.Analyzers.MemoryLeak,
		analyzer.AnalyzerDatabase:            &c.Analyzers.Database,
		analyzer.AnalyzerNilPtr:              &c.Analyzers.NilPtr,
		analyzer.AnalyzerAPIMisuse:           &c.Analyzers.APIMisuse,
		analyzer.AnalyzerAIBullshit:          &c.Analyzers.AIBullshit,
		analyzer.AnalyzerChannel:             &c.Analyzers.Channel,
		analyzer.AnalyzerHTTPClient:          &c.Analyzers.HTTPClient,
		analyzer.AnalyzerPrivacy:             &c.Analyzers.Privacy,
		analyzer.AnalyzerContext:             &c.Analyzers.Context,
		analyzer.AnalyzerRaceCondition:       &c.Analyzers.RaceCondition,
		analyzer.AnalyzerErrorHandling:       &c.Analyzers.ErrorHandling,
		analyzer.AnalyzerGCPressure:          &c.Analyzers.GCPressure,
		analyzer.AnalyzerConcurrencyPatterns: &c.Analyzers.ConcurrencyPatterns,
		analyzer.AnalyzerCPUOptimization:     &c.Analyzers.CPUOptimization,
		analyzer.AnalyzerNetworkPatterns:     &c.Analyzers.NetworkPatterns,
		analyzer.AnalyzerSyncPool:            &c.Analyzers.SyncPool,
		analyzer.AnalyzerTestCoverage:        &c.Analyzers.TestCoverage,
		analyzer.AnalyzerCrypto:              &c.Analyzers.Crypto,
		analyzer.AnalyzerSerialization:       &c.Analyzers.Serialization,
		analyzer.AnalyzerIOBuffer:            &c.Analyzers.IOBuffer,
		analyzer.AnalyzerHTTPReuse:           &c.Analyzers.HTTPReuse,
		analyzer.AnalyzerCGO:                 &c.Analyzers.CGO,
		analyzer.AnalyzerString:              &c.Analyzers.String,
		analyzer.AnalyzerDependency:          &c.Analyzers.Dependency,
	}

	if analyzerType == analyzer.AnalyzerTypeMax {
		return false // Sentinel value
	}

	if cfg, ok := analyzerConfigs[analyzerType]; ok {
		return cfg.Enabled
	}
	return true // Enable by default
}

// GetAnalyzerConfig returns config for a specific analyzer
func (c *Config) GetAnalyzerConfig(analyzerName string) AnalyzerConfig {
	analyzerConfigMap := map[string]AnalyzerConfig{
		"loop":                c.Analyzers.Loop,
		"deferoptimization":   c.Analyzers.DeferOptimization,
		"slice":               c.Analyzers.Slice,
		"map":                 c.Analyzers.Map,
		"reflection":          c.Analyzers.Reflection,
		"goroutine":           c.Analyzers.Goroutine,
		"interface":           c.Analyzers.Interface,
		"regex":               c.Analyzers.Regex,
		"time":                c.Analyzers.Time,
		"memoryleak":          c.Analyzers.MemoryLeak,
		"database":            c.Analyzers.Database,
		"nilptr":              c.Analyzers.NilPtr,
		"apimisuse":           c.Analyzers.APIMisuse,
		"aibullshit":          c.Analyzers.AIBullshit,
		"channel":             c.Analyzers.Channel,
		"httpclient":          c.Analyzers.HTTPClient,
		"privacy":             c.Analyzers.Privacy,
		"context":             c.Analyzers.Context,
		"racecondition":       c.Analyzers.RaceCondition,
		"errorhandling":       c.Analyzers.ErrorHandling,
		"gcpressure":          c.Analyzers.GCPressure,
		"concurrencypatterns": c.Analyzers.ConcurrencyPatterns,
		"cpuoptimization":     c.Analyzers.CPUOptimization,
		"networkpatterns":     c.Analyzers.NetworkPatterns,
		"syncpool":            c.Analyzers.SyncPool,
		"testcoverage":        c.Analyzers.TestCoverage,
		"crypto":              c.Analyzers.Crypto,
		"serialization":       c.Analyzers.Serialization,
		"iobuffer":            c.Analyzers.IOBuffer,
		"httpreuse":           c.Analyzers.HTTPReuse,
		"cgo":                 c.Analyzers.CGO,
		"string":              c.Analyzers.String,
		"dependency":          c.Analyzers.Dependency,
		"structlayout":        c.Analyzers.StructLayout,
		"cpucache":            c.Analyzers.CPUCache,
	}

	if cfg, ok := analyzerConfigMap[strings.ToLower(analyzerName)]; ok {
		return cfg
	}
	return AnalyzerConfig{Enabled: true}
}
