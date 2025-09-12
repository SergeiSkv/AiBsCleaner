package analyzer

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

// APIMisuseAnalyzer focuses on high signal API pitfalls (waitgroup misuse, expensive calls in loops, etc.).
type APIMisuseAnalyzer struct{}

func NewAPIMisuseAnalyzer() Analyzer {
	return &APIMisuseAnalyzer{}
}

func (a *APIMisuseAnalyzer) Name() string {
	return "API Misuse"
}

func (a *APIMisuseAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	ctx := newAPIContext(fset, filename)
	ast.Walk(&apiVisitor{ctx: ctx}, file)
	return ctx.issues
}

type apiVisitor struct {
	ctx *apiContext
}

func (v *apiVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		v.ctx.popState()
		return nil
	}

	switch n := node.(type) {
	case *ast.FuncDecl:
		v.visitFuncDecl(n)
		return nil
	case *ast.FuncLit:
		v.visitFuncLit(n)
		return nil
	case *ast.RangeStmt:
		v.visitRangeStmt(n)
		return nil
	case *ast.ForStmt:
		v.visitForStmt(n)
		return nil
	case *ast.DeferStmt:
		if v.visitDeferStmt(n) {
			return nil
		}
	case *ast.GoStmt:
		if v.visitGoStmt(n) {
			return nil
		}
	case *ast.ValueSpec:
		v.ctx.recordValueSpec(n)
	case *ast.CallExpr:
		v.ctx.handleCall(n)
	case *ast.GenDecl:
		// value specs handled when walking deeper
	default:
		// no-op
	}

	v.ctx.pushCopy()
	return v
}

func (v *apiVisitor) visitFuncDecl(n *ast.FuncDecl) {
	v.ctx.recordFuncParams(n)
	v.ctx.detectMutexParams(n)
	if n.Body == nil {
		return
	}
	pop := v.ctx.pushScope(false, false, false)
	v.ctx.funcDepth++
	ast.Walk(v, n.Body)
	pop()
	v.ctx.funcDepth--
}

func (v *apiVisitor) visitFuncLit(n *ast.FuncLit) {
	if n.Body == nil {
		return
	}
	pop := v.ctx.pushScope(v.ctx.state.inLoop, v.ctx.state.inDefer, v.ctx.state.inGoroutine)
	v.ctx.funcDepth++
	ast.Walk(v, n.Body)
	pop()
	v.ctx.funcDepth--
}

func (v *apiVisitor) visitRangeStmt(n *ast.RangeStmt) {
	pop := v.ctx.pushScope(true, false, false)
	if n.Key != nil {
		ast.Walk(v, n.Key)
	}
	if n.Value != nil {
		ast.Walk(v, n.Value)
	}
	if n.X != nil {
		ast.Walk(v, n.X)
	}
	if n.Body != nil {
		ast.Walk(v, n.Body)
	}
	pop()
}

func (v *apiVisitor) visitForStmt(n *ast.ForStmt) {
	pop := v.ctx.pushScope(true, false, false)
	if n.Init != nil {
		ast.Walk(v, n.Init)
	}
	if n.Cond != nil {
		ast.Walk(v, n.Cond)
	}
	if n.Post != nil {
		ast.Walk(v, n.Post)
	}
	if n.Body != nil {
		ast.Walk(v, n.Body)
	}
	pop()
}

func (v *apiVisitor) visitDeferStmt(n *ast.DeferStmt) bool {
	if n.Call == nil {
		return false
	}
	pop := v.ctx.pushScope(v.ctx.state.inLoop, true, v.ctx.state.inGoroutine)
	ast.Walk(v, n.Call)
	pop()
	return true
}

func (v *apiVisitor) visitGoStmt(n *ast.GoStmt) bool {
	if n.Call == nil {
		return false
	}
	pop := v.ctx.pushScope(v.ctx.state.inLoop, v.ctx.state.inDefer, true)
	ast.Walk(v, n.Call)
	pop()
	return true
}

type apiState struct {
	inLoop      bool
	inDefer     bool
	inGoroutine bool
}

type apiContext struct {
	fset     *token.FileSet
	filename string
	issues   []*models.Issue

	stateStack []apiState
	state      apiState
	funcDepth  int

	types *typeTable
}

func newAPIContext(fset *token.FileSet, filename string) *apiContext {
	ctx := &apiContext{
		fset:       fset,
		filename:   filename,
		issues:     make([]*models.Issue, 0, 16),
		stateStack: make([]apiState, 0, 8),
		types:      newTypeTable(),
	}
	ctx.stateStack = append(ctx.stateStack, apiState{})
	ctx.state = apiState{}
	return ctx
}

