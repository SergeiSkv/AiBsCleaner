package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SergeiSkv/AiBsCleaner/models"
	"github.com/stretchr/testify/require"
)

func TestNewFileCache(t *testing.T) {
	tdir := t.TempDir()
	fc, err := New(tdir)
	require.NoError(t, err)
	require.NotNil(t, fc)
	require.Equal(t, filepath.Join(tdir, CacheDir), fc.GetCacheDir())
}

func TestSaveAndLoadFileRecord(t *testing.T) {
	tdir := t.TempDir()
	testFile := filepath.Join(tdir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("package main"), 0o644))

	fc, err := New(tdir)
	require.NoError(t, err)

	issue := &models.Issue{
		ID:      "test-1",
		Type:    models.IssueAllocInLoop,
		Message: "test allocation",
	}
	err = fc.SaveFileRecord(testFile, []interface{}{issue})
	require.NoError(t, err)

	record, err := fc.GetFileRecord(testFile)
	require.NoError(t, err)
	require.NotEmpty(t, record.Hash)
	require.Len(t, record.Issues, 1)
	require.Equal(t, "test-1", record.Issues[0].ID)
}

func TestIsFileChanged(t *testing.T) {
	tdir := t.TempDir()
	testFile := filepath.Join(tdir, "test.go")
	content1 := []byte("package main\nvar x = 1")
	require.NoError(t, os.WriteFile(testFile, content1, 0o644))

	fc, err := New(tdir)
	require.NoError(t, err)

	// Save initial
	err = fc.SaveFileRecord(testFile, []interface{}{})
	require.NoError(t, err)

	// File not changed
	changed, err := fc.IsFileChanged(testFile)
	require.NoError(t, err)
	require.False(t, changed)

	// Modify file
	content2 := []byte("package main\nvar x = 2")
	require.NoError(t, os.WriteFile(testFile, content2, 0o644))

	// File changed
	changed, err = fc.IsFileChanged(testFile)
	require.NoError(t, err)
	require.True(t, changed)
}

func TestIgnoreIssue(t *testing.T) {
	tdir := t.TempDir()
	testFile := filepath.Join(tdir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("package main"), 0o644))

	fc, err := New(tdir)
	require.NoError(t, err)

	issue := &models.Issue{ID: "issue-1", Type: models.IssueAllocInLoop}
	err = fc.SaveFileRecord(testFile, []interface{}{issue})
	require.NoError(t, err)

	err = fc.IgnoreIssue(testFile, "issue-1", "manual")
	require.NoError(t, err)

	ignored, err := fc.IsIssueIgnored(testFile, "issue-1")
	require.NoError(t, err)
	require.True(t, ignored)

	ignored, err = fc.IsIssueIgnored(testFile, "issue-2")
	require.NoError(t, err)
	require.False(t, ignored)
}

func TestMarkIssueFixed(t *testing.T) {
	tdir := t.TempDir()
	testFile := filepath.Join(tdir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("package main"), 0o644))

	fc, err := New(tdir)
	require.NoError(t, err)

	issue := &models.Issue{ID: "issue-1", Type: models.IssueAllocInLoop}
	err = fc.SaveFileRecord(testFile, []interface{}{issue})
	require.NoError(t, err)

	err = fc.MarkIssueFixed(testFile, "issue-1")
	require.NoError(t, err)

	record, err := fc.GetFileRecord(testFile)
	require.NoError(t, err)
	require.NotNil(t, record.Issues[0].FixedAt)
}

func TestGetStats(t *testing.T) {
	tdir := t.TempDir()
	testFile := filepath.Join(tdir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("package main"), 0o644))

	fc, err := New(tdir)
	require.NoError(t, err)

	issue := &models.Issue{ID: "i1", Type: models.IssueAllocInLoop}
	err = fc.SaveFileRecord(testFile, []interface{}{issue})
	require.NoError(t, err)

	stats := fc.GetStats()
	require.Equal(t, 1, stats["total_files"])
	require.Equal(t, 1, stats["total_issues"])
}

func TestClearCache(t *testing.T) {
	tdir := t.TempDir()
	testFile := filepath.Join(tdir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("package main"), 0o644))

	fc, err := New(tdir)
	require.NoError(t, err)

	issue := &models.Issue{ID: "i1", Type: models.IssueAllocInLoop}
	err = fc.SaveFileRecord(testFile, []interface{}{issue})
	require.NoError(t, err)

	err = fc.ClearCache()
	require.NoError(t, err)

	_, err = fc.GetFileRecord(testFile)
	require.Error(t, err)
}

func TestAddIgnoredRule(t *testing.T) {
	tdir := t.TempDir()
	fc, err := New(tdir)
	require.NoError(t, err)

	err = fc.AddIgnoredRule("pkg/", "RaceCondition")
	require.NoError(t, err)

	require.True(t, fc.ShouldIgnoreRule("pkg/file.go", "RaceCondition"))
	require.False(t, fc.ShouldIgnoreRule("pkg/file.go", "LoopAllocation"))
}

func TestShouldIgnoreRuleWithWildcard(t *testing.T) {
	tdir := t.TempDir()
	fc, err := New(tdir)
	require.NoError(t, err)

	err = fc.AddIgnoredRule("test/", "*")
	require.NoError(t, err)

	require.True(t, fc.ShouldIgnoreRule("test/file.go", "RaceCondition"))
	require.True(t, fc.ShouldIgnoreRule("test/file.go", "AnyIssue"))
}

