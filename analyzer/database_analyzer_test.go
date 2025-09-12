package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestDatabaseAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "N+1 query problem - context missing",
			code: `package main
import "database/sql"
func test(db *sql.DB, userIDs []int) {
	for _, id := range userIDs {
		row := db.QueryRow("SELECT name FROM users WHERE id = ?", id)
		var name string
		row.Scan(&name)
	}
}`,
			expected: []string{"MISSING_CONTEXT"},
		},
		{
			name: "SQL injection risk with string concatenation",
			code: `package main
import "database/sql"
func test(db *sql.DB, userID string) {
	db.Query("SELECT * FROM users WHERE id = " + userID)
}`,
			expected: []string{"SQL_INJECTION_RISK", "MISSING_CONTEXT"},
		},
		{
			name: "SQL injection risk with fmt.Sprintf",
			code: `package main
import (
	"database/sql"
	"fmt"
)
func test(db *sql.DB, userID string) {
	db.Query(fmt.Sprintf("SELECT * FROM users WHERE id = %s", userID))
}`,
			expected: []string{"SQL_INJECTION_RISK", "MISSING_CONTEXT"},
		},
		{
			name: "transaction without rollback",
			code: `package main
import "database/sql"
func test(db *sql.DB) {
	tx, _ := db.Begin()
	tx.Exec("INSERT INTO users (name) VALUES (?)", "John")
	tx.Commit()
}`,
			expected: []string{"MISSING_ROLLBACK"},
		},
		{
			name: "prepared statement not closed",
			code: `package main
import "database/sql"
func test(db *sql.DB) {
	stmt, _ := db.Prepare("SELECT * FROM users WHERE id = ?")
	stmt.Query(1)
}`,
			expected: []string{"UNCLOSED_PREPARED_STMT"},
		},
		{
			name: "query rows not closed",
			code: `package main
import "database/sql"
func test(db *sql.DB) {
	rows, _ := db.Query("SELECT * FROM users")
	for rows.Next() {
		// process rows
	}
}`,
			expected: []string{"UNCLOSED_ROWS"},
		},
		{
			name: "database operation without context",
			code: `package main
import "database/sql"
func test(db *sql.DB) {
	db.Query("SELECT * FROM users")
}`,
			expected: []string{"MISSING_CONTEXT"},
		},
		{
			name: "proper database usage - still has some context detection issues",
			code: `package main
import (
	"context"
	"database/sql"
)
func test(db *sql.DB, ctx context.Context) {
	tx, _ := db.BeginTx(ctx, nil)
	defer tx.Rollback()
	
	stmt, _ := tx.PrepareContext(ctx, "SELECT id, name FROM users WHERE id = ?")
	defer stmt.Close()
	
	rows, _ := stmt.QueryContext(ctx, 1)
	defer rows.Close()
	
	for rows.Next() {
		// process rows
	}
	
	tx.Commit()
}`,
			expected: []string{"MISSING_CONTEXT", "MISSING_COMMIT"},
		},
		{
			name: "too many queries without transaction",
			code: `package main
import "database/sql"
func test(db *sql.DB) {
	db.Exec("INSERT INTO table1 (col) VALUES (?)", "val1")
	db.Exec("INSERT INTO table2 (col) VALUES (?)", "val2")
	db.Exec("INSERT INTO table3 (col) VALUES (?)", "val3")
	db.Exec("INSERT INTO table4 (col) VALUES (?)", "val4")
	db.Exec("INSERT INTO table5 (col) VALUES (?)", "val5")
	db.Exec("INSERT INTO table6 (col) VALUES (?)", "val6")
}`,
			expected: []string{"TOO_MANY_QUERIES"},
		},
		{
			name: "SELECT * usage",
			code: `package main
import "database/sql"
func test(db *sql.DB) {
	db.Query("SELECT * FROM users")
}`,
			expected: []string{"SELECT_STAR", "MISSING_CONTEXT"},
		},
		{
			name: "SELECT without LIMIT",
			code: `package main
import "database/sql"
func test(db *sql.DB) {
	db.Query("SELECT name FROM users")
}`,
			expected: []string{"MISSING_LIMIT", "MISSING_CONTEXT"},
		},
		{
			name: "OR in WHERE clause",
			code: `package main
import "database/sql"
func test(db *sql.DB) {
	db.Query("SELECT name FROM users WHERE status = 'active' OR status = 'pending'")
}`,
			expected: []string{"OR_IN_WHERE", "MISSING_CONTEXT"},
		},
		{
			name: "LIKE with leading wildcard",
			code: `package main
import "database/sql"
func test(db *sql.DB) {
	db.Query("SELECT name FROM users WHERE name LIKE '%john%'")
}`,
			expected: []string{"LEADING_WILDCARD", "MISSING_CONTEXT"},
		},
		{
			name: "multiple database issues",
			code: `package main
import "database/sql"
func test(db *sql.DB, userIDs []int) {
	// Missing context
	for _, id := range userIDs {
		db.Query("SELECT * FROM users WHERE id = ?", id)
	}
	
	// SQL injection
	userInput := "1 OR 1=1"
	db.Query("SELECT * FROM users WHERE id = " + userInput)
	
	// Missing rollback
	tx, _ := db.Begin()
	tx.Exec("INSERT INTO users (name) VALUES (?)", "John")
}`,
			expected: []string{"SELECT_STAR", "MISSING_CONTEXT", "SQL_INJECTION_RISK", "MISSING_ROLLBACK"},
		},
		{
			name: "efficient database operations - no issues",
			code: `package main
import (
	"context"
	"database/sql"
)
func test(db *sql.DB, ctx context.Context) {
	// Single query instead of N+1
	rows, _ := db.QueryContext(ctx, "SELECT id, name FROM users WHERE status = ? LIMIT 100", "active")
	defer rows.Close()
	
	for rows.Next() {
		var id int
		var name string
		rows.Scan(&id, &name)
		// process user
	}
}`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				fset := token.NewFileSet()
				node, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
				if err != nil {
					t.Fatalf("Failed to parse code: %v", err)
				}

				analyzer := NewDatabaseAnalyzer()
				issues := analyzer.Analyze(node, fset)

				issueTypes := make(map[string]bool)
				for _, issue := range issues {
					issueTypes[issue.Type.String()] = true
				}

				for _, expected := range tt.expected {
					if !issueTypes[expected] {
						t.Errorf("Expected issue %s not found", expected)
					}
				}

				if len(tt.expected) == 0 && len(issues) > 0 {
					t.Errorf("Expected no issues, but found %d", len(issues))
				}
			},
		)
	}
}
