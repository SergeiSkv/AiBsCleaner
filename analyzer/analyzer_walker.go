package analyzer

import "go/ast"

// Helper functions to reduce walkWithContext complexity

func walkForStmt(n *ast.ForStmt, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	oldLoopDepth := ctx.LoopDepth
	oldInLoop := ctx.InLoop
	ctx.LoopDepth++
	ctx.InLoop = true

	if n.Init != nil {
		walkWithContext(n.Init, ctx, fn)
	}
	if n.Cond != nil {
		walkWithContext(n.Cond, ctx, fn)
	}
	if n.Post != nil {
		walkWithContext(n.Post, ctx, fn)
	}
	if n.Body != nil {
		walkWithContext(n.Body, ctx, fn)
	}

	ctx.LoopDepth = oldLoopDepth
	ctx.InLoop = oldInLoop
}

func walkRangeStmt(n *ast.RangeStmt, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	oldLoopDepth := ctx.LoopDepth
	oldInLoop := ctx.InLoop
	ctx.LoopDepth++
	ctx.InLoop = true

	if n.Key != nil {
		walkWithContext(n.Key, ctx, fn)
	}
	if n.Value != nil {
		walkWithContext(n.Value, ctx, fn)
	}
	if n.X != nil {
		walkWithContext(n.X, ctx, fn)
	}
	if n.Body != nil {
		walkWithContext(n.Body, ctx, fn)
	}

	ctx.LoopDepth = oldLoopDepth
	ctx.InLoop = oldInLoop
}

func walkFuncDecl(n *ast.FuncDecl, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	oldFunc := ctx.CurrentFunc
	if n.Name != nil {
		ctx.CurrentFunc = n.Name.Name
	}

	if n.Type != nil {
		walkWithContext(n.Type, ctx, fn)
	}
	if n.Body != nil {
		walkWithContext(n.Body, ctx, fn)
	}

	ctx.CurrentFunc = oldFunc
}

func walkIfStmt(n *ast.IfStmt, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	if n.Init != nil {
		walkWithContext(n.Init, ctx, fn)
	}
	if n.Cond != nil {
		walkWithContext(n.Cond, ctx, fn)
	}
	if n.Body != nil {
		walkWithContext(n.Body, ctx, fn)
	}
	if n.Else != nil {
		walkWithContext(n.Else, ctx, fn)
	}
}

func walkSliceExpr(n *ast.SliceExpr, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	walkWithContext(n.X, ctx, fn)
	if n.Low != nil {
		walkWithContext(n.Low, ctx, fn)
	}
	if n.High != nil {
		walkWithContext(n.High, ctx, fn)
	}
	if n.Max != nil {
		walkWithContext(n.Max, ctx, fn)
	}
}

func walkValueSpec(n *ast.ValueSpec, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	for _, name := range n.Names {
		walkWithContext(name, ctx, fn)
	}
	if n.Type != nil {
		walkWithContext(n.Type, ctx, fn)
	}
	for _, value := range n.Values {
		walkWithContext(value, ctx, fn)
	}
}

func walkCallExpr(n *ast.CallExpr, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	walkWithContext(n.Fun, ctx, fn)
	for _, arg := range n.Args {
		walkWithContext(arg, ctx, fn)
	}
}

func walkAssignStmt(n *ast.AssignStmt, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	for _, x := range n.Lhs {
		walkWithContext(x, ctx, fn)
	}
	for _, x := range n.Rhs {
		walkWithContext(x, ctx, fn)
	}
}

func walkFile(n *ast.File, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	for _, decl := range n.Decls {
		walkWithContext(decl, ctx, fn)
	}
}

func walkBlockStmt(n *ast.BlockStmt, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	for _, stmt := range n.List {
		walkWithContext(stmt, ctx, fn)
	}
}

func walkReturnStmt(n *ast.ReturnStmt, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	for _, x := range n.Results {
		walkWithContext(x, ctx, fn)
	}
}

func walkGenDecl(n *ast.GenDecl, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	for _, spec := range n.Specs {
		walkWithContext(spec, ctx, fn)
	}
}

func walkSimpleExpr(x ast.Expr, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	walkWithContext(x, ctx, fn)
}

func walkBinaryExpr(n *ast.BinaryExpr, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	walkWithContext(n.X, ctx, fn)
	walkWithContext(n.Y, ctx, fn)
}

func walkIndexExpr(n *ast.IndexExpr, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	walkWithContext(n.X, ctx, fn)
	walkWithContext(n.Index, ctx, fn)
}