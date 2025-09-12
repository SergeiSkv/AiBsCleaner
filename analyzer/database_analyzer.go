package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

type DatabaseAnalyzer struct {
	queries       []QueryInfo
	transactions  []TransactionInfo
	connections   map[string]ConnectionInfo
}

type QueryInfo struct {
	Location   token.Position
	Query      string
	InLoop     bool
	InFunction string
	Type       string // SELECT, INSERT, UPDATE, DELETE
}

type TransactionInfo struct {
	Location  token.Position
	HasCommit bool
	HasRollback bool
	InLoop    bool
}

type ConnectionInfo struct {
	Location   token.Position
	IsClosed   bool
	IsPooled   bool
	MaxConns   int
}

func NewDatabaseAnalyzer() *DatabaseAnalyzer {
	return &DatabaseAnalyzer{
		queries:     []QueryInfo{},
		connections: make(map[string]ConnectionInfo, 10),
	}
}

func (da *DatabaseAnalyzer) Name() string {
	return "DatabaseAnalyzer"
}

func (da *DatabaseAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Reset state for new file
	da.queries = []QueryInfo{}
	da.transactions = []TransactionInfo{}

	// Collect database operations
	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			da.analyzeCall(node, fset)
		case *ast.FuncDecl:
			issues = append(issues, da.analyzeFunctionDB(node, filename, fset)...)
		}
		return true
	})

	// Analyze collected queries
	issues = append(issues, da.analyzeQueries(filename, fset)...)
	issues = append(issues, da.analyzeTransactions(filename, fset)...)
	issues = append(issues, da.analyzeConnections(filename, fset)...)

	return issues
}

func (da *DatabaseAnalyzer) analyzeCall(call *ast.CallExpr, fset *token.FileSet) {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		methodName := sel.Sel.Name
		
		// Check for database query methods
		queryMethods := []string{"Query", "QueryRow", "QueryContext", "Exec", "ExecContext", "Prepare"}
		for _, method := range queryMethods {
			if methodName == method {
				da.collectQuery(call, fset)
				break
			}
		}

		// Check for transaction methods
		if methodName == "Begin" || methodName == "BeginTx" {
			da.collectTransaction(call, fset)
		}
	}
}

