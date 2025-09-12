package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

type MemoryLeakAnalyzer struct{}

func NewMemoryLeakAnalyzer() Analyzer {
	return &MemoryLeakAnalyzer{}
}

func (mla *MemoryLeakAnalyzer) Name() string {
	return "MemoryLeakAnalyzer"
}

func (mla *MemoryLeakAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
	var issues []*Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Get filename from the first position we encounter
	filename := ""
	if astNode.Pos().IsValid() {
		filename = fset.Position(astNode.Pos()).Filename
	}

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			if node, ok := n.(*ast.FuncDecl); ok {
				issues = append(issues, mla.analyzeFunction(node, filename, fset)...)
			}
			return true
		},
	)

	return issues
}

func (mla *MemoryLeakAnalyzer) analyzeFunction(fn *ast.FuncDecl, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	openedResources := make(map[string]bool, 10)

	ast.Inspect(
		fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				for i, expr := range node.Rhs {
					if i >= len(node.Lhs) {
						continue
					}

					ident, ok := node.Lhs[i].(*ast.Ident)
					if !ok {
						continue
					}

					resourceName := ident.Name
					call, ok := expr.(*ast.CallExpr)
					if !ok {
						continue
					}

					// Check for resource opening
					if mla.isResourceOpening(call) {
						openedResources[resourceName] = true

						// Check if it's closed
						if !mla.hasResourceClose(fn.Body, resourceName) {
							pos := fset.Position(node.Pos())
							issues = append(
								issues, &Issue{
									File:       filename,
									Line:       pos.Line,
									Column:     pos.Column,
									Position:   pos,
									Type:       IssueMemoryLeak,
									Severity:   SeverityLevelHigh,
									Message:    "Resource opened but not closed, potential memory leak",
									Suggestion: "Ensure resource is closed with defer or explicit close",
								},
							)
						}
					}

					// Check for time.NewTicker
					if mla.isTicker(call) {
						if !mla.hasTickerStop(fn.Body, resourceName) {
							pos := fset.Position(node.Pos())
							issues = append(
								issues, &Issue{
									File:       filename,
									Line:       pos.Line,
									Column:     pos.Column,
									Position:   pos,
									Type:       IssueMemoryLeak,
									Severity:   SeverityLevelHigh,
									Message:    "time.Ticker created but not stopped, will leak memory",
									Suggestion: "Call ticker.Stop() when done, preferably with defer",
								},
							)
						}
					}

					// Check for context.WithCancel/WithTimeout
					if mla.isContextWithCancel(call) {
						if !mla.hasContextCancel(fn.Body, resourceName) {
							pos := fset.Position(node.Pos())
							issues = append(
								issues, &Issue{
									File:       filename,
									Line:       pos.Line,
									Column:     pos.Column,
									Position:   pos,
									Type:       IssueMemoryLeak,
									Severity:   SeverityLevelMedium,
									Message:    "Context with cancel created but cancel func not called",
									Suggestion: "Call the cancel function when done, preferably with defer",
								},
							)
						}
					}
				}

			case *ast.GoStmt:
				// Check for goroutine leaks
				if mla.hasInfiniteLoop(node.Call) && !mla.hasExitCondition(node.Call) {
					pos := fset.Position(node.Pos())
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueMemoryLeak,
							Severity:   SeverityLevelHigh,
							Message:    "Goroutine with infinite loop and no exit condition",
							Suggestion: "Add a done channel or context to allow goroutine termination",
						},
					)
				}

			case *ast.CallExpr:
				// Check for global variable modifications that could leak
				if mla.isGlobalMapAppend(node) {
					pos := fset.Position(node.Pos())
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueMemoryLeak,
							Severity:   SeverityLevelMedium,
							Message:    "Adding to global map without cleanup can cause memory leak",
							Suggestion: "Implement cleanup mechanism or use weak references",
						},
					)
				}
			}
			return true
		},
	)

	// Check for circular references
	issues = append(issues, mla.checkCircularReferences(fn, filename, fset)...)

	return issues
}

func (mla *MemoryLeakAnalyzer) isResourceOpening(call *ast.CallExpr) bool {
	selExpr, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := selExpr.X.(*ast.Ident)
	if !ok {
		return false
	}

	resources := map[string][]string{
		"os":    {"Open", "Create", "OpenFile"},
		"net":   {"Dial", "Listen"},
		"http":  {"Get", "Post", "Head"},
		"sql":   {"Open"},
		"bufio": {"NewReader", "NewWriter"}, // Scanner doesn't need Close()
	}

	methods, exists := resources[ident.Name]
	if !exists {
		return false
	}

	for _, method := range methods {
		if selExpr.Sel.Name == method {
			return true
		}
	}
	return false
}

