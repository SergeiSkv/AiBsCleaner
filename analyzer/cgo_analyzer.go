package analyzer

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type CGOAnalyzer struct{}

func NewCGOAnalyzer() Analyzer {
	return &CGOAnalyzer{}
}

func (ca *CGOAnalyzer) Name() string {
	return "CGO Performance"
}

func (ca *CGOAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	if !usesCGO(file) {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	ctx := newCGOContext(fset, filename)
	ast.Walk(&cgoVisitor{ctx: ctx}, file)
	return ctx.issues
}

func usesCGO(file *ast.File) bool {
	for _, imp := range file.Imports {
		if imp.Path != nil && imp.Path.Value == `"C"` {
			return true
		}
	}
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if comment == nil {
				continue
			}
			lower := strings.ToLower(comment.Text)
			if strings.Contains(lower, "#include") || strings.Contains(lower, "#cgo") {
				return true
			}
		}
	}
	return false
}

type cgoVisitor struct {
	ctx *cgoContext
}

func (v *cgoVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		v.ctx.popFrame()
		return nil
	}

	switch node.(type) {
	case *ast.ForStmt, *ast.RangeStmt:
		v.ctx.pushFrame(true)
	default:
		v.ctx.pushFrame(false)
	}

	if call, ok := node.(*ast.CallExpr); ok {
		v.ctx.handleCall(call)
	}

	return v
}

type cgoContext struct {
	fset       *token.FileSet
	filename   string
	issues     []*models.Issue
	loopDepth  int
	frameStack []bool
}

func newCGOContext(fset *token.FileSet, filename string) *cgoContext {
	return &cgoContext{
		fset:       fset,
		filename:   filename,
		issues:     make([]*models.Issue, 0, 8),
		frameStack: make([]bool, 0, 16),
	}
}

func (ctx *cgoContext) pushFrame(isLoop bool) {
	ctx.frameStack = append(ctx.frameStack, isLoop)
	if isLoop {
		ctx.loopDepth++
	}
}

func (ctx *cgoContext) popFrame() {
	if len(ctx.frameStack) == 0 {
		return
	}
	last := ctx.frameStack[len(ctx.frameStack)-1]
	ctx.frameStack = ctx.frameStack[:len(ctx.frameStack)-1]
	if last && ctx.loopDepth > 0 {
		ctx.loopDepth--
	}
}

func (ctx *cgoContext) handleCall(call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok || pkgIdent.Name != "C" {
		return
	}

	pos := ctx.fset.Position(call.Pos())

	if ctx.loopDepth > 0 {
		severity := models.SeverityLevelMedium
		if ctx.loopDepth > 1 {
			severity = models.SeverityLevelHigh
		}
		ctx.issues = append(ctx.issues, &models.Issue{
			File:       pos.Filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       models.IssueCGOInLoop,
			Severity:   severity,
			Message:    "CGO call inside loop adds heavy crossing overhead",
			Suggestion: "Batch CGO work or move it outside the loop",
		})
	}

	if isConversionCall(sel.Sel.Name) {
		ctx.issues = append(ctx.issues, &models.Issue{
			File:       pos.Filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       models.IssueCGOMemoryLeak,
			Severity:   models.SeverityLevelMedium,
			Message:    "CGO conversion allocates and copies between Go and C",
			Suggestion: "Reuse buffers or prefer pure-Go conversions when possible",
		})
	}
}

func isConversionCall(name string) bool {
	switch name {
	case "CString", "GoString", "CBytes", "GoBytes":
		return true
	default:
		return false
	}
}