func TestSaveRawRecord(t *testing.T) {
	tdir := t.TempDir()
	fc, err := New(tdir)
	require.NoError(t, err)

	entry := Entry{
		Hash:   "abc123",
		Issues: []*models.Issue{{ID: "raw-1", Type: models.IssueAllocInLoop}},
	}
	err = fc.SaveRawRecord("raw.go", entry)
	require.NoError(t, err)

	record, err := fc.GetFileRecord("raw.go")
	require.NoError(t, err)
	require.Equal(t, "abc123", record.Hash)
	require.Len(t, record.Issues, 1)
}

func TestClose(t *testing.T) {
	tdir := t.TempDir()
	testFile := filepath.Join(tdir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("package main"), 0o644))

	fc, err := New(tdir)
	require.NoError(t, err)

	issue := &models.Issue{ID: "i1", Type: models.IssueAllocInLoop}
	err = fc.SaveFileRecord(testFile, []interface{}{issue})
	require.NoError(t, err)

	err = fc.Close()
	require.NoError(t, err)

	// Reload and verify persistence
	fc2, err := New(tdir)
	require.NoError(t, err)
	record, err := fc2.GetFileRecord(testFile)
	require.NoError(t, err)
	require.NotEmpty(t, record.Hash)
	require.Len(t, record.Issues, 1)
}

func TestCalculateFileHash(t *testing.T) {
	tdir := t.TempDir()
	file := filepath.Join(tdir, "hash_test.go")
	require.NoError(t, os.WriteFile(file, []byte("content"), 0o644))

	hash1, err := CalculateFileHash(file)
	require.NoError(t, err)
	require.NotEmpty(t, hash1)

	// Same content -> same hash
	hash2, err := CalculateFileHash(file)
	require.NoError(t, err)
	require.Equal(t, hash1, hash2)

	// Different content -> different hash
	require.NoError(t, os.WriteFile(file, []byte("different"), 0o644))
	hash3, err := CalculateFileHash(file)
	require.NoError(t, err)
	require.NotEqual(t, hash1, hash3)
}

func TestCacheStatsUpdated(t *testing.T) {
	tdir := t.TempDir()
	fc, err := New(tdir)
	require.NoError(t, err)

	file1 := filepath.Join(tdir, "f1.go")
	file2 := filepath.Join(tdir, "f2.go")
	require.NoError(t, os.WriteFile(file1, []byte("package f1"), 0o644))
	require.NoError(t, os.WriteFile(file2, []byte("package f2"), 0o644))

	issue1 := &models.Issue{ID: "i1", Type: models.IssueAllocInLoop}
	issue2 := &models.Issue{ID: "i2", Type: models.IssueAllocInLoop}
	issue3 := &models.Issue{ID: "i3", Type: models.IssueAllocInLoop}

	err = fc.SaveFileRecord(file1, []interface{}{issue1, issue2})
	require.NoError(t, err)
	err = fc.SaveFileRecord(file2, []interface{}{issue3})
	require.NoError(t, err)

	stats := fc.GetStats()
	require.Equal(t, 2, stats["total_files"])
	require.Equal(t, 3, stats["total_issues"])

	// Ignore one issue
	err = fc.IgnoreIssue(file1, "i1", "manual")
	require.NoError(t, err)

	stats = fc.GetStats()
	require.Equal(t, 1, stats["ignored_issues"])

	// Mark one fixed
	err = fc.MarkIssueFixed(file1, "i2")
	require.NoError(t, err)

	stats = fc.GetStats()
	require.Equal(t, 1, stats["fixed_issues"])
}

func TestGetFileRecordNotFound(t *testing.T) {
	tdir := t.TempDir()
	fc, err := New(tdir)
	require.NoError(t, err)

	_, err = fc.GetFileRecord("nonexistent.go")
	require.Error(t, err)
}

func TestUpdateExistingRecord(t *testing.T) {
	tdir := t.TempDir()
	testFile := filepath.Join(tdir, "test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("package main"), 0o644))

	fc, err := New(tdir)
	require.NoError(t, err)

	// First save
	issue1 := &models.Issue{ID: "i1", Type: models.IssueAllocInLoop}
	err = fc.SaveFileRecord(testFile, []interface{}{issue1})
	require.NoError(t, err)

	// Ignore issue
	err = fc.IgnoreIssue(testFile, "i1", "manual")
	require.NoError(t, err)

	// Update record with new issues
	issue2 := &models.Issue{ID: "i2", Type: models.IssueAllocInLoop}
	require.NoError(t, os.WriteFile(testFile, []byte("package main\n// changed"), 0o644))
	err = fc.SaveFileRecord(testFile, []interface{}{issue2})
	require.NoError(t, err)

	// Verify ignored list preserved
	record, err := fc.GetFileRecord(testFile)
	require.NoError(t, err)
	require.Contains(t, record.Ignored, "i1")
}

func TestConvertIssues(t *testing.T) {
	issue := &models.Issue{
		ID:      "test",
		Type:    models.IssueAllocInLoop,
		Message: "test message",
	}

	items := []interface{}{issue, "not an issue"}
	converted := convertIssues(items)

	require.Len(t, converted, 2)
	require.Equal(t, "test", converted[0].ID)
	require.NotEmpty(t, converted[1].ID) // Generated ID for non-Issue
}
