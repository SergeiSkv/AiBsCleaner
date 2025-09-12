package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SergeiSkv/AiBsCleaner/cache"
	"github.com/SergeiSkv/AiBsCleaner/models"
)

const sampleGoFile = "sample.go"

type stubFileInfo struct {
	name string
	size int64
	mode os.FileMode
	mod  time.Time
	dir  bool
}

func (s stubFileInfo) Name() string       { return s.name }
func (s stubFileInfo) Size() int64        { return s.size }
func (s stubFileInfo) Mode() os.FileMode  { return s.mode }
func (s stubFileInfo) ModTime() time.Time { return s.mod }
func (s stubFileInfo) IsDir() bool        { return s.dir }
func (s stubFileInfo) Sys() any           { return nil }

func TestReconstructIssuesFromCacheFiltersIgnored(t *testing.T) {
	record := &cache.FileRecord{
		Issues: []*models.Issue{
			{ID: "keep", Line: 10},
			{ID: "ignore-me", Line: 20},
		},
		Ignored: []string{"ignore-me"},
	}

	issues := reconstructIssuesFromCache("sample.go", record)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].File != sampleGoFile || issues[0].Line != 10 {
		t.Fatalf("unexpected issue payload: %+v", issues[0])
	}
}

func TestConvertDBIssueToAnalyzerIssueDefaultsSeverity(t *testing.T) {
	issue := convertDBIssueToAnalyzerIssue(sampleGoFile, &models.Issue{
		Line:       7,
		Column:     3,
		Message:    "problem",
		Suggestion: "fix",
		CanBeFixed: true,
	})

	if issue.Severity != models.SeverityLevelMedium {
		t.Fatalf("expected medium severity, got %v", issue.Severity)
	}
	if issue.Position.Filename != sampleGoFile || issue.Position.Line != 7 {
		t.Fatalf("unexpected position: %+v", issue.Position)
	}
}

func TestBuildEnabledAnalyzersUsesConfig(t *testing.T) {
	cfg := &Config{}
	cfg.Analyzers.Loop.Enabled = true
	cfg.Analyzers.Map.Enabled = false

	enabled := buildEnabledAnalyzers(cfg)
	if enabled == nil {
		t.Fatalf("expected map of enabled analyzers")
	}

	if !enabled["loop"] {
		t.Fatalf("loop analyzer should be enabled")
	}
	if enabled["map"] {
		t.Fatalf("map analyzer should not be enabled")
	}
}

func TestBuildEnabledAnalyzersReturnsNilWhenNoneEnabled(t *testing.T) {
	cfg := &Config{}
	if result := buildEnabledAnalyzers(cfg); result != nil {
		t.Fatalf("expected nil when no analyzers enabled, got %v", result)
	}
}

func TestShouldSkipPath(t *testing.T) {
	excludes := []string{"vendor", "_test.go"}

	skip, skipDir := shouldSkipPath(filepath.Join("project", "vendor"), stubFileInfo{dir: true}, excludes)
	if skip || !skipDir {
		t.Fatalf("expected to skip directory traversal for vendor")
	}

	skip, skipDir = shouldSkipPath(filepath.Join("project", "handler_test.go"), stubFileInfo{name: "handler_test.go"}, excludes)
	if !skip || skipDir {
		t.Fatalf("expected to skip test file")
	}

	skip, skipDir = shouldSkipPath(filepath.Join("project", "main.go"), stubFileInfo{name: "main.go"}, excludes)
	if skip || skipDir {
		t.Fatalf("expected to keep regular file")
	}
}

func TestIsGoFile(t *testing.T) {
	if !isGoFile("main.go", stubFileInfo{name: "main.go"}) {
		t.Fatalf("expected .go file to be detected")
	}
	if isGoFile("data.txt", stubFileInfo{name: "data.txt"}) {
		t.Fatalf("expected non-go file to be skipped")
	}
}

func TestCountLinesHandlesMissingNewline(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "sample.go")
	if err := os.WriteFile(filePath, []byte("line1\nline2"), 0o644); err != nil {
		t.Fatalf("failed writing temp file: %v", err)
	}

	if got := countLines(filePath); got != 2 {
		t.Fatalf("expected 2 lines, got %d", got)
	}
}
