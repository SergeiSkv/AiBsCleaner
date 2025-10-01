package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type CPUCacheAnalyzer struct{}

func NewCPUCacheAnalyzer() Analyzer {
	return &CPUCacheAnalyzer{}
}

func (c *CPUCacheAnalyzer) Name() string {
	return "CPUCache"
}

func (c *CPUCacheAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	issues := make([]*models.Issue, 0, 4)

	ast.Inspect(file, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}

		if issue := analyzeStructForFalseSharing(ts.Name.Name, st, fset, filename); issue != nil {
			issues = append(issues, issue)
		}
		return true
	})

	return issues
}

func analyzeStructForFalseSharing(name string, st *ast.StructType, fset *token.FileSet, filename string) *models.Issue {
	if st.Fields == nil {
		return nil
	}

	concurrentFields := 0
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		if isConcurrentType(field.Type) {
			concurrentFields++
			if concurrentFields > 1 {
				pos := fset.Position(st.Pos())
				return &models.Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       models.IssueCacheFalseSharing,
					Severity:   models.SeverityLevelLow,
					Message:    fmt.Sprintf("Struct %s contains multiple sync/atomic fields; consider separating them to avoid false sharing", name),
					Suggestion: "Split the struct into per-goroutine parts or add padding between sync/atomic fields",
				}
			}
		}
	}

	return nil
}

func isConcurrentType(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	if pkg.Name != "sync" && pkg.Name != "atomic" {
		return false
	}

	switch sel.Sel.Name {
	case "Mutex", "RWMutex", "WaitGroup", "Cond", "Value", "Bool",
		"Int32", "Int64", "Uint32", "Uint64", "Pointer":
		return true
	default:
		return false
	}
}
