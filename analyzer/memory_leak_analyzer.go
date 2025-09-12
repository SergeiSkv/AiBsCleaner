package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

type MemoryLeakAnalyzer struct{}

func NewMemoryLeakAnalyzer() *MemoryLeakAnalyzer {
	return &MemoryLeakAnalyzer{}
}

func (mla *MemoryLeakAnalyzer) Name() string {
	return "MemoryLeakAnalyzer"
}

func (mla *MemoryLeakAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			issues = append(issues, mla.analyzeFunction(node, filename, fset)...)
		}
		return true
	})

	return issues
}

func (mla *MemoryLeakAnalyzer) analyzeFunction(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	openedResources := make(map[string]bool, 10)

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			for i, expr := range node.Rhs {
				if i < len(node.Lhs) {
					if ident, ok := node.Lhs[i].(*ast.Ident); ok {
						resourceName := ident.Name

						if call, ok := expr.(*ast.CallExpr); ok {
							// Check for resource opening
							if mla.isResourceOpening(call) {
								openedResources[resourceName] = true

								// Check if it's closed
								if !mla.hasResourceClose(fn.Body, resourceName) {
									pos := fset.Position(node.Pos())
									issues = append(issues, Issue{
										File:       filename,
										Line:       pos.Line,
										Column:     pos.Column,
										Position:   pos,
										Type:       "UNCLOSED_RESOURCE",
										Severity:   SeverityHigh,
										Message:    "Resource opened but not closed, potential memory leak",
										Suggestion: "Ensure resource is closed with defer or explicit close",
									})
								}
							}

							// Check for time.NewTicker
							if mla.isTicker(call) {
								if !mla.hasTickerStop(fn.Body, resourceName) {
									pos := fset.Position(node.Pos())
									issues = append(issues, Issue{
										File:       filename,
										Line:       pos.Line,
										Column:     pos.Column,
										Position:   pos,
										Type:       "UNSTOPPED_TICKER",
										Severity:   SeverityHigh,
										Message:    "time.Ticker created but not stopped, will leak memory",
										Suggestion: "Call ticker.Stop() when done, preferably with defer",
									})
								}
							}

							// Check for context.WithCancel/WithTimeout
							if mla.isContextWithCancel(call) {
								if !mla.hasContextCancel(fn.Body, resourceName) {
									pos := fset.Position(node.Pos())
									issues = append(issues, Issue{
										File:       filename,
										Line:       pos.Line,
										Column:     pos.Column,
										Position:   pos,
										Type:       "UNCANCELLED_CONTEXT",
										Severity:   SeverityMedium,
										Message:    "Context with cancel created but cancel func not called",
										Suggestion: "Call the cancel function when done, preferably with defer",
									})
								}
							}
						}
					}
				}
			}

		case *ast.GoStmt:
			// Check for goroutine leaks
			if mla.hasInfiniteLoop(node.Call) && !mla.hasExitCondition(node.Call) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "GOROUTINE_LEAK",
					Severity:   SeverityHigh,
					Message:    "Goroutine with infinite loop and no exit condition",
					Suggestion: "Add a done channel or context to allow goroutine termination",
				})
			}

		case *ast.CallExpr:
			// Check for global variable modifications that could leak
			if mla.isGlobalMapAppend(node) {
				pos := fset.Position(node.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "GLOBAL_MAP_LEAK",
					Severity:   SeverityMedium,
					Message:    "Adding to global map without cleanup can cause memory leak",
					Suggestion: "Implement cleanup mechanism or use weak references",
				})
			}
		}
		return true
	})

	// Check for circular references
	issues = append(issues, mla.checkCircularReferences(fn, filename, fset)...)

	return issues
}

func (mla *MemoryLeakAnalyzer) isResourceOpening(call *ast.CallExpr) bool {
	if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			resources := map[string][]string{
				"os":    {"Open", "Create", "OpenFile"},
				"net":   {"Dial", "Listen"},
				"http":  {"Get", "Post", "Head"},
				"sql":   {"Open"},
				"bufio": {"NewReader", "NewWriter"}, // Scanner doesn't need Close()
			}

			if methods, exists := resources[ident.Name]; exists {
				for _, method := range methods {
					if selExpr.Sel.Name == method {
						return true
					}
				}
			}
		}
	}
	return false
}

