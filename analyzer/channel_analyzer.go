package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type ChannelAnalyzer struct{}

func NewChannelAnalyzer() Analyzer {
	return &ChannelAnalyzer{}
}

func (ca *ChannelAnalyzer) Name() string {
	return "Channel Patterns"
}

func (ca *ChannelAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	ctx := newChannelContext(fset, filename)
	ast.Walk(&channelVisitor{ctx: ctx}, file)
	ctx.finalize()
	return ctx.issues
}

type channelVisitor struct {
	ctx *channelContext
}

func (v *channelVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		v.ctx.leave()
		return nil
	}

	isLoop := false
	switch node.(type) {
	case *ast.ForStmt, *ast.RangeStmt:
		isLoop = true
	}

	if isLoop {
		v.ctx.enterLoop()
	} else {
		v.ctx.pushFlat()
	}

	switch n := node.(type) {
	case *ast.GenDecl:
		v.ctx.recordDecl(n)
	case *ast.AssignStmt:
		v.ctx.recordAssign(n)
	case *ast.GoStmt:
		v.ctx.markGoroutine(n)
	case *ast.SendStmt:
		v.ctx.markSend(n)
	case *ast.UnaryExpr:
		if n.Op == token.ARROW {
			v.ctx.markReceive(n)
		}
	case *ast.CallExpr:
		v.ctx.markClose(n)
	}

	return v
}

type channelContext struct {
	fset     *token.FileSet
	filename string
	issues   []*models.Issue

	loopDepth int
	stack     []bool

	chans map[string]*channelData
}

type channelData struct {
	buffered          bool
	sends             []token.Position
	receives          []token.Position
	closes            []token.Position
	goroutineSends    bool
	goroutineReceives bool
}

func newChannelContext(fset *token.FileSet, filename string) *channelContext {
	return &channelContext{
		fset:     fset,
		filename: filename,
		issues:   make([]*models.Issue, 0, 8),
		stack:    make([]bool, 0, 32),
		chans:    make(map[string]*channelData, 16),
	}
}

func (ctx *channelContext) pushFlat() {
	ctx.stack = append(ctx.stack, false)
}

func (ctx *channelContext) leave() {
	if len(ctx.stack) == 0 {
		return
	}
	last := ctx.stack[len(ctx.stack)-1]
	ctx.stack = ctx.stack[:len(ctx.stack)-1]
	if last && ctx.loopDepth > 0 {
		ctx.loopDepth--
	}
}

func (ctx *channelContext) enterLoop() {
	ctx.stack = append(ctx.stack, true)
	ctx.loopDepth++
}

func (ctx *channelContext) recordDecl(decl *ast.GenDecl) {
	for _, spec := range decl.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for i, name := range vs.Names {
			if name == nil || i >= len(vs.Values) {
				continue
			}
			call, ok := vs.Values[i].(*ast.CallExpr)
			if !ok {
				continue
			}
			if info := resolveChannel(call); info != nil {
				ctx.chans[name.Name] = info
			}
		}
	}
}

func (ctx *channelContext) recordAssign(assign *ast.AssignStmt) {
	for i, rhs := range assign.Rhs {
		call, ok := rhs.(*ast.CallExpr)
		if !ok {
			continue
		}
		if i >= len(assign.Lhs) {
			continue
		}
		ident, ok := assign.Lhs[i].(*ast.Ident)
		if !ok {
			continue
		}
		if info := resolveChannel(call); info != nil {
			ctx.chans[ident.Name] = info
		}
	}
}

func resolveChannel(call *ast.CallExpr) *channelData {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != "make" || len(call.Args) == 0 {
		return nil
	}
	if _, ok := call.Args[0].(*ast.ChanType); !ok {
		return nil
	}

	data := &channelData{}
	if len(call.Args) > 1 {
		data.buffered = true
		if blit, ok := call.Args[1].(*ast.BasicLit); ok && blit.Kind == token.INT && blit.Value == "0" {
			data.buffered = false
		}
	}
	return data
}