func (da *DatabaseAnalyzer) analyzeFunctionDB(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue
	
	if fn.Body == nil {
		return issues
	}

	var (
		queryCount      int
		hasTransaction  bool
		hasPreparedStmt bool
		queries         []string
	)

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				// Check for database operations
				if da.isDatabaseCall(sel) {
					queryCount++
					
					// Check for N+1 problem
					if da.isInLoop(node) {
						pos := fset.Position(node.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "N_PLUS_ONE_QUERY",
							Severity:   SeverityHigh,
							Message:    "Database query in loop causes N+1 problem",
							Suggestion: "Use JOIN or batch query to fetch all data at once",
						})
					}

					// Check for missing prepared statements
					if da.isQueryWithParams(node) && !hasPreparedStmt {
						if strings.Contains(sel.Sel.Name, "Query") || strings.Contains(sel.Sel.Name, "Exec") {
							if !strings.Contains(sel.Sel.Name, "Context") {
								pos := fset.Position(node.Pos())
								issues = append(issues, Issue{
									File:       filename,
									Line:       pos.Line,
									Column:     pos.Column,
									Position:   pos,
									Type:       "UNPREPARED_STATEMENT",
									Severity:   SeverityMedium,
									Message:    "Query not using prepared statement",
									Suggestion: "Use Prepare() for repeated queries or queries with parameters",
								})
							}
						}
					}

					// Check for SQL injection
					if da.hasSQLInjectionRisk(node) {
						pos := fset.Position(node.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "SQL_INJECTION_RISK",
							Severity:   SeverityHigh,
							Message:    "Potential SQL injection vulnerability - string concatenation in query",
							Suggestion: "Use parameterized queries with placeholders (? or $1, $2...)",
						})
					}

					// Collect query for analysis
					if query := da.extractQuery(node); query != "" {
						queries = append(queries, query)
					}
				}

				// Check for transactions
				if sel.Sel.Name == "Begin" || sel.Sel.Name == "BeginTx" {
					hasTransaction = true
					
					// Check for missing rollback
					if !da.hasRollback(fn.Body) {
						pos := fset.Position(node.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "MISSING_ROLLBACK",
							Severity:   SeverityHigh,
							Message:    "Transaction without proper rollback handling",
							Suggestion: "Add defer tx.Rollback() after Begin()",
						})
					}
				}

				// Check for Prepare
				if sel.Sel.Name == "Prepare" || sel.Sel.Name == "PrepareContext" {
					hasPreparedStmt = true
					
					// Check if prepared statement is closed
					if !da.hasPreparedStmtClose(fn.Body) {
						pos := fset.Position(node.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "UNCLOSED_PREPARED_STMT",
							Severity:   SeverityMedium,
							Message:    "Prepared statement not closed",
							Suggestion: "Add defer stmt.Close() after Prepare()",
						})
					}
				}

				// Check for rows.Close()
				if strings.Contains(sel.Sel.Name, "Query") && !strings.Contains(sel.Sel.Name, "QueryRow") {
					if !da.hasRowsClose(fn.Body) {
						pos := fset.Position(node.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "UNCLOSED_ROWS",
							Severity:   SeverityMedium,
							Message:    "Query rows not closed",
							Suggestion: "Add defer rows.Close() after Query()",
						})
					}
				}

				// Check for missing context
				if da.isDatabaseCall(sel) && !strings.Contains(sel.Sel.Name, "Context") {
					pos := fset.Position(node.Pos())
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "MISSING_CONTEXT",
						Severity:   SeverityLow,
						Message:    "Database operation without context",
						Suggestion: "Use QueryContext/ExecContext for timeout and cancellation support",
					})
				}
			}
		}
		return true
	})

	// Check for too many queries in one function
	if queryCount > MaxQueriesPerFunction && !hasTransaction {
		issues = append(issues, Issue{
			File:       filename,
			Line:       fset.Position(fn.Pos()).Line,
			Column:     fset.Position(fn.Pos()).Column,
			Position:   fset.Position(fn.Pos()),
			Type:       "TOO_MANY_QUERIES",
			Severity:   SeverityMedium,
			Message:    fmt.Sprintf("Function executes %d queries without transaction", queryCount),
			Suggestion: "Consider using a transaction or batch operations",
		})
	}

	// Analyze specific query patterns
	for _, query := range queries {
		issues = append(issues, da.analyzeQueryPattern(query, filename, fset, fn.Pos())...)
	}

	return issues
}

func (da *DatabaseAnalyzer) analyzeQueries(filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	// Group queries by function to detect patterns
	queryByFunc := make(map[string][]QueryInfo, 10)
	for _, q := range da.queries {
		queryByFunc[q.InFunction] = append(queryByFunc[q.InFunction], q)
	}

	// Check for multiple similar queries that could be batched
	for funcName, queries := range queryByFunc {
		if len(queries) > MaxNestedLoops {
			similarCount := da.countSimilarQueries(queries)
			if similarCount > MaxNestedLoops {
				issues = append(issues, Issue{
					File:       filename,
					Line:       queries[0].Location.Line,
					Column:     queries[0].Location.Column,
					Position:   queries[0].Location,
					Type:       "UNBATCHED_QUERIES",
					Severity:   SeverityMedium,
					Message:    fmt.Sprintf("Function '%s' has %d similar queries that could be batched", funcName, similarCount),
					Suggestion: "Use batch insert/update or UNION for multiple selects",
				})
			}
		}
	}

	return issues
}

func (da *DatabaseAnalyzer) analyzeTransactions(filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	for _, tx := range da.transactions {
		if !tx.HasCommit {
			issues = append(issues, Issue{
				File:       filename,
				Line:       tx.Location.Line,
				Column:     tx.Location.Column,
				Position:   tx.Location,
				Type:       "MISSING_COMMIT",
				Severity:   SeverityHigh,
				Message:    "Transaction started but never committed",
				Suggestion: "Ensure transaction is committed on success",
			})
		}

		if tx.InLoop {
			issues = append(issues, Issue{
				File:       filename,
				Line:       tx.Location.Line,
				Column:     tx.Location.Column,
				Position:   tx.Location,
				Type:       "TRANSACTION_IN_LOOP",
				Severity:   SeverityHigh,
				Message:    "Creating transaction inside loop",
				Suggestion: "Move transaction outside the loop",
			})
		}
	}

	return issues
}