func (mla *MemoryLeakAnalyzer) hasResourceClose(block *ast.BlockStmt, resourceName string) bool {
	found := false
	ast.Inspect(block, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.DeferStmt:
			call := node.Call
			if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := selExpr.X.(*ast.Ident); ok {
					if ident.Name == resourceName && selExpr.Sel.Name == "Close" {
						found = true
					}
				}
			}
		case *ast.CallExpr:
			if selExpr, ok := node.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := selExpr.X.(*ast.Ident); ok {
					if ident.Name == resourceName && selExpr.Sel.Name == "Close" {
						found = true
					}
				}
			}
		}
		return !found
	})
	return found
}

func (mla *MemoryLeakAnalyzer) isTicker(call *ast.CallExpr) bool {
	if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			return ident.Name == "time" && selExpr.Sel.Name == "NewTicker"
		}
	}
	return false
}

func (mla *MemoryLeakAnalyzer) hasTickerStop(block *ast.BlockStmt, tickerName string) bool {
	found := false
	ast.Inspect(block, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := selExpr.X.(*ast.Ident); ok {
					if ident.Name == tickerName && selExpr.Sel.Name == "Stop" {
						found = true
					}
				}
			}
		}
		return !found
	})
	return found
}

func (mla *MemoryLeakAnalyzer) isContextWithCancel(call *ast.CallExpr) bool {
	if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := selExpr.X.(*ast.Ident); ok {
			return ident.Name == "context" &&
				(selExpr.Sel.Name == "WithCancel" ||
					selExpr.Sel.Name == "WithTimeout" ||
					selExpr.Sel.Name == "WithDeadline")
		}
	}
	return false
}

func (mla *MemoryLeakAnalyzer) hasContextCancel(block *ast.BlockStmt, ctxName string) bool {
	// Look for cancel function call (usually assigned as second return value)
	found := false
	ast.Inspect(block, func(n ast.Node) bool {
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
	})
	return found
}

func (mla *MemoryLeakAnalyzer) hasInfiniteLoop(call *ast.CallExpr) bool {
	if fn, ok := call.Fun.(*ast.FuncLit); ok {
		hasForLoop := false
		hasInfinite := false

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if forStmt, ok := n.(*ast.ForStmt); ok {
				hasForLoop = true
				if forStmt.Cond == nil {
					hasInfinite = true
				}
			}
			return true
		})

		return hasForLoop && hasInfinite
	}
	return false
}

func (mla *MemoryLeakAnalyzer) hasExitCondition(call *ast.CallExpr) bool {
	if fn, ok := call.Fun.(*ast.FuncLit); ok {
		hasExit := false

		ast.Inspect(fn.Body, func(n ast.Node) bool {
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
		})

		return hasExit
	}
	return false
}

func (mla *MemoryLeakAnalyzer) isGlobalMapAppend(call *ast.CallExpr) bool {
	// Check for map assignments to what might be global variables
	if indexExpr, ok := call.Fun.(*ast.IndexExpr); ok {
		if ident, ok := indexExpr.X.(*ast.Ident); ok {
			// Check if identifier starts with uppercase (exported/global)
			if len(ident.Name) > 0 && ident.Name[0] >= 'A' && ident.Name[0] <= 'Z' {
				return true
			}
		}
	}
	return false
}

func (mla *MemoryLeakAnalyzer) checkCircularReferences(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	// Check for struct fields that reference parent
	ast.Inspect(fn, func(n ast.Node) bool {
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if structType, ok := typeSpec.Type.(*ast.StructType); ok {
				for _, field := range structType.Fields.List {
					if starExpr, ok := field.Type.(*ast.StarExpr); ok {
						if ident, ok := starExpr.X.(*ast.Ident); ok {
							if ident.Name == typeSpec.Name.Name {
								pos := fset.Position(field.Pos())
								issues = append(issues, Issue{
									File:       filename,
									Line:       pos.Line,
									Column:     pos.Column,
									Position:   pos,
									Type:       "POTENTIAL_CIRCULAR_REF",
									Severity:   SeverityMedium,
									Message:    "Struct contains pointer to itself, potential circular reference",
									Suggestion: "Use weak references or ensure proper cleanup to avoid memory leaks",
								})
							}
						}
					}
				}
			}
		}
		return true
	})

	return issues
}
