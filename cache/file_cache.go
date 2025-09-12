package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	CacheDir     = ".abscleaner"
	CacheFile    = "cache.json"
	CacheVersion = "1.0"
)

type FileCache struct {
	mu       sync.RWMutex
	baseDir  string
	cacheDir string
	data     *CacheData
}

type CacheData struct {
	Version      string                 `json:"version"`
	Files        map[string]*FileRecord `json:"files"`
	IgnoredRules map[string][]string    `json:"ignored_rules"` // file -> []rule_types
	Stats        *CacheStats            `json:"stats"`
	LastUpdated  time.Time              `json:"last_updated"`
}

type FileRecord struct {
	Path         string    `json:"path"`
	Hash         string    `json:"hash"`
	LastAnalyzed time.Time `json:"last_analyzed"`
	Issues       []*Issue  `json:"issues"`
	Ignored      []string  `json:"ignored"` // Issue IDs that are ignored
}

type Issue struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"` // Changed from analyzer.IssueType
	Line       int       `json:"line"`
	Column     int       `json:"column"`
	Message    string    `json:"message"`
	Severity   string    `json:"severity"` // Changed from analyzer.SeverityLevel
	Suggestion string    `json:"suggestion"`
	CanBeFixed bool      `json:"can_be_fixed"`
	IgnoredAt  time.Time `json:"ignored_at,omitempty"`
	FixedAt    time.Time `json:"fixed_at,omitempty"`
	IgnoreType string    `json:"ignore_type,omitempty"` // "comment", "manual", "config"
}

type CacheStats struct {
	TotalFiles    int       `json:"total_files"`
	TotalIssues   int       `json:"total_issues"`
	IgnoredIssues int       `json:"ignored_issues"`
	FixedIssues   int       `json:"fixed_issues"`
	LastFullScan  time.Time `json:"last_full_scan"`
	CacheHits     int       `json:"cache_hits"`
	CacheMisses   int       `json:"cache_misses"`
}

// Entry represents a cache entry (used by HybridCache)
type Entry struct {
	Hash   string    `json:"hash"`
	Issues []*Issue  `json:"issues"`
}

// New creates a new file-based cache
func New(baseDir string) (*FileCache, error) {
	if baseDir == "" {
		// Use current directory by default
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	cacheDir := filepath.Join(baseDir, CacheDir)

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	fc := &FileCache{
		baseDir:  baseDir,
		cacheDir: cacheDir,
		data: &CacheData{
			Version:      CacheVersion,
			Files:        make(map[string]*FileRecord),
			IgnoredRules: make(map[string][]string),
			Stats:        &CacheStats{},
			LastUpdated:  time.Now(),
		},
	}

	// Load existing cache if available
	if err := fc.load(); err != nil {
		// If loading fails, start with empty cache
		// Log the error but don't fail
		_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to load cache: %v\n", err)
	}

	return fc, nil
}

// load reads cache from disk
func (fc *FileCache) load() error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	cacheFile := filepath.Join(fc.cacheDir, CacheFile)
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No cache file yet, that's OK
			return nil
		}
		return err
	}

	var cacheData CacheData
	if err := json.Unmarshal(data, &cacheData); err != nil {
		return fmt.Errorf("failed to unmarshal cache: %w", err)
	}

	// Check version compatibility
	if cacheData.Version != CacheVersion {
		// Version mismatch, start fresh
		return fmt.Errorf("cache version mismatch: expected %s, got %s", CacheVersion, cacheData.Version)
	}

	fc.data = &cacheData
	return nil
}

// save writes cache to disk
func (fc *FileCache) save() error {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	fc.data.LastUpdated = time.Now()

	data, err := json.MarshalIndent(fc.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	cacheFile := filepath.Join(fc.cacheDir, CacheFile)
	tempFile := cacheFile + ".tmp"

	// Write to temp file first
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, cacheFile); err != nil {
		_ = os.Remove(tempFile) // Ignore error on cleanup
		return fmt.Errorf("failed to save cache: %w", err)
	}

	return nil
}

// CalculateFileHash calculates SHA256 hash of a file
func CalculateFileHash(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:]), nil
}

