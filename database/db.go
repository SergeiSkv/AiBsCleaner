package database

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vmihailenco/msgpack/v5"
	bolt "go.etcd.io/bbolt"

	"github.com/SergeiSkv/AiBsCleaner/analyzer"
)

type DB struct {
	db *bolt.DB
}

type FileRecord struct {
	Path         string    `msgpack:"path"`
	Hash         string    `msgpack:"hash"`
	LastAnalyzed time.Time `msgpack:"last_analyzed"`
	Issues       []Issue   `msgpack:"issues"`
	Ignored      []string  `msgpack:"ignored"` // Issue IDs that are ignored
}

type Issue struct {
	ID         string    `msgpack:"id"`
	Type       string    `msgpack:"type"`
	Line       int       `msgpack:"line"`
	Column     int       `msgpack:"column"`
	Message    string    `msgpack:"message"`
	Severity   string    `msgpack:"severity"`
	Suggestion string    `msgpack:"suggestion"`
	CanBeFixed bool      `msgpack:"can_be_fixed"`
	IgnoredAt  time.Time `msgpack:"ignored_at,omitempty"`
	FixedAt    time.Time `msgpack:"fixed_at,omitempty"`
	IgnoreType string    `msgpack:"ignore_type,omitempty"` // "comment", "manual", "config"
}

const (
	BucketFiles   = "files"
	BucketIgnored = "ignored"
	BucketFixed   = "fixed"
)

// New creates a new database instance
func New(dbPath string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create buckets if they don't exist
	err = db.Update(
		func(tx *bolt.Tx) error {
			for _, bucket := range []string{BucketFiles, BucketIgnored, BucketFixed} {
				if _, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
					return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
				}
			}
			return nil
		},
	)
	if err != nil {
		db.Close()
		return nil, err
	}

	return &DB{db: db}, nil
}

// Close closes the database
func (d *DB) Close() error {
	return d.db.Close()
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
func (d *DB) IsFileChanged(filePath string) (bool, error) {
	currentHash, err := CalculateFileHash(filePath)
	if err != nil {
		return true, err // Assume changed if can't read
	}

	var record FileRecord
	err = d.db.View(
		func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(BucketFiles))
			data := b.Get([]byte(filePath))
			if data == nil {
				return nil // File not in DB, so it's "changed"
			}
			return msgpack.Unmarshal(data, &record)
		},
	)
	if err != nil {
		return true, err
	}

	return record.Hash != currentHash, nil
}

// SaveFileRecord saves or updates a file record
func (d *DB) SaveFileRecord(filePath string, issues []analyzer.Issue) error {
	hash, err := CalculateFileHash(filePath)
	if err != nil {
		return err
	}

	record := FileRecord{
		Path:         filePath,
		Hash:         hash,
		LastAnalyzed: time.Now(),
		Issues:       convertIssues(issues),
		Ignored:      []string{},
	}

	// Preserve ignored issues from previous record
	var oldRecord FileRecord
	err = d.db.View(
		func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(BucketFiles))
			data := b.Get([]byte(filePath))
			if data != nil {
				return msgpack.Unmarshal(data, &oldRecord)
			}
			return nil
		},
	)
	if err == nil && len(oldRecord.Ignored) > 0 {
		record.Ignored = oldRecord.Ignored
	}

	return d.db.Update(
		func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(BucketFiles))
			data, err := msgpack.Marshal(record)
			if err != nil {
				return err
			}
			return b.Put([]byte(filePath), data)
		},
	)
}

// GetFileRecord retrieves a file record
func (d *DB) GetFileRecord(filePath string) (*FileRecord, error) {
	var record FileRecord
	err := d.db.View(
		func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(BucketFiles))
			data := b.Get([]byte(filePath))
			if data == nil {
				return fmt.Errorf("file not found: %s", filePath)
			}
			return msgpack.Unmarshal(data, &record)
		},
	)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// IgnoreIssue marks an issue as ignored
