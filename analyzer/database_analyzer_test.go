package analyzer

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type dbExpectation struct {
	issueType models.IssueType
	message   string
}

func TestDatabaseAnalyzer_ReducedFalsePositives(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		expectations []dbExpectation
	}{
		{
			name: "Query in loop triggers N+1 warning",
			code: `package main
import "database/sql"
func run(db *sql.DB, ids []int) {
    for _, id := range ids {
        db.Query("SELECT name FROM users WHERE id = ?", id)
    }
}`,
			expectations: []dbExpectation{{issueType: models.IssueSQLNPlusOne, message: "N+1"}},
		},
		{
			name: "Parameterized single query remains clean",
			code: `package main
import "database/sql"
func run(db *sql.DB, id int) {
    db.Query("SELECT name FROM users WHERE id = ?", id)
}`,
			expectations: nil,
		},
		{
			name: "String concatenation is treated as risky",
			code: `package main
import "database/sql"
func run(db *sql.DB, userInput string) {
    db.Query("SELECT * FROM users WHERE id = " + userInput)
}`,
			expectations: []dbExpectation{{issueType: models.IssueSQLNPlusOne, message: "injectable"}},
		},
		{
			name: "fmt.Sprintf with parameters is flagged",
			code: `package main
import (
    "database/sql"
    "fmt"
)
func run(db *sql.DB, userInput string) {
    db.Query(fmt.Sprintf("SELECT * FROM users WHERE id = %s", userInput))
}`,
			expectations: []dbExpectation{{issueType: models.IssueSQLNPlusOne, message: "injectable"}},
		},
		{
			name: "Missing rollback surfaces issue",
			code: `package main
import "database/sql"
func run(db *sql.DB) {
    tx, _ := db.Begin()
    tx.Exec("INSERT INTO users(name) VALUES(?)", "bob")
}`,
			expectations: []dbExpectation{{issueType: models.IssueMissingDefer, message: "rollback"}},
		},
		{
			name: "Rows must be closed",
			code: `package main
import "database/sql"
func run(db *sql.DB) {
    rows, _ := db.Query("SELECT * FROM users")
    _ = rows
}`,
			expectations: []dbExpectation{{issueType: models.IssueMissingClose, message: "Result set"}},
		},
		{
			name: "Prepared statements must be closed",
			code: `package main
import "database/sql"
func run(db *sql.DB) {
    stmt, _ := db.Prepare("SELECT * FROM users WHERE id = ?")
    _, _ = stmt.Query(1)
}`,
			expectations: []dbExpectation{{issueType: models.IssueMissingClose, message: "Prepared"}},
		},
		{
			name: "Select star reported with low severity",
			code: `package main
import "database/sql"
func run(db *sql.DB) {
    db.Query("SELECT * FROM users")
}`,
			expectations: []dbExpectation{{issueType: models.IssueSQLNPlusOne, message: "SELECT *"}},
		},
		{
			name: "Proper transaction handling stays clean",
			code: `package main
import (
    "context"
    "database/sql"
)
func run(db *sql.DB, ctx context.Context) {
    tx, _ := db.BeginTx(ctx, nil)
    defer tx.Rollback()
    stmt, _ := tx.PrepareContext(ctx, "SELECT id FROM users WHERE id = ?")
    defer stmt.Close()
    rows, _ := stmt.QueryContext(ctx, 1)
    defer rows.Close()
    _ = rows
}
`,
			expectations: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, "test.go", tc.code, parser.ParseComments)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			analyzer := NewDatabaseAnalyzer()
			issues := analyzer.Analyze(node, fset)

			if len(tc.expectations) == 0 && len(issues) > 0 {
				t.Fatalf("expected no issues, found %d", len(issues))
			}

			for _, exp := range tc.expectations {
				found := false
				for _, issue := range issues {
					if issue.Type == exp.issueType && strings.Contains(strings.ToLower(issue.Message), strings.ToLower(exp.message)) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected issue %v with message containing %q, got %+v", exp.issueType, exp.message, issues)
				}
			}
		})
	}
}