func (ctx *apiContext) pushScope(loop, deferCtx, goroutine bool) func() {
	ctx.stateStack = append(ctx.stateStack, apiState{
		inLoop:      loop,
		inDefer:     deferCtx,
		inGoroutine: goroutine,
	})
	ctx.state = ctx.stateStack[len(ctx.stateStack)-1]
	return ctx.popState
}

func (ctx *apiContext) pushCopy() {
	ctx.stateStack = append(ctx.stateStack, ctx.state)
	ctx.state = ctx.stateStack[len(ctx.stateStack)-1]
}

func (ctx *apiContext) popState() {
	if len(ctx.stateStack) == 0 {
		return
	}
	ctx.stateStack = ctx.stateStack[:len(ctx.stateStack)-1]
	if len(ctx.stateStack) == 0 {
		ctx.state = apiState{}
		ctx.stateStack = append(ctx.stateStack, ctx.state)
		return
	}
	ctx.state = ctx.stateStack[len(ctx.stateStack)-1]
}

func (ctx *apiContext) inDeferContext() bool {
	if ctx.state.inDefer {
		return true
	}
	for i := len(ctx.stateStack) - 1; i >= 0; i-- {
		if ctx.stateStack[i].inDefer {
			return true
		}
	}
	return false
}

func (ctx *apiContext) recordValueSpec(spec *ast.ValueSpec) {
	typ := canonicalType(spec.Type)
	if typ == "" {
		return
	}
	for _, name := range spec.Names {
		if name == nil || name.Obj == nil {
			continue
		}
		ctx.types.record(name.Obj.Pos(), typ)
	}
}

func (ctx *apiContext) recordFuncParams(fn *ast.FuncDecl) {
	if fn.Type == nil || fn.Type.Params == nil {
		return
	}
	for _, field := range fn.Type.Params.List {
		typ := canonicalType(field.Type)
		if typ == "" {
			continue
		}
		for _, name := range field.Names {
			if name == nil || name.Obj == nil {
				continue
			}
			ctx.types.record(name.Obj.Pos(), typ)
		}
	}
}

func (ctx *apiContext) detectMutexParams(fn *ast.FuncDecl) {
	if fn.Type == nil || fn.Type.Params == nil {
		return
	}

	for _, field := range fn.Type.Params.List {
		if field == nil {
			continue
		}

		// Only flag direct sync.Mutex / sync.RWMutex (passing by value)
		sel, ok := field.Type.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok || pkgIdent.Name != pkgSync {
			continue
		}

		if sel.Sel.Name != typeMutex && sel.Sel.Name != typeRWMutex {
			continue
		}

		pos := ctx.fset.Position(field.Pos())
		issue := ctx.newIssue(pos, models.IssueMutexByValue, models.SeverityLevelHigh,
			"sync.Mutex passed by value - copying a mutex breaks locking semantics",
			"Accept *sync.Mutex or *sync.RWMutex instead of a value copy")

		ctx.issues = append(ctx.issues, issue)
	}
}

func (ctx *apiContext) handleCall(call *ast.CallExpr) {
	pos := ctx.fset.Position(call.Pos())

	if issue := ctx.detectWaitGroupAdd(call, pos); issue != nil {
		ctx.issues = append(ctx.issues, issue)
	}
	if issue := ctx.detectTimeMisuse(call, pos); issue != nil {
		ctx.issues = append(ctx.issues, issue)
	}
	if issue := ctx.detectFmtConcat(call, pos); issue != nil {
		ctx.issues = append(ctx.issues, issue)
	}
	if issue := ctx.detectPprofMisuse(call, pos); issue != nil {
		ctx.issues = append(ctx.issues, issue)
	}
	if issue := ctx.detectRecoverMisuse(call, pos); issue != nil {
		ctx.issues = append(ctx.issues, issue)
	}
	if issue := ctx.detectJSONMarshal(call, pos); issue != nil {
		ctx.issues = append(ctx.issues, issue)
	}
	ctx.issues = append(ctx.issues, ctx.detectRegexIssues(call, pos)...)
}