func (mla *MemoryLeakAnalyzer) hasResourceClose(block *ast.BlockStmt, resourceName string) bool {
	found := false
	ast.Inspect(
		block, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.DeferStmt:
				call := node.Call
				if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
					if ident, ok := selExpr.X.(*ast.Ident); ok {
						if ident.Name == resourceName && selExpr.Sel.Name == methodClose {
							found = true
						}
					}
				}
			case *ast.CallExpr:
				if selExpr, ok := node.Fun.(*ast.SelectorExpr); ok {
					if ident, ok := selExpr.X.(*ast.Ident); ok {
						if ident.Name == resourceName && selExpr.Sel.Name == methodClose {
							found = true
						}
					}
				}
			}
			return !found
		},
	)
	return found
}

func (mla *MemoryLeakAnalyzer) isTicker(call *ast.CallExpr) bool {
	if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			return ident.Name == pkgTime && selExpr.Sel.Name == "NewTicker"
		}
	}
	return false
}

func (mla *MemoryLeakAnalyzer) hasTickerStop(block *ast.BlockStmt, tickerName string) bool {
	found := false
	ast.Inspect(
		block, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			selExpr, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			ident, ok := selExpr.X.(*ast.Ident)
			if ok && ident.Name == tickerName && selExpr.Sel.Name == "Stop" {
				found = true
			}
			return !found
		},
	)
	return found
}

func (mla *MemoryLeakAnalyzer) isContextWithCancel(call *ast.CallExpr) bool {
	if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			return ident.Name == pkgContext &&
				(selExpr.Sel.Name == "WithCancel" ||
					selExpr.Sel.Name == "WithTimeout" ||
					selExpr.Sel.Name == "WithDeadline")
		}
	}
	return false
}

func (mla *MemoryLeakAnalyzer) hasContextCancel(block *ast.BlockStmt, _ string) bool {
	// Look for cancel function call (usually assigned as second return value)
	found := false
	ast.Inspect(
		block, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if ident, ok := call.Fun.(*ast.Ident); ok {
					// Check for typical cancel function names
					if strings.Contains(ident.Name, "cancel") {
						found = true
					}
				}
			}
			if deferStmt, ok := n.(*ast.DeferStmt); ok {
				call := deferStmt.Call
				if ident, ok := call.Fun.(*ast.Ident); ok {
					if strings.Contains(ident.Name, "cancel") {
						found = true
					}
				}
			}
			return !found
		},
	)
	return found
}

func (mla *MemoryLeakAnalyzer) hasInfiniteLoop(call *ast.CallExpr) bool {
	if fn, ok := call.Fun.(*ast.FuncLit); ok {
		hasForLoop := false
		hasInfinite := false

		ast.Inspect(
			fn.Body, func(n ast.Node) bool {
				if forStmt, ok := n.(*ast.ForStmt); ok {
					hasForLoop = true
					if forStmt.Cond == nil {
						hasInfinite = true
					}
				}
				return true
			},
		)

		return hasForLoop && hasInfinite
	}
	return false
}

func (mla *MemoryLeakAnalyzer) hasExitCondition(call *ast.CallExpr) bool {
	if fn, ok := call.Fun.(*ast.FuncLit); ok {
		hasExit := false

		ast.Inspect(
			fn.Body, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.SelectStmt:
					// Check for done channel or context
					for _, clause := range node.Body.List {
						if commClause, ok := clause.(*ast.CommClause); ok {
							if commClause.Comm != nil {
								hasExit = true
							}
						}
					}
				case *ast.ReturnStmt:
					hasExit = true
				case *ast.BranchStmt:
					hasExit = true
				}
				return !hasExit
			},
		)

		return hasExit
	}
	return false
}

func (mla *MemoryLeakAnalyzer) isGlobalMapAppend(call *ast.CallExpr) bool {
	// Check for map assignments to what might be global variables
	if indexExpr, ok := call.Fun.(*ast.IndexExpr); ok {
		if ident, ok := indexExpr.X.(*ast.Ident); ok {
			// Check if identifier starts with uppercase (exported/global)
			if ident.Name != "" && ident.Name[0] >= 'A' && ident.Name[0] <= 'Z' {
				return true
			}
		}
	}
	return false
}

func (mla *MemoryLeakAnalyzer) checkCircularReferences(fn *ast.FuncDecl, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	// Check for struct fields that reference parent
	ast.Inspect(
		fn, func(n ast.Node) bool {
			typeSpec, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}

			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				return true
			}

			for _, field := range structType.Fields.List {
				starExpr, ok := field.Type.(*ast.StarExpr)
				if !ok {
					continue
				}

				ident, ok := starExpr.X.(*ast.Ident)
				if !ok {
					continue
				}

				if ident.Name == typeSpec.Name.Name {
					pos := fset.Position(field.Pos())
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueMemoryLeak,
							Severity:   SeverityLevelMedium,
							Message:    "Struct contains pointer to itself, potential circular reference",
							Suggestion: "Use weak references or ensure proper cleanup to avoid memory leaks",
						},
					)
				}
			}
			return true
		},
	)

	return issues
}
