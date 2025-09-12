package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

const backgroundFunc = "Background"

// ConcurrencyPatternsAnalyzer focuses on high-signal concurrency mistakes.
type ConcurrencyPatternsAnalyzer struct{}

func NewConcurrencyPatternsAnalyzer() Analyzer {
	return &ConcurrencyPatternsAnalyzer{}
}

func (cpa *ConcurrencyPatternsAnalyzer) Name() string {
	return "ConcurrencyPatternsAnalyzer"
}

func (cpa *ConcurrencyPatternsAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	ctx := newConcurrencyContext(fset, filename)
	ast.Walk(&concurrencyVisitor{ctx: ctx}, file)
	return ctx.issues
}

type concurrencyVisitor struct {
	ctx *concurrencyContext
}

func (v *concurrencyVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		v.ctx.popNode()
		return nil
	}

	v.ctx.pushNode(node)

	switch n := node.(type) {
	case *ast.RangeStmt:
		v.ctx.pushLoop(n)
	case *ast.ForStmt:
		v.ctx.pushLoop(n)
	case *ast.ValueSpec:
		v.ctx.recordValueSpec(n)
	case *ast.FuncDecl:
		v.ctx.recordFuncParams(n)
	case *ast.GoStmt:
		v.ctx.handleGoStmt(n)
	case *ast.CallExpr:
		v.ctx.handleCallExpr(n)
	}

	return v
}

type concurrencyContext struct {
	fset     *token.FileSet
	filename string
	issues   []*models.Issue
	types    *typeTable

	nodeStack []ast.Node
	loopStack []loopFrame
}

type loopFrame struct {
	node ast.Node
	vars map[token.Pos]struct{}
}

func newConcurrencyContext(fset *token.FileSet, filename string) *concurrencyContext {
	return &concurrencyContext{
		fset:      fset,
		filename:  filename,
		issues:    make([]*models.Issue, 0, 16),
		types:     newTypeTable(),
		nodeStack: make([]ast.Node, 0, 32),
		loopStack: make([]loopFrame, 0, 8),
	}
}

func (ctx *concurrencyContext) pushNode(node ast.Node) {
	ctx.nodeStack = append(ctx.nodeStack, node)
}

func (ctx *concurrencyContext) popNode() {
	if len(ctx.nodeStack) == 0 {
		return
	}

	node := ctx.nodeStack[len(ctx.nodeStack)-1]
	ctx.nodeStack = ctx.nodeStack[:len(ctx.nodeStack)-1]

	switch node.(type) {
	case *ast.RangeStmt, *ast.ForStmt:
		ctx.popLoop()
	}
}

func (ctx *concurrencyContext) pushLoop(node ast.Node) {
	frame := loopFrame{node: node}

	if rng, ok := node.(*ast.RangeStmt); ok {
		vars := make(map[token.Pos]struct{}, 2)
		if ident, ok := rng.Key.(*ast.Ident); ok && ident.Obj != nil {
			vars[ident.Obj.Pos()] = struct{}{}
		}
		if ident, ok := rng.Value.(*ast.Ident); ok && ident.Obj != nil {
			vars[ident.Obj.Pos()] = struct{}{}
		}
		frame.vars = vars
	}

	ctx.loopStack = append(ctx.loopStack, frame)
}

func (ctx *concurrencyContext) popLoop() {
	if len(ctx.loopStack) == 0 {
		return
	}
	ctx.loopStack = ctx.loopStack[:len(ctx.loopStack)-1]
}

func (ctx *concurrencyContext) insideLoop() bool {
	return len(ctx.loopStack) > 0
}

func (ctx *concurrencyContext) currentLoopVars() map[token.Pos]struct{} {
	if len(ctx.loopStack) == 0 {
		return nil
	}
	return ctx.loopStack[len(ctx.loopStack)-1].vars
}

func (ctx *concurrencyContext) recordValueSpec(spec *ast.ValueSpec) {
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

func (ctx *concurrencyContext) recordFuncParams(fn *ast.FuncDecl) {
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

func (ctx *concurrencyContext) handleGoStmt(stmt *ast.GoStmt) {
	if stmt.Call == nil {
		return
	}

	fn, ok := stmt.Call.Fun.(*ast.FuncLit)
	if !ok || fn.Body == nil {
		return
	}

	if ctx.loopCapturesRangeVar(fn.Body) {
		ctx.addIssue(stmt.Pos(), models.IssueGoroutineCapturesLoop, models.SeverityLevelHigh,
			"goroutine captures loop variable", "Copy the loop variable inside the goroutine or pass it as an argument")
	}

	if pos, ok := findContextBackground(fn.Body); ok {
		ctx.addIssue(pos, models.IssueContextBackgroundInGoroutine, models.SeverityLevelMedium,
			"goroutine starts with context.Background()", "Propagate the parent context instead of creating a background context")
	}
}

func (ctx *concurrencyContext) handleCallExpr(call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Obj == nil {
		return
	}

	typ := ctx.types.lookup(ident.Obj.Pos())
	if sel.Sel.Name == methodAdd {
		if isWaitGroupType(typ) && ctx.insideLoop() {
			ctx.addIssue(call.Pos(), models.IssueWaitGroupAddInLoop, models.SeverityLevelMedium,
				"WaitGroup.Add inside loop", "Call Add once before the loop and use Done inside goroutines")
		}
	}
}

func (ctx *concurrencyContext) loopCapturesRangeVar(body *ast.BlockStmt) bool {
	vars := ctx.currentLoopVars()
	if len(vars) == 0 {
		return false
	}

	captured := false
	ast.Inspect(body, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok || ident.Obj == nil {
			return true
		}
		if _, exists := vars[ident.Obj.Pos()]; exists {
			captured = true
			return false
		}
		return true
	})

	return captured
}

func (ctx *concurrencyContext) addIssue(pos token.Pos, issueType models.IssueType, severity models.SeverityLevel, message, suggestion string) {
	position := ctx.fset.Position(pos)
	ctx.issues = append(ctx.issues, &models.Issue{
		File:       ctx.filename,
		Line:       position.Line,
		Column:     position.Column,
		Position:   position,
		Type:       issueType,
		Severity:   severity,
		Message:    message,
		Suggestion: suggestion,
	})
}

func findContextBackground(body *ast.BlockStmt) (token.Pos, bool) {
	var hit token.Pos

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}

		if ident.Name == pkgContext && sel.Sel.Name == backgroundFunc {
			hit = call.Pos()
			return false
		}

		return true
	})

	if hit == token.NoPos {
		return token.NoPos, false
	}
	return hit, true
}