func (ctx *channelContext) markSend(send *ast.SendStmt) {
	ident, ok := send.Chan.(*ast.Ident)
	if !ok {
		return
	}
	ch := ctx.ensureChannel(ident.Name)
	ch.sends = append(ch.sends, ctx.fset.Position(send.Pos()))
	if ctx.loopDepth > 0 {
		ch.goroutineSends = true
	}
}

func (ctx *channelContext) markReceive(recv *ast.UnaryExpr) {
	ident, ok := recv.X.(*ast.Ident)
	if !ok {
		return
	}
	ch := ctx.ensureChannel(ident.Name)
	ch.receives = append(ch.receives, ctx.fset.Position(recv.Pos()))
	if ctx.loopDepth > 0 {
		ch.goroutineReceives = true
	}
}

func (ctx *channelContext) markClose(call *ast.CallExpr) {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != "close" || len(call.Args) == 0 {
		return
	}
	target, ok := call.Args[0].(*ast.Ident)
	if !ok {
		return
	}
	ch := ctx.ensureChannel(target.Name)
	ch.closes = append(ch.closes, ctx.fset.Position(call.Pos()))
}

func (ctx *channelContext) markGoroutine(goStmt *ast.GoStmt) {
	hasSelect := false
	ast.Inspect(goStmt.Call, func(n ast.Node) bool {
		if _, ok := n.(*ast.SelectStmt); ok {
			hasSelect = true
			return false
		}
		return true
	})

	if hasSelect {
		return
	}

	ast.Inspect(goStmt.Call, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SendStmt:
			if ident, ok := node.Chan.(*ast.Ident); ok {
				ch := ctx.ensureChannel(ident.Name)
				ch.goroutineSends = true
			}
		case *ast.UnaryExpr:
			if node.Op == token.ARROW {
				if ident, ok := node.X.(*ast.Ident); ok {
					ch := ctx.ensureChannel(ident.Name)
					ch.goroutineReceives = true
				}
			}
		}
		return true
	})
}

func (ctx *channelContext) ensureChannel(name string) *channelData {
	if data, ok := ctx.chans[name]; ok {
		return data
	}
	data := &channelData{}
	ctx.chans[name] = data
	return data
}

func (ctx *channelContext) finalize() {
	for name, ch := range ctx.chans {
		if len(ch.closes) > 1 {
			for _, pos := range ch.closes[1:] {
				ctx.addIssue(pos, models.IssueChannelMultipleClose,
					"channel '"+name+"' closed multiple times")
			}
		}

		if len(ch.closes) > 0 {
			closeLine := ch.closes[0].Line
			for _, sendPos := range ch.sends {
				if sendPos.Line > closeLine {
					ctx.addIssue(sendPos, models.IssueChannelSendOnClosed,
						"sending on closed channel '"+name+"'")
				}
			}
		}

		if !ch.buffered {
			if len(ch.sends) > 0 && len(ch.receives) == 0 {
				for _, pos := range ch.sends {
					ctx.addIssue(pos, models.IssueChannelDeadlock,
						"send on unbuffered channel without matching receive")
				}
			}

			if ch.goroutineSends || ch.goroutineReceives {
				ctx.addIssue(ch.firstOp(), models.IssueUnbufferedChannel,
					"unbuffered channel used in goroutine without select")
			}
		}

		_ = ch
	}
}

func (ch *channelData) firstOp() token.Position {
	if len(ch.sends) > 0 {
		return ch.sends[0]
	}
	if len(ch.receives) > 0 {
		return ch.receives[0]
	}
	if len(ch.closes) > 0 {
		return ch.closes[0]
	}
	return token.Position{}
}

func (ctx *channelContext) addIssue(pos token.Position, issueType models.IssueType, msg string) {
	ctx.issues = append(ctx.issues, &models.Issue{
		File:     ctx.filename,
		Line:     pos.Line,
		Column:   pos.Column,
		Position: pos,
		Type:     issueType,
		Severity: issueType.Severity(),
		Message:  msg,
	})
}
