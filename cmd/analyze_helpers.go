package cmd

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/SergeiSkv/AiBsCleaner/cache"
	"github.com/SergeiSkv/AiBsCleaner/models"
)

// loadCachedIssues attempts to load issues from cache
func loadCachedIssues(filename string, cacheDB *cache.FileCache) ([]*models.Issue, bool) {
	if cacheDB == nil || noCache {
		return nil, false
	}

	changed, err := cacheDB.IsFileChanged(filename)
	if err != nil || changed {
		return nil, false
	}

	record, err := cacheDB.GetFileRecord(filename)
	if err != nil {
		return nil, false
	}

	cachedIssues := reconstructIssuesFromCache(filename, record)
	if verbose {
		slog.Debug("Using cached results", "file", filename, "issues", len(cachedIssues))
	}
	return cachedIssues, true
}

// reconstructIssuesFromCache converts database issues to analyzer issues
func reconstructIssuesFromCache(filename string, record *cache.FileRecord) []*models.Issue {
	var cachedIssues []*models.Issue

	for _, dbIssue := range record.Issues {
		if isIssueIgnored(dbIssue.ID, record.Ignored) {
			continue
		}

		issue := convertDBIssueToAnalyzerIssue(filename, dbIssue)
		if issue != nil {
			cachedIssues = append(cachedIssues, issue)
		}
	}

	return cachedIssues
}

// isIssueIgnored checks if an issue ID is in the ignored list
func isIssueIgnored(issueID string, ignoredIDs []string) bool {
	for _, ignoredID := range ignoredIDs {
		if issueID == ignoredID {
			return true
		}
	}
	return false
}

// convertDBIssueToAnalyzerIssue converts a database issue to an analyzer issue
func convertDBIssueToAnalyzerIssue(filename string, dbIssue *models.Issue) *models.Issue {
	return &models.Issue{
		File:       filename,
		Line:       dbIssue.Line,
		Column:     dbIssue.Column,
		Type:       dbIssue.Type,
		Message:    dbIssue.Message,
		Severity:   models.SeverityLevelMedium, // Default severity
		Suggestion: dbIssue.Suggestion,
		CanBeFixed: dbIssue.CanBeFixed,
		Position: token.Position{
			Filename: filename,
			Line:     dbIssue.Line,
			Column:   dbIssue.Column,
		},
	}
}

// parseGoFile parses a Go source file
func parseGoFile(filename string) (*token.FileSet, *ast.File, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		slog.Warn("Error parsing file", "file", filename, "error", err)
		return nil, nil, err
	}
	return fset, node, nil
}

// buildEnabledAnalyzers builds the map of enabled analyzers from config
func buildEnabledAnalyzers(config *Config) map[string]bool {
	if config == nil {
		return nil // nil means run all
	}

	enabledAnalyzers := make(map[string]bool)
	analyzerMapping := getAnalyzerMapping()

	for configName, analyzerName := range analyzerMapping {
		analyzerConfig := config.GetAnalyzerConfig(configName)
		if analyzerConfig.Enabled {
			enabledAnalyzers[analyzerName] = true
		}
	}

	// If no analyzers enabled (shouldn't happen with default config), run all
	if len(enabledAnalyzers) == 0 {
		return nil
	}

	return enabledAnalyzers
}

// getAnalyzerMapping returns the mapping between config names and analyzer names
func getAnalyzerMapping() map[string]string {
	return map[string]string{
		"loop":                "loop",
		"deferoptimization":   "deferoptimization",
		"slice":               "slice",
		"map":                 "map",
		"reflection":          "reflection",
		"interface":           "interface",
		"regex":               "regex",
		"time":                "time",
		"memoryleak":          "memoryleak",
		"database":            "database",
		"apimisuse":           "apimisuse",
		"aibullshit":          "aibullshit",
		"goroutine":           "goroutine",
		"channel":             "channel",
		"httpclient":          "httpclient",
		"context":             "context",
		"racecondition":       "racecondition",
		"concurrencypatterns": "concurrencypatterns",
		"networkpatterns":     "networkpatterns",
		"cpuoptimization":     "cpuoptimization",
		"gcpressure":          "gcpressure",
		"syncpool":            "syncpool",
		"cgo":                 "cgo",
		"serialization":       "serialization",
		"crypto":              "crypto",
		"httpreuse":           "httpreuse",
		"iobuffer":            "iobuffer",
		"privacy":             "privacy",
		"testcoverage":        "testcoverage",
		"structlayout":        "structlayout",
		"cpucache":            "cpucache",
	}
}

// saveToCacheDB saves issues to the cache database
func saveToCacheDB(filename string, issues []*models.Issue, cacheDB *cache.FileCache) {
	if cacheDB == nil || noCache {
		return
	}

	interfaceIssues := make([]interface{}, len(issues))
	for i, issue := range issues {
		interfaceIssues[i] = issue
	}

	if err := cacheDB.SaveFileRecord(filename, interfaceIssues); err != nil {
		slog.Debug("Failed to save to cache", "file", filename, "error", err)
	}
}

// shouldSkipPath checks if a path should be skipped based on exclusion rules
func shouldSkipPath(path string, info os.FileInfo, excludes []string) (skip, skipDir bool) {
	cleanPath := filepath.Clean(path)

	for _, exclude := range excludes {
		cleanExclude := filepath.Clean(exclude)

		if strings.HasSuffix(exclude, ".go") {
			// File pattern (e.g., "_test.go")
			if !info.IsDir() && strings.HasSuffix(cleanPath, exclude) {
				return true, false
			}
			continue
		}

		// Check if path contains or matches the exclude pattern
		if strings.Contains(cleanPath, cleanExclude) {
			if info.IsDir() {
				return false, true
			}
			return true, false
		}

		// Check base name matching
		if filepath.Base(cleanPath) == exclude {
			if info.IsDir() {
				return false, true
			}
			return true, false
		}
	}

	return false, false
}

// isGoFile checks if a file is a Go source file
func isGoFile(path string, info os.FileInfo) bool {
	return !info.IsDir() && strings.HasSuffix(path, ".go")
}

// countLines counts the number of lines in a file using streaming for better memory efficiency
func countLines(path string) int {
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	// Handle files that don't end with newline
	if lineCount == 0 {
		// Check if file has any content
		if stat, err := file.Stat(); err == nil && stat.Size() > 0 {
			lineCount = 1
		}
	}

	return lineCount
}