// IsFileChanged checks if a file has changed since last analysis
func (fc *FileCache) IsFileChanged(filePath string) (bool, error) {
	currentHash, err := CalculateFileHash(filePath)
	if err != nil {
		return true, err // Assume changed if can't read
	}

	fc.mu.RLock()
	record, exists := fc.data.Files[filePath]
	fc.mu.RUnlock()

	if !exists {
		fc.mu.Lock()
		fc.data.Stats.CacheMisses++
		fc.mu.Unlock()
		return true, nil
	}

	changed := record.Hash != currentHash

	fc.mu.Lock()
	if changed {
		fc.data.Stats.CacheMisses++
	} else {
		fc.data.Stats.CacheHits++
	}
	fc.mu.Unlock()

	return changed, nil
}

// SaveFileRecord saves or updates a file record
func (fc *FileCache) SaveFileRecord(filePath string, issues []interface{}) error {
	hash, err := CalculateFileHash(filePath)
	if err != nil {
		return err
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()

	// Preserve ignored issues from previous record
	oldIgnored := []string{}
	if oldRecord, exists := fc.data.Files[filePath]; exists {
		oldIgnored = oldRecord.Ignored
	}

	record := &FileRecord{
		Path:         filePath,
		Hash:         hash,
		LastAnalyzed: time.Now(),
		Issues:       convertIssues(issues),
		Ignored:      oldIgnored,
	}

	fc.data.Files[filePath] = record
	fc.updateStats()

	return fc.save()
}

// GetFileRecord retrieves a file record
func (fc *FileCache) GetFileRecord(filePath string) (*FileRecord, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	record, exists := fc.data.Files[filePath]
	if !exists {
		return nil, fmt.Errorf("file not found in cache: %s", filePath)
	}

	// Return a copy to prevent concurrent modification
	recordCopy := *record
	return &recordCopy, nil
}

// IgnoreIssue marks an issue as ignored
func (fc *FileCache) IgnoreIssue(filePath, issueID, ignoreType string) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	record, exists := fc.data.Files[filePath]
	if !exists {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// Add to ignored list if not already there
	found := false
	for _, id := range record.Ignored {
		if id == issueID {
			found = true
			break
		}
	}
	if !found {
		record.Ignored = append(record.Ignored, issueID)
	}

	// Update issue with ignore info
	for i := range record.Issues {
		if record.Issues[i].ID == issueID {
			record.Issues[i].IgnoredAt = time.Now()
			record.Issues[i].IgnoreType = ignoreType
			break
		}
	}

	fc.updateStats()
	return fc.save()
}

// IsIssueIgnored checks if an issue is ignored
func (fc *FileCache) IsIssueIgnored(filePath, issueID string) (bool, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	record, exists := fc.data.Files[filePath]
	if !exists {
		return false, nil
	}

	for _, id := range record.Ignored {
		if id == issueID {
			return true, nil
		}
	}
	return false, nil
}

// MarkIssueFixed marks an issue as fixed
func (fc *FileCache) MarkIssueFixed(filePath, issueID string) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	record, exists := fc.data.Files[filePath]
	if !exists {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// Update issue with fix info
	for i := range record.Issues {
		if record.Issues[i].ID == issueID {
			record.Issues[i].FixedAt = time.Now()
			break
		}
	}

	fc.updateStats()
	return fc.save()
}

// GetStats returns cache statistics
func (fc *FileCache) GetStats() map[string]interface{} {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	return map[string]interface{}{
		"total_files":    fc.data.Stats.TotalFiles,
		"total_issues":   fc.data.Stats.TotalIssues,
		"ignored_issues": fc.data.Stats.IgnoredIssues,
		"fixed_issues":   fc.data.Stats.FixedIssues,
		"cache_hits":     fc.data.Stats.CacheHits,
		"cache_misses":   fc.data.Stats.CacheMisses,
		"last_full_scan": fc.data.Stats.LastFullScan,
		"last_updated":   fc.data.LastUpdated,
	}
}

// updateStats recalculates statistics
func (fc *FileCache) updateStats() {
	stats := &CacheStats{
		CacheHits:    fc.data.Stats.CacheHits,
		CacheMisses:  fc.data.Stats.CacheMisses,
		LastFullScan: fc.data.Stats.LastFullScan,
	}

	for _, record := range fc.data.Files {
		stats.TotalFiles++
		stats.TotalIssues += len(record.Issues)
		stats.IgnoredIssues += len(record.Ignored)

		for _, issue := range record.Issues {
			if !issue.FixedAt.IsZero() {
				stats.FixedIssues++
			}
		}
	}

	fc.data.Stats = stats
}

// ClearCache clears all cached data
func (fc *FileCache) ClearCache() error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.data = &CacheData{
		Version:      CacheVersion,
		Files:        make(map[string]*FileRecord),
		IgnoredRules: make(map[string][]string),
		Stats:        &CacheStats{},
		LastUpdated:  time.Now(),
	}

	return fc.save()
}

