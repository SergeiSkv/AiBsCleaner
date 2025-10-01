package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

const formatText = "text"

func TestDefaultConfigEnablesCoreAnalyzers(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Analyzers.Loop.Enabled {
		t.Fatalf("loop analyzer should be enabled by default")
	}
	if cfg.Analyzers.TestCoverage.Enabled {
		t.Fatalf("test coverage analyzer should be disabled by default")
	}
	if cfg.Output.Format != formatText {
		t.Fatalf("unexpected output format: %s", cfg.Output.Format)
	}
}

func TestGetAnalyzerConfigIsCaseInsensitive(t *testing.T) {
	cfg := &Config{}
	cfg.Analyzers.Loop.Enabled = true
	cfg.Analyzers.Map.Enabled = false

	loop := cfg.GetAnalyzerConfig("LoOp")
	if !loop.Enabled {
		t.Fatalf("expected loop analyzer to be enabled")
	}

	unknown := cfg.GetAnalyzerConfig("nonexistent")
	if !unknown.Enabled {
		t.Fatalf("unknown analyzers default to enabled")
	}
}

func TestParseIgnoreLines(t *testing.T) {
	lines := []string{"# comment", "vendor/", "**/generated", "", "node_modules/*"}
	patterns := parseIgnoreLines(lines)

	want := []string{"vendor", "generated", "node_modules/"}
	if len(patterns) != len(want) {
		t.Fatalf("expected %d patterns, got %d", len(want), len(patterns))
	}
	for i, pattern := range patterns {
		if pattern != want[i] {
			t.Fatalf("expected pattern %q at index %d, got %q", want[i], i, pattern)
		}
	}
}

func TestLoadConfigReturnsDefaultWhenMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.yaml")
	cfg, err := LoadConfig(missing)
	if err != nil {
		t.Fatalf("expected default config without error, got %v", err)
	}
	if cfg == nil || !cfg.Analyzers.Loop.Enabled {
		t.Fatalf("expected default configuration when file is missing")
	}
}

func TestLoadConfigParsesYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	payload := []byte("analyzers:\n  loop:\n    enabled: false\noutput:\n  format: json\n")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("failed to write config fixture: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}
	if cfg.Analyzers.Loop.Enabled {
		t.Fatalf("loop analyzer should be disabled by YAML override")
	}
	if cfg.Output.Format != "json" {
		t.Fatalf("expected json format, got %s", cfg.Output.Format)
	}
}

func TestLoadIgnoreFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".abcignore")
	if err := os.WriteFile(path, []byte("vendor\n#comment\n*.tmp\n"), 0o644); err != nil {
		t.Fatalf("failed to write ignore file: %v", err)
	}

	patterns, err := loadIgnoreFile(path)
	if err != nil {
		t.Fatalf("unexpected error loading ignore file: %v", err)
	}
	if len(patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(patterns))
	}
	if patterns[0] != "vendor" || patterns[1] != "*.tmp" {
		t.Fatalf("unexpected patterns: %v", patterns)
	}
}