func (ctx *apiContext) detectWaitGroupAdd(call *ast.CallExpr, pos token.Position) *models.Issue {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != methodAdd {
		return nil
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Obj == nil {
		return nil
	}

	if !isWaitGroupType(ctx.types.lookup(ident.Obj.Pos())) {
		return nil
	}

	if !ctx.state.inGoroutine {
		return nil
	}

	return ctx.newIssue(pos, models.IssueWaitgroupAddInGoroutine, models.SeverityLevelHigh,
		"WaitGroup.Add called from goroutine - call Add before starting goroutines",
		"Increment the WaitGroup counter before launching the goroutine")
}

func (ctx *apiContext) detectTimeMisuse(call *ast.CallExpr, pos token.Position) *models.Issue {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok || pkgIdent.Name != pkgTime {
		return nil
	}

	if !ctx.state.inLoop || ctx.funcDepth == 0 {
		return nil
	}

	switch sel.Sel.Name {
	case "Sleep":
		return ctx.newIssue(pos, models.IssueSleepInLoop, models.SeverityLevelMedium,
			"time.Sleep in loop blocks the entire iteration",
			"Use a time.Ticker or rate limiter outside the loop")
	case "Now":
		return ctx.newIssue(pos, models.IssueTimeNowInLoop, models.SeverityLevelMedium,
			"time.Now called in loop - repeated syscalls",
			"Capture time once before the loop or reuse a ticker")
	default:
		return nil
	}
}

func (ctx *apiContext) detectFmtConcat(call *ast.CallExpr, pos token.Position) *models.Issue {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok || pkgIdent.Name != "fmt" || sel.Sel.Name != "Sprintf" {
		return nil
	}
	if ctx.funcDepth == 0 || len(call.Args) != 2 {
		return nil
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || !strings.EqualFold(strings.Trim(lit.Value, `"`), "%s%s") {
		return nil
	}

	return ctx.newIssue(pos, models.IssueSprintfConcatenation, models.SeverityLevelLow,
		"fmt.Sprintf used for simple concatenation",
		"Use the + operator or strings.Builder for simple joins")
}

func (ctx *apiContext) detectPprofMisuse(call *ast.CallExpr, pos token.Position) *models.Issue {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "StartCPUProfile" {
		return nil
	}

	if len(call.Args) == 0 {
		return nil
	}
	if ident, ok := call.Args[0].(*ast.Ident); !ok || ident.Name != "nil" {
		return nil
	}

	return ctx.newIssue(pos, models.IssuePprofNilWriter, models.SeverityLevelHigh,
		"pprof.StartCPUProfile called with nil writer",
		"Pass an io.Writer (e.g. os.Create) instead of nil")
}

func (ctx *apiContext) detectRecoverMisuse(call *ast.CallExpr, pos token.Position) *models.Issue {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != funcRecover {
		return nil
	}
	if ctx.inDeferContext() {
		return nil
	}

	return ctx.newIssue(pos, models.IssueRecoverWithoutDefer, models.SeverityLevelHigh,
		"recover must be called from within a deferred function",
		"Wrap recover in a deferred closure")
}

func (ctx *apiContext) detectJSONMarshal(call *ast.CallExpr, pos token.Position) *models.Issue {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok || pkgIdent.Name != "json" {
		return nil
	}

	if sel.Sel.Name != "Marshal" && sel.Sel.Name != "MarshalIndent" {
		return nil
	}

	if !ctx.state.inLoop || ctx.funcDepth == 0 {
		return nil
	}

	return ctx.newIssue(pos, models.IssueJSONMarshalInLoop, models.SeverityLevelHigh,
		"encoding/json marshaling in loop allocates every iteration",
		"Move marshaling outside the loop or reuse an encoder")
}

func (ctx *apiContext) detectRegexIssues(call *ast.CallExpr, pos token.Position) []*models.Issue {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok || pkgIdent.Name != "regexp" {
		return nil
	}

	if sel.Sel.Name != "Compile" && sel.Sel.Name != "MustCompile" {
		return nil
	}

	if ctx.state.inLoop {
		return []*models.Issue{
			ctx.newIssue(pos, models.IssueRegexCompileInLoop, models.SeverityLevelHigh,
				"regexp compile in loop is extremely expensive",
				"Compile the regexp once and reuse it"),
		}
	}

	if ctx.funcDepth == 0 {
		return nil
	}

	return []*models.Issue{
		ctx.newIssue(pos, models.IssueRegexCompileInFunc, models.SeverityLevelMedium,
			"regexp compiled inside function - runs on every call",
			"Move regexp.MustCompile to package scope or cache it"),
	}
}

func (ctx *apiContext) newIssue(pos token.Position, issueType models.IssueType, severity models.SeverityLevel, message, suggestion string) *models.Issue {
	return &models.Issue{
		File:       ctx.filename,
		Line:       pos.Line,
		Column:     pos.Column,
		Position:   pos,
		Type:       issueType,
		Severity:   severity,
		Message:    message,
		Suggestion: suggestion,
	}
}