func (d *DB) IgnoreIssue(filePath string, issueID string, ignoreType string) error {
	return d.db.Update(
		func(tx *bolt.Tx) error {
			// Update file record
			b := tx.Bucket([]byte(BucketFiles))
			data := b.Get([]byte(filePath))
			if data == nil {
				return fmt.Errorf("file not found: %s", filePath)
			}

			var record FileRecord
			if err := msgpack.Unmarshal(data, &record); err != nil {
				return err
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
			for i, issue := range record.Issues {
				if issue.ID == issueID {
					record.Issues[i].IgnoredAt = time.Now()
					record.Issues[i].IgnoreType = ignoreType
					break
				}
			}

			// Save updated record
			data, err := msgpack.Marshal(record)
			if err != nil {
				return err
			}
			return b.Put([]byte(filePath), data)
		},
	)
}

// IsIssueIgnored checks if an issue is ignored
func (d *DB) IsIssueIgnored(filePath string, issueID string) (bool, error) {
	var record FileRecord
	err := d.db.View(
		func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(BucketFiles))
			data := b.Get([]byte(filePath))
			if data == nil {
				return nil // File not found, issue not ignored
			}
			return msgpack.Unmarshal(data, &record)
		},
	)
	if err != nil {
		return false, err
	}

	for _, id := range record.Ignored {
		if id == issueID {
			return true, nil
		}
	}
	return false, nil
}

// MarkIssueFixed marks an issue as fixed
func (d *DB) MarkIssueFixed(filePath string, issueID string) error {
	return d.db.Update(
		func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(BucketFiles))
			data := b.Get([]byte(filePath))
			if data == nil {
				return fmt.Errorf("file not found: %s", filePath)
			}

			var record FileRecord
			if err := msgpack.Unmarshal(data, &record); err != nil {
				return err
			}

			// Update issue with fix info
			for i, issue := range record.Issues {
				if issue.ID == issueID {
					record.Issues[i].FixedAt = time.Now()
					break
				}
			}

			// Save updated record
			data, err := msgpack.Marshal(record)
			if err != nil {
				return err
			}
			return b.Put([]byte(filePath), data)
		},
	)
}

// GetStats returns database statistics
func (d *DB) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	err := d.db.View(
		func(tx *bolt.Tx) error {
			filesB := tx.Bucket([]byte(BucketFiles))

			totalFiles := 0
			totalIssues := 0
			ignoredIssues := 0
			fixedIssues := 0

			err := filesB.ForEach(
				func(k, v []byte) error {
					totalFiles++
					var record FileRecord
					err := msgpack.Unmarshal(v, &record)
					if err != nil {
						// Skip malformed records
						return nil
					}
					totalIssues += len(record.Issues)
					ignoredIssues += len(record.Ignored)
					for _, issue := range record.Issues {
						if !issue.FixedAt.IsZero() {
							fixedIssues++
						}
					}
					return nil
				},
			)
			if err != nil {
				return err
			}

			stats["total_files"] = totalFiles
			stats["total_issues"] = totalIssues
			stats["ignored_issues"] = ignoredIssues
			stats["fixed_issues"] = fixedIssues

			return nil
		},
	)

	return stats, err
}

// Helper function to convert analyzer.Issue to database Issue
func convertIssues(analyzerIssues []analyzer.Issue) []Issue {
	issues := make([]Issue, len(analyzerIssues))
	for i, ai := range analyzerIssues {
		issues[i] = Issue{
			ID:         generateIssueID(ai),
			Type:       ai.Type,
			Line:       ai.Line,
			Column:     ai.Column,
			Message:    ai.Message,
			Severity:   string(ai.Severity),
			Suggestion: ai.Suggestion,
			CanBeFixed: ai.CanBeFixed,
		}
	}
	return issues
}

// Generate unique ID for an issue
func generateIssueID(issue analyzer.Issue) string {
	data := fmt.Sprintf("%s:%d:%d:%s", issue.File, issue.Line, issue.Column, issue.Type)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes for shorter ID
}

// ClearCache clears all cached data
func (d *DB) ClearCache() error {
	return d.db.Update(
		func(tx *bolt.Tx) error {
			for _, bucket := range []string{BucketFiles, BucketIgnored, BucketFixed} {
				if err := tx.DeleteBucket([]byte(bucket)); err != nil && err != bolt.ErrBucketNotFound {
					return err
				}
				if _, err := tx.CreateBucket([]byte(bucket)); err != nil {
					return err
				}
			}
			return nil
		},
	)
}
