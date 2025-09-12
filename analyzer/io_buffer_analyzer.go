package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type IOBufferAnalyzer struct{}

func NewIOBufferAnalyzer() Analyzer {
	return &IOBufferAnalyzer{}
}

func (ia *IOBufferAnalyzer) Name() string {
	return "I/O Buffer Efficiency"
}

func (ia *IOBufferAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	visitor := &ioVisitor{
		fset:     fset,
		filename: filename,
		issues:   make([]*models.Issue, 0, 8),
	}

	ast.Walk(visitor, file)
	return visitor.issues
}

type ioVisitor struct {
	fset      *token.FileSet
	filename  string
	loopDepth int
	issues    []*models.Issue
}

func (v *ioVisitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.ForStmt:
		v.loopDepth++
		if n.Body != nil {
			ast.Walk(v, n.Body)
		}
		v.loopDepth--
		return nil
	case *ast.RangeStmt:
		v.loopDepth++
		if n.Body != nil {
			ast.Walk(v, n.Body)
		}
		v.loopDepth--
		return nil
	case *ast.CallExpr:
		v.inspectCall(n)
	}
	return v
}

func (v *ioVisitor) inspectCall(call *ast.CallExpr) {
	if v.loopDepth == 0 {
		return
	}

	if matchSelector(call, "os", "ReadFile") || matchSelector(call, "os", "WriteFile") {
		v.addIssue(call, models.IssueUnbufferedIO, models.SeverityLevelMedium,
			"os.ReadFile/WriteFile inside loop repeatedly touches disk",
			"Open the file once or stream the content instead of calling inside the loop")
		return
	}

	if matchSelector(call, "ioutil", "ReadFile") || matchSelector(call, "ioutil", "WriteFile") {
		v.addIssue(call, models.IssueUnbufferedIO, models.SeverityLevelMedium,
			"ioutil.ReadFile/WriteFile inside loop causes repeated allocations",
			"Move ReadFile/WriteFile outside the loop or reuse buffers")
		return
	}

	if matchSelector(call, "io", "ReadAll") || matchSelector(call, "io", "Copy") {
		v.addIssue(call, models.IssueUnbufferedIO, models.SeverityLevelLow,
			"io.ReadAll/io.Copy inside loop can create garbage and block",
			"Reuse buffers or copy fixed-sized chunks instead of calling each iteration")
	}
}

func (v *ioVisitor) addIssue(call *ast.CallExpr, issueType models.IssueType, sev models.SeverityLevel, msg, suggestion string) {
	pos := v.fset.Position(call.Pos())
	v.issues = append(v.issues, &models.Issue{
		File:       v.filename,
		Line:       pos.Line,
		Column:     pos.Column,
		Position:   pos,
		Type:       issueType,
		Severity:   sev,
		Message:    msg,
		Suggestion: suggestion,
	})
}

func matchSelector(call *ast.CallExpr, pkg, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == pkg && sel.Sel.Name == name
}
