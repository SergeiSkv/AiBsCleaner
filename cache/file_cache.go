package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SergeiSkv/AiBsCleaner/models"
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
	Version      string        `json:"version"`
	Files        []*FileRecord `json:"files"`
	IgnoredRules []IgnoredRule `json:"ignored_rules"`
	Stats        *CacheStats   `json:"stats"`
	LastUpdated  time.Time     `json:"last_updated"`
}

type IgnoredRule struct {
	FilePattern string   `json:"file_pattern"`
	RuleTypes   []string `json:"rule_types"`
}

type FileRecord struct {
	Path         string          `json:"path"`
	Hash         string          `json:"hash"`
	LastAnalyzed time.Time       `json:"last_analyzed"`
	Issues       []*models.Issue `json:"issues"`
	Ignored      []string        `json:"ignored"` // Issue IDs that are ignored
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
	Hash   string          `json:"hash"`
	Issues []*models.Issue `json:"issues"`
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
			Files:        make([]*FileRecord, 0, 100),
			IgnoredRules: make([]IgnoredRule, 0, 10),
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
	file, err := os.Open(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No cache file yet, that's OK
			return nil
		}
		return err
	}
	defer func() { _ = file.Close() }()

	// Use streaming decoder to reduce memory pressure and improve performance
	var cacheData CacheData
	decoder := json.NewDecoder(file)
	decoder.UseNumber() // Use Number for better performance with large numbers
	if err := decoder.Decode(&cacheData); err != nil {
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

// save writes cache to disk (acquires lock)
func (fc *FileCache) save() error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	return fc.saveUnsafe()
}

