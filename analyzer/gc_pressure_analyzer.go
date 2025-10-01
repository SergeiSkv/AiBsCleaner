package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type GCPressureAnalyzer struct{}

func NewGCPressureAnalyzer() Analyzer {
	return &GCPressureAnalyzer{}
}

func (gpa *GCPressureAnalyzer) Name() string {
	return "GCPressureAnalyzer"
}

func (gpa *GCPressureAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	visitor := &gcVisitor{
		fset:     fset,
		filename: filename,
		issues:   make([]*models.Issue, 0, 8),
	}

	ast.Walk(visitor, file)
	return visitor.issues
}

type gcVisitor struct {
	fset      *token.FileSet
	filename  string
	loopDepth int
	issues    []*models.Issue
}

func (v *gcVisitor) Visit(node ast.Node) ast.Visitor {
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
	case *ast.AssignStmt:
		v.inspectAssign(n)
	}
	return v
}

func (v *gcVisitor) inspectCall(call *ast.CallExpr) {
	if v.loopDepth == 0 {
		return
	}

	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != funcMake {
		return
	}

	if len(call.Args) == 0 {
		return
	}

	switch call.Args[0].(type) {
	case *ast.MapType:
		pos := v.fset.Position(call.Pos())
		v.issues = append(
			v.issues, &models.Issue{
				File:       v.filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       models.IssueHighGCPressure,
				Severity:   models.SeverityLevelMedium,
				Message:    "Map allocation inside loop allocates each iteration",
				Suggestion: "Move make(map) outside loop or reuse a cleared map",
			},
		)
	case *ast.ArrayType:
		if len(call.Args) >= 2 {
			if size := literalIntValue(call.Args[1]); size >= 512 {
				pos := v.fset.Position(call.Pos())
				v.issues = append(
					v.issues, &models.Issue{
						File:       v.filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       models.IssueHighGCPressure,
						Severity:   models.SeverityLevelLow,
						Message:    "Large slice allocation inside loop",
						Suggestion: "Consider reusing a buffer or allocating once",
					},
				)
			}
		}
	}
}

func (v *gcVisitor) inspectAssign(assign *ast.AssignStmt) {
	if v.loopDepth == 0 {
		return
	}

	if assign.Tok != token.ADD_ASSIGN {
		return
	}

	if len(assign.Rhs) != 1 {
		return
	}

	if isStringLiteral(assign.Rhs[0]) {
		pos := v.fset.Position(assign.Pos())
		v.issues = append(
			v.issues, &models.Issue{
				File:       v.filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       models.IssueHighGCPressure,
				Severity:   models.SeverityLevelLow,
				Message:    "String concatenation in loop allocates on every iteration",
				Suggestion: "Use strings.Builder or bytes.Buffer",
			},
		)
	}
}

func literalIntValue(expr ast.Expr) int {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return -1
	}
	var value int
	_, err := fmt.Sscanf(lit.Value, "%d", &value)
	if err != nil {
		return -1
	}
	return value
}