func (da *DatabaseAnalyzer) analyzeConnections(filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	for name, conn := range da.connections {
		if !conn.IsClosed {
			issues = append(issues, Issue{
				File:       filename,
				Line:       conn.Location.Line,
				Column:     conn.Location.Column,
				Position:   conn.Location,
				Type:       "UNCLOSED_CONNECTION",
				Severity:   SeverityHigh,
				Message:    fmt.Sprintf("Database connection '%s' not closed", name),
				Suggestion: "Add defer db.Close() after opening connection",
			})
		}

		if !conn.IsPooled && conn.MaxConns == 0 {
			issues = append(issues, Issue{
				File:       filename,
				Line:       conn.Location.Line,
				Column:     conn.Location.Column,
				Position:   conn.Location,
				Type:       "UNBOUNDED_CONNECTIONS",
				Severity:   SeverityMedium,
				Message:    "Database connection pool without max connections limit",
				Suggestion: "Set db.SetMaxOpenConns() and db.SetMaxIdleConns()",
			})
		}
	}

	return issues
}

func (da *DatabaseAnalyzer) analyzeQueryPattern(query string, filename string, fset *token.FileSet, pos token.Pos) []Issue {
	var issues []Issue
	upperQuery := strings.ToUpper(query)

	// Check for SELECT * 
	if strings.Contains(upperQuery, "SELECT *") {
		issues = append(issues, Issue{
			File:       filename,
			Line:       fset.Position(pos).Line,
			Column:     fset.Position(pos).Column,
			Position:   fset.Position(pos),
			Type:       "SELECT_STAR",
			Severity:   SeverityLow,
			Message:    "SELECT * fetches unnecessary columns",
			Suggestion: "Specify only required columns to reduce data transfer",
		})
	}

	// Check for missing LIMIT in SELECT
	if strings.Contains(upperQuery, "SELECT") && !strings.Contains(upperQuery, "LIMIT") && 
	   !strings.Contains(upperQuery, "COUNT(") && !strings.Contains(upperQuery, "WHERE") {
		issues = append(issues, Issue{
			File:       filename,
			Line:       fset.Position(pos).Line,
			Column:     fset.Position(pos).Column,
			Position:   fset.Position(pos),
			Type:       "MISSING_LIMIT",
			Severity:   SeverityMedium,
			Message:    "SELECT without LIMIT may return too many rows",
			Suggestion: "Add LIMIT clause to prevent fetching excessive data",
		})
	}

	// Check for OR in WHERE clause (can prevent index usage)
	if strings.Contains(upperQuery, "WHERE") && strings.Contains(upperQuery, " OR ") {
		issues = append(issues, Issue{
			File:       filename,
			Line:       fset.Position(pos).Line,
			Column:     fset.Position(pos).Column,
			Position:   fset.Position(pos),
			Type:       "OR_IN_WHERE",
			Severity:   SeverityLow,
			Message:    "OR in WHERE clause may prevent index usage",
			Suggestion: "Consider using UNION or IN clause for better performance",
		})
	}

	// Check for LIKE with leading wildcard
	if strings.Contains(upperQuery, "LIKE '%") || strings.Contains(upperQuery, "LIKE '_%") {
		issues = append(issues, Issue{
			File:       filename,
			Line:       fset.Position(pos).Line,
			Column:     fset.Position(pos).Column,
			Position:   fset.Position(pos),
			Type:       "LEADING_WILDCARD",
			Severity:   SeverityMedium,
			Message:    "LIKE with leading wildcard prevents index usage",
			Suggestion: "Consider full-text search or redesign query pattern",
		})
	}

	return issues
}

// Helper methods

