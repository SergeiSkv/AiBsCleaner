package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
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
	config.Analyzers.CGO.Enabled = true
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

// findConfigPath searches for a config file in common locations
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
	resolvedPath := resolveConfigPath(path)
	if resolvedPath == "" {
		return DefaultConfig(), nil
	}

	file, err := os.Open(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	defer func() { _ = file.Close() }()

	config, err := decodeConfigFile(file, resolvedPath)
	if err != nil {
		return nil, err
	}

	mergeIgnorePatterns(config, ".abcignore")
	return config, nil
}

func resolveConfigPath(path string) string {
	if path != "" {
		return path
	}
	return findConfigPath()
}

func decodeConfigFile(r io.ReadSeeker, path string) (*Config, error) {
	config := &Config{}
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".json":
		if err := json.NewDecoder(r).Decode(config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("failed to read YAML config: %w", err)
		}
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	default:
		if err := tryJSONThenYAML(r, config); err != nil {
			return nil, err
		}
	}

	return config, nil
}

func tryJSONThenYAML(r io.ReadSeeker, config *Config) error {
	if err := json.NewDecoder(r).Decode(config); err == nil {
		return nil
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to reset file position: %w", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read config for YAML parsing: %w", err)
	}
	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse config (tried JSON and YAML): %w", err)
	}
	return nil
}

func mergeIgnorePatterns(cfg *Config, ignorePath string) {
	patterns, err := loadIgnoreFile(ignorePath)
	if err != nil {
		return
	}
	cfg.Paths.Exclude = append(cfg.Paths.Exclude, patterns...)
}

// loadIgnoreFile loads patterns from an ignored file like .gitignore
func loadIgnoreFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	lines, err := readLines(file)
	if err != nil {
		return nil, err
	}

	patterns := parseIgnoreLines(lines)
	return patterns, nil
}

func readLines(r io.Reader) ([]string, error) {
	const maxLineSize = 1024 * 1024
	scanner := bufio.NewScanner(r)
	buf := make([]byte, maxLineSize)
	scanner.Buffer(buf, maxLineSize)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func parseIgnoreLines(lines []string) []string {
	patterns := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimSuffix(line, "/")
		line = strings.TrimSuffix(line, "*")
		line = strings.TrimPrefix(line, "**/")

		patterns = append(patterns, line)
	}

	return patterns
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