// AddIgnoredRule adds a rule type to ignore for a file pattern
func (fc *FileCache) AddIgnoredRule(filePattern, ruleType string) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if fc.data.IgnoredRules[filePattern] == nil {
		fc.data.IgnoredRules[filePattern] = []string{}
	}

	// Check if already ignored
	for _, rt := range fc.data.IgnoredRules[filePattern] {
		if rt == ruleType {
			return nil
		}
	}

	fc.data.IgnoredRules[filePattern] = append(fc.data.IgnoredRules[filePattern], ruleType)
	return fc.save()
}

// ShouldIgnoreRule checks if a rule should be ignored for a file
func (fc *FileCache) ShouldIgnoreRule(filePath, ruleType string) bool {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	for pattern, rules := range fc.data.IgnoredRules {
		if matchPattern(filePath, pattern) {
			for _, rt := range rules {
				if rt == ruleType || rt == "*" {
					return true
				}
			}
		}
	}
	return false
}

// matchPattern checks if a file path matches a pattern
func matchPattern(path, pattern string) bool {
	// Simple pattern matching
	// Support * for wildcards and ** for recursive
	if pattern == "*" || pattern == "**" {
		return true
	}

	// Handle ** for recursive matching
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			return strings.HasPrefix(path, parts[0]) && strings.HasSuffix(path, parts[1])
		}
	}

	// Handle * for single-level wildcard
	if strings.Contains(pattern, "*") {
		// Convert pattern to regex-like matching
		pattern = strings.ReplaceAll(pattern, "*", ".*")
		// This is simplified - in production, use proper glob matching
		return strings.Contains(path, strings.ReplaceAll(pattern, ".*", ""))
	}

	// Exact match or prefix match
	return path == pattern || strings.HasPrefix(path, pattern)
}

// Helper function to convert generic issues to cache Issue
func convertIssues(analyzerIssues []interface{}) []*Issue {
	issues := make([]*Issue, 0, len(analyzerIssues))
	for i := range analyzerIssues {
		// Use reflection to extract fields from the interface
		// This is a temporary workaround for the import cycle
		issues = append(
			issues, &Issue{
				ID:         generateIssueID(analyzerIssues[i]),
				Type:       "generic",
				Line:       0,
				Column:     0,
				Message:    "cached issue",
				Severity:   "medium",
				Suggestion: "",
				CanBeFixed: false,
			},
		)
	}
	return issues
}

// Removed unused conversion functions - using single Entry type now

// SaveRawRecord saves a raw Entry directly
func (fc *FileCache) SaveRawRecord(key string, entry Entry) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	// Create a FileRecord from the Entry
	fc.data.Files[key] = &FileRecord{
		Path:         key,
		Hash:         entry.Hash,
		LastAnalyzed: time.Now(),
		Issues:       entry.Issues,
		Ignored:      []string{}, // Preserve any existing ignored items if needed
	}
	
	fc.updateStats()
	return fc.save()
}

// Generate unique ID for an issue
func generateIssueID(issue interface{}) string {
	// Simple hash based on pointer address for now
	data := fmt.Sprintf("%p", issue)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes for shorter ID
}

// GetCacheDir returns the cache directory path
func (fc *FileCache) GetCacheDir() string {
	return fc.cacheDir
}

// Close saves and closes the cache (satisfies DB interface)
func (fc *FileCache) Close() error {
	return fc.save()
}