// saveUnsafe writes cache to disk without locking (must be called with lock held)
func (fc *FileCache) saveUnsafe() error {
	fc.data.LastUpdated = time.Now()

	data, err := json.Marshal(fc.data)
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	cacheFile := filepath.Join(fc.cacheDir, CacheFile)
	tempFile := cacheFile + ".tmp"

	// Write to temp file first
	if err := os.WriteFile(tempFile, data, 0o644); err != nil {
		return fmt.Errorf("failed to write cache: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, cacheFile); err != nil {
		_ = os.Remove(tempFile) // Ignore error on cleanup
		return fmt.Errorf("failed to save cache: %w", err)
	}

	return nil
}

// CalculateFileHash calculates SHA256 hash of a file using streaming for better memory efficiency
func CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// IsFileChanged checks if a file has changed since last analysis
func (fc *FileCache) IsFileChanged(filePath string) (bool, error) {
	currentHash, err := CalculateFileHash(filePath)
	if err != nil {
		return true, err // Assume changed if can't read
	}

	fc.mu.RLock()
	var record *FileRecord
	for _, r := range fc.data.Files {
		if r.Path == filePath {
			record = r
			break
		}
	}
	fc.mu.RUnlock()

	if record == nil {
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
	var oldIgnored []string
	for i, existingRecord := range fc.data.Files {
		if existingRecord.Path == filePath {
			oldIgnored = existingRecord.Ignored
			// Remove old record
			fc.data.Files = append(fc.data.Files[:i], fc.data.Files[i+1:]...)
			break
		}
	}

	record := &FileRecord{
		Path:         filePath,
		Hash:         hash,
		LastAnalyzed: time.Now(),
		Issues:       convertIssues(issues),
		Ignored:      oldIgnored,
	}

	fc.data.Files = append(fc.data.Files, record)
	fc.updateStats()

	return fc.saveUnsafe()
}

// GetFileRecord retrieves a file record
func (fc *FileCache) GetFileRecord(filePath string) (*FileRecord, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	for _, record := range fc.data.Files {
		if record.Path == filePath {
			// Return a copy to prevent concurrent modification
			recordCopy := *record
			return &recordCopy, nil
		}
	}

	return nil, fmt.Errorf("file not found in cache: %s", filePath)
}

// IgnoreIssue marks an issue as ignored
func (fc *FileCache) IgnoreIssue(filePath, issueID, ignoreType string) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	var record *FileRecord
	for _, r := range fc.data.Files {
		if r.Path == filePath {
			record = r
			break
		}
	}
	if record == nil {
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
	now := time.Now()
	for i := range record.Issues {
		if record.Issues[i].ID == issueID {
			record.Issues[i].IgnoredAt = now
			// Convert string to IssueType - for now use a default
			record.Issues[i].IgnoreType = models.IssueAPIMisuse // Default ignore type
			break
		}
	}

	fc.updateStats()
	return fc.saveUnsafe()
}

// IsIssueIgnored checks if an issue is ignored
func (fc *FileCache) IsIssueIgnored(filePath, issueID string) (bool, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	var record *FileRecord
	for _, r := range fc.data.Files {
		if r.Path == filePath {
			record = r
			break
		}
	}
	if record == nil {
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

	var record *FileRecord
	for _, r := range fc.data.Files {
		if r.Path == filePath {
			record = r
			break
		}
	}
	if record == nil {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// Update issue with fix info
	now := time.Now()
	for i := range record.Issues {
		if record.Issues[i].ID == issueID {
			record.Issues[i].FixedAt = now
			break
		}
	}

	fc.updateStats()
	return fc.saveUnsafe()
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
		Files:        make([]*FileRecord, 0, 100),
		IgnoredRules: make([]IgnoredRule, 0, 10),
		Stats:        &CacheStats{},
		LastUpdated:  time.Now(),
	}

	return fc.saveUnsafe()
}

// AddIgnoredRule adds a rule type to ignore for a file pattern
func (fc *FileCache) AddIgnoredRule(filePattern, ruleType string) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	// Find existing rule for this pattern
	for i, rule := range fc.data.IgnoredRules {
		if rule.FilePattern == filePattern {
			// Check if rule type already exists
			for _, rt := range rule.RuleTypes {
				if rt == ruleType {
					return nil // Already exists
				}
			}
			// Add a new rule type
			fc.data.IgnoredRules[i].RuleTypes = append(fc.data.IgnoredRules[i].RuleTypes, ruleType)
			return fc.saveUnsafe()
		}
	}

	// Create new ignored rule
	newRule := IgnoredRule{
		FilePattern: filePattern,
		RuleTypes:   []string{ruleType},
	}
	fc.data.IgnoredRules = append(fc.data.IgnoredRules, newRule)
	return fc.saveUnsafe()
}

// ShouldIgnoreRule checks if a rule should be ignored for a file
func (fc *FileCache) ShouldIgnoreRule(filePath, ruleType string) bool {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	for _, rule := range fc.data.IgnoredRules {
		if matchPattern(filePath, rule.FilePattern) {
			for _, rt := range rule.RuleTypes {
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
func convertIssues(analyzerIssues []interface{}) []*models.Issue {
	issues := make([]*models.Issue, 0, len(analyzerIssues))
	for i := range analyzerIssues {
		// Check if it's already an Issue
		if issue, ok := analyzerIssues[i].(*models.Issue); ok {
			issues = append(issues, issue)
			continue
		}

		// Otherwise create a generic issue
		issues = append(
			issues, &models.Issue{
				ID:         generateIssueID(analyzerIssues[i]),
				Type:       models.IssueAPIMisuse, // Default generic type
				Line:       0,
				Column:     0,
				Message:    "cached issue",
				Severity:   models.SeverityLevelMedium,
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

	// Remove existing record if it exists
	for i, existingRecord := range fc.data.Files {
		if existingRecord.Path == key {
			fc.data.Files = append(fc.data.Files[:i], fc.data.Files[i+1:]...)
			break
		}
	}

	// Create a FileRecord from the Entry
	newRecord := &FileRecord{
		Path:         key,
		Hash:         entry.Hash,
		LastAnalyzed: time.Now(),
		Issues:       entry.Issues,
		Ignored:      []string{}, // Preserve any existing ignored items if needed
	}
	fc.data.Files = append(fc.data.Files, newRecord)

	fc.updateStats()
	return fc.saveUnsafe()
}

// Generate unique ID for an issue
func generateIssueID(issue interface{}) string {
	// Simple hash based on pointer address for now
	data := fmt.Sprintf("%p", issue)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // Use the first 8 bytes for shorter ID
}

// GetCacheDir returns the cache directory path
func (fc *FileCache) GetCacheDir() string {
	return fc.cacheDir
}

// Close saves and closes the cache (satisfies DB interface)
func (fc *FileCache) Close() error {
	return fc.save()
}