func (da *DatabaseAnalyzer) isDatabaseCall(sel *ast.SelectorExpr) bool {
	dbMethods := []string{
		"Query", "QueryRow", "QueryContext", "QueryRowContext",
		"Exec", "ExecContext", "Prepare", "PrepareContext",
		"Begin", "BeginTx", "Commit", "Rollback",
	}
	
	for _, method := range dbMethods {
		if sel.Sel.Name == method {
			return true
		}
	}
	return false
}

func (da *DatabaseAnalyzer) isInLoop(node ast.Node) bool {
	// Simplified check - would need proper AST traversal for accuracy
	return false
}

func (da *DatabaseAnalyzer) isQueryWithParams(call *ast.CallExpr) bool {
	return len(call.Args) > 1
}

func (da *DatabaseAnalyzer) hasSQLInjectionRisk(call *ast.CallExpr) bool {
	if len(call.Args) > 0 {
		// Check if first argument (query) contains string concatenation
		if binExpr, ok := call.Args[0].(*ast.BinaryExpr); ok {
			if binExpr.Op == token.ADD {
				// String concatenation detected
				return true
			}
		}
		
		// Check for fmt.Sprintf in query
		if callExpr, ok := call.Args[0].(*ast.CallExpr); ok {
			if sel, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					if ident.Name == "fmt" && sel.Sel.Name == "Sprintf" {
						return true
					}
				}
			}
		}
	}
	return false
}

func (da *DatabaseAnalyzer) extractQuery(call *ast.CallExpr) string {
	if len(call.Args) > 0 {
		if lit, ok := call.Args[0].(*ast.BasicLit); ok {
			return strings.Trim(lit.Value, "\"'`")
		}
	}
	return ""
}

func (da *DatabaseAnalyzer) hasRollback(body *ast.BlockStmt) bool {
	hasRollback := false
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "Rollback" {
					hasRollback = true
					return false
				}
			}
		}
		return true
	})
	return hasRollback
}

func (da *DatabaseAnalyzer) hasPreparedStmtClose(body *ast.BlockStmt) bool {
	hasClose := false
	ast.Inspect(body, func(n ast.Node) bool {
		if deferStmt, ok := n.(*ast.DeferStmt); ok {
			call := deferStmt.Call
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "Close" {
					hasClose = true
					return false
				}
			}
		}
		return true
	})
	return hasClose
}

func (da *DatabaseAnalyzer) hasRowsClose(body *ast.BlockStmt) bool {
	return da.hasPreparedStmtClose(body) // Same pattern
}

func (da *DatabaseAnalyzer) collectQuery(call *ast.CallExpr, fset *token.FileSet) {
	query := da.extractQuery(call)
	if query != "" {
		da.queries = append(da.queries, QueryInfo{
			Location: fset.Position(call.Pos()),
			Query:    query,
			InLoop:   da.isInLoop(call),
			Type:     da.getQueryType(query),
		})
	}
}

func (da *DatabaseAnalyzer) collectTransaction(call *ast.CallExpr, fset *token.FileSet) {
	da.transactions = append(da.transactions, TransactionInfo{
		Location: fset.Position(call.Pos()),
		InLoop:   da.isInLoop(call),
	})
}

func (da *DatabaseAnalyzer) getQueryType(query string) string {
	upperQuery := strings.ToUpper(strings.TrimSpace(query))
	if strings.HasPrefix(upperQuery, "SELECT") {
		return "SELECT"
	} else if strings.HasPrefix(upperQuery, "INSERT") {
		return "INSERT"
	} else if strings.HasPrefix(upperQuery, "UPDATE") {
		return "UPDATE"
	} else if strings.HasPrefix(upperQuery, "DELETE") {
		return "DELETE"
	}
	return "OTHER"
}

func (da *DatabaseAnalyzer) countSimilarQueries(queries []QueryInfo) int {
	typeCount := make(map[string]int, 10)
	for _, q := range queries {
		typeCount[q.Type]++
	}
	
	maxCount := 0
	for _, count := range typeCount {
		if count > maxCount {
			maxCount = count
		}
	}
	return maxCount
}