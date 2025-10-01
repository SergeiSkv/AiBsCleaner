package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type DatabaseAnalyzer struct{}

func NewDatabaseAnalyzer() Analyzer {
	return &DatabaseAnalyzer{}
}

func (da *DatabaseAnalyzer) Name() string {
	return "Database Performance"
}

func (da *DatabaseAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	root, ok := node.(ast.Node)
	if !ok {
		return []*models.Issue{}
	}

	issues := make([]*models.Issue, 0, 16)

	ast.Inspect(
		root, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}

			issues = append(issues, da.analyzeFunction(fn, fset)...)
			return false
		},
	)

	return issues
}

func (da *DatabaseAnalyzer) analyzeFunction(fn *ast.FuncDecl, fset *token.FileSet) []*models.Issue {
	if fn.Body == nil {
		return nil
	}

	ctx := &dbFunctionContext{
		analyzer:     da,
		fn:           fn,
		loopRoot:     fn.Body,
		fset:         fset,
		issues:       make([]*models.Issue, 0, 8),
		transactions: make(map[string]*transactionState, 4),
		rows:         make(map[string]*resourceState, 4),
		statements:   make(map[string]*resourceState, 4),
	}

	ast.Inspect(
		fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				ctx.handleAssign(node)
			case *ast.DeferStmt:
				ctx.handleDefer(node)
			case *ast.CallExpr:
				ctx.handleCall(node)
			}
			return true
		},
	)

	ctx.finalize()
	return ctx.issues
}

type dbFunctionContext struct {
	analyzer     *DatabaseAnalyzer
	fn           *ast.FuncDecl
	loopRoot     ast.Node
	fset         *token.FileSet
	issues       []*models.Issue
	transactions map[string]*transactionState
	rows         map[string]*resourceState
	statements   map[string]*resourceState
}

type transactionState struct {
	pos              token.Position
	hasRollback      bool
	rollbackDeferred bool
	hasCommit        bool
}

type resourceState struct {
	pos    token.Position
	closed bool
}

func (ctx *dbFunctionContext) handleAssign(assign *ast.AssignStmt) {
	for idx, rhs := range assign.Rhs {
		call, ok := rhs.(*ast.CallExpr)
		if !ok {
			continue
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !ctx.analyzer.isDatabaseObject(sel.X) {
			continue
		}

		if idx >= len(assign.Lhs) {
			continue
		}

		ident, ok := assign.Lhs[idx].(*ast.Ident)
		if !ok || ident.Name == "_" {
			continue
		}

		name := ident.Name
		pos := ctx.fset.Position(call.Pos())
		method := sel.Sel.Name

		switch {
		case ctx.analyzer.isQueryReturningRows(method):
			ctx.rows[name] = &resourceState{pos: pos}

		case ctx.analyzer.isPrepareMethod(method):
			ctx.statements[name] = &resourceState{pos: pos}

		case ctx.analyzer.isTransactionBegin(method):
			ctx.transactions[name] = &transactionState{pos: pos}
		}
	}
}

func (ctx *dbFunctionContext) handleDefer(deferStmt *ast.DeferStmt) {
	call := deferStmt.Call
	if call == nil {
		return
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}

	name := ident.Name

	switch sel.Sel.Name {
	case methodClose:
		if res, exists := ctx.rows[name]; exists {
			res.closed = true
		}
		if stmt, exists := ctx.statements[name]; exists {
			stmt.closed = true
		}
	case methodRollback:
		if tx, exists := ctx.transactions[name]; exists {
			tx.hasRollback = true
			tx.rollbackDeferred = true
		}
	}
}

func (ctx *dbFunctionContext) handleCall(call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	ctx.handleResourceCall(sel)
	if !ctx.analyzer.isDatabaseObject(sel.X) {
		return
	}

	ctx.handleQueryCall(sel, call)
}

func (ctx *dbFunctionContext) handleResourceCall(sel *ast.SelectorExpr) {
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}

	name := ident.Name
	switch sel.Sel.Name {
	case methodRollback:
		if tx, exists := ctx.transactions[name]; exists {
			tx.hasRollback = true
		}
	case methodCommit:
		if tx, exists := ctx.transactions[name]; exists {
			tx.hasCommit = true
		}
	case methodClose:
		if res, exists := ctx.rows[name]; exists {
			res.closed = true
		}
		if stmt, exists := ctx.statements[name]; exists {
			stmt.closed = true
		}
	}
}

func (ctx *dbFunctionContext) handleQueryCall(sel *ast.SelectorExpr, call *ast.CallExpr) {
	method := sel.Sel.Name
	if !ctx.analyzer.isQueryMethod(method) {
		return
	}

	pos := ctx.fset.Position(call.Pos())
	if IsInLoop(ctx.loopRoot, call) {
		ctx.addIssue(
			pos,
			models.IssueSQLNPlusOne,
			models.SeverityLevelHigh,
			"Database query inside loop can trigger N+1 problems",
			"Collect identifiers first and issue a single batched query",
		)
	}

	if ctx.analyzer.hasInjectionRisk(call, method) {
		ctx.addIssue(
			pos,
			models.IssueSQLNPlusOne,
			models.SeverityLevelHigh,
			"Query text built from dynamic strings may be injectable",
			"Switch to parameter placeholders instead of string concatenation",
		)
	}

	query, ok := extractQueryLiteral(call)
	if !ok {
		return
	}
	upperQuery := strings.ToUpper(query)
	if strings.Contains(upperQuery, "SELECT *") {
		ctx.addIssue(
			pos,
			models.IssueSQLNPlusOne,
			models.SeverityLevelLow,
			"SELECT * fetches unnecessary columns",
			"List only the columns the code actually needs",
		)
	}
}

func (ctx *dbFunctionContext) finalize() {
	for name, tx := range ctx.transactions {
		if !tx.hasRollback {
			ctx.addIssue(
				tx.pos,
				models.IssueMissingDefer,
				models.SeverityLevelHigh,
				fmt.Sprintf("Transaction '%s' has no rollback protection", name),
				fmt.Sprintf("Add defer %s.Rollback() to guarantee cleanup", name),
			)
		}
	}

	for name, res := range ctx.rows {
		if !res.closed {
			ctx.addIssue(
				res.pos,
				models.IssueMissingClose,
				models.SeverityLevelMedium,
				fmt.Sprintf("Result set '%s' is not closed", name),
				fmt.Sprintf("Call defer %s.Close() after checking the error", name),
			)
		}
	}

	for name, stmt := range ctx.statements {
		if !stmt.closed {
			ctx.addIssue(
				stmt.pos,
				models.IssueMissingClose,
				models.SeverityLevelMedium,
				fmt.Sprintf("Prepared statement '%s' is not closed", name),
				fmt.Sprintf("Close the statement with defer %s.Close()", name),
			)
		}
	}
}

func (ctx *dbFunctionContext) addIssue(
	pos token.Position, issueType models.IssueType, severity models.SeverityLevel, message, suggestion string,
) {
	ctx.issues = append(
		ctx.issues, &models.Issue{
			File:       pos.Filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       issueType,
			Severity:   severity,
			Message:    message,
			Suggestion: suggestion,
		},
	)
}

func (da *DatabaseAnalyzer) isDatabaseObject(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		lower := strings.ToLower(e.Name)
		patterns := []string{"db", "conn", "tx", "stmt", "rows", "repository", "store"}
		for _, p := range patterns {
			if strings.HasPrefix(lower, p) {
				return true
			}
		}
	case *ast.SelectorExpr:
		if pkg, ok := e.X.(*ast.Ident); ok {
			pkgName := pkg.Name
			switch pkgName {
			case pkgSQL, "sqlx", "pgx", "gorm", "mongo", "redis":
				return true
			}
		}
	}
	return false
}

func (da *DatabaseAnalyzer) isQueryMethod(name string) bool {
	switch name {
	case methodQuery, methodQueryContext, methodQueryRow, methodQueryRowCtx, methodExec, methodExecContext:
		return true
	}
	return false
}

func (da *DatabaseAnalyzer) isQueryReturningRows(name string) bool {
	switch name {
	case methodQuery, methodQueryContext:
		return true
	}
	return false
}

func (da *DatabaseAnalyzer) isPrepareMethod(name string) bool {
	return name == "Prepare" || name == "PrepareContext"
}

func (da *DatabaseAnalyzer) isTransactionBegin(name string) bool {
	return name == methodBegin || name == methodBeginTx
}

func (da *DatabaseAnalyzer) hasInjectionRisk(call *ast.CallExpr, method string) bool {
	if len(call.Args) == 0 {
		return false
	}

	argIdx := 0
	if strings.HasSuffix(method, "Context") {
		argIdx = 1
	}

	if argIdx >= len(call.Args) {
		return false
	}

	return isRiskyStringExpr(call.Args[argIdx])
}

func isRiskyStringExpr(expr ast.Expr) bool {
	switch v := expr.(type) {
	case *ast.BasicLit:
		return false

	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return false
		}
		return isRiskyStringExpr(v.X) || isRiskyStringExpr(v.Y)

	case *ast.CallExpr:
		if isFmtSprintfCall(v) {
			if len(v.Args) <= 1 {
				return false
			}
			for _, arg := range v.Args[1:] {
				if !isStringLiteral(arg) {
					return true
				}
			}
			return false
		}
		return true

	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.SliceExpr:
		return true
	}

	return true
}

func isFmtSprintfCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	return pkgIdent.Name == pkgFmt && sel.Sel.Name == methodSprintf
}

func extractQueryLiteral(call *ast.CallExpr) (string, bool) {
	if len(call.Args) == 0 {
		return "", false
	}

	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}

	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		value = strings.Trim(lit.Value, "\"'`")
	}

	return value, true
}
