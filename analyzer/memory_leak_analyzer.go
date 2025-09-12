package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type MemoryLeakAnalyzer struct{}

func NewMemoryLeakAnalyzer() Analyzer {
	return &MemoryLeakAnalyzer{}
}

func (mla *MemoryLeakAnalyzer) Name() string {
	return "Memory Leak Detection"
}

func (mla *MemoryLeakAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	visitor := &memoryVisitor{
		fset:     fset,
		filename: filename,
		issues:   make([]*models.Issue, 0, 8),
	}

	ast.Walk(visitor, file)
	return visitor.issues
}

type memoryVisitor struct {
	fset       *token.FileSet
	filename   string
	issues     []*models.Issue
	nodeStack  []ast.Node
	blockStack []*ast.BlockStmt
}

func (v *memoryVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		if len(v.nodeStack) > 0 {
			popped := v.nodeStack[len(v.nodeStack)-1]
			v.nodeStack = v.nodeStack[:len(v.nodeStack)-1]
			if _, ok := popped.(*ast.BlockStmt); ok && len(v.blockStack) > 0 {
				v.blockStack = v.blockStack[:len(v.blockStack)-1]
			}
		}
		return nil
	}

	v.nodeStack = append(v.nodeStack, node)
	if block, ok := node.(*ast.BlockStmt); ok {
		v.blockStack = append(v.blockStack, block)
	}

	if assign, ok := node.(*ast.AssignStmt); ok {
		v.inspectAssign(assign)
	}
	return v
}

func (v *memoryVisitor) inspectAssign(assign *ast.AssignStmt) {
	if len(assign.Lhs) == 0 {
		return
	}

	ident, ok := assign.Lhs[0].(*ast.Ident)
	if !ok || ident.Name == "_" {
		return
	}

	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok {
		return
	}

	if !isResourceConstructor(call) {
		return
	}

	block := v.currentBlock()
	if hasClose(block, ident.Name) {
		return
	}

	pos := v.fset.Position(assign.Pos())
	v.issues = append(v.issues, &models.Issue{
		File:       v.filename,
		Line:       pos.Line,
		Column:     pos.Column,
		Position:   pos,
		Type:       models.IssueMemoryLeak,
		Severity:   models.SeverityLevelMedium,
		Message:    "Resource opened without corresponding Close",
		Suggestion: "Call defer resource.Close() or close explicitly on all paths",
	})
}

func isResourceConstructor(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	switch pkgIdent.Name {
	case pkgOS:
		switch sel.Sel.Name {
		case methodOpen, methodOpenFile, methodCreate, methodNewFile:
			return true
		}
	case pkgNet:
		switch sel.Sel.Name {
		case methodDial, methodListen, methodListenTCP, methodListenUDP:
			return true
		}
	case pkgHTTP:
		if sel.Sel.Name == "NewRequest" {
			return true
		}
	case pkgSQL:
		if sel.Sel.Name == methodOpen {
			return true
		}
	case "os/exec":
		if sel.Sel.Name == "Command" {
			return true
		}
	}

	return false
}

func hasClose(block *ast.BlockStmt, name string) bool {
	if block == nil {
		return false
	}

	found := false
	ast.Inspect(block, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != name {
			return true
		}
		if sel.Sel.Name == methodClose {
			found = true
			return false
		}
		return true
	})
	return found
}

func (v *memoryVisitor) currentBlock() *ast.BlockStmt {
	if len(v.blockStack) == 0 {
		return nil
	}
	return v.blockStack[len(v.blockStack)-1]
}
