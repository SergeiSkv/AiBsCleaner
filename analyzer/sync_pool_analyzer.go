package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// SyncPoolAnalyzer detects opportunities for sync.Pool usage and misuse
type SyncPoolAnalyzer struct{}

func NewSyncPoolAnalyzer() *SyncPoolAnalyzer {
	return &SyncPoolAnalyzer{}
}

func (spa *SyncPoolAnalyzer) Name() string {
	return "SyncPoolAnalyzer"
}

func (spa *SyncPoolAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Track sync.Pool usage
	poolVars := make(map[string]bool)

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.GenDecl:
			// Find sync.Pool declarations
			if node.Tok == token.VAR {
				for _, spec := range node.Specs {
					if vspec, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range vspec.Names {
							if spa.isSyncPool(vspec.Type) {
								poolVars[name.Name] = true
							}
						}
					}
				}
			}

		case *ast.FuncDecl:
			if node.Body != nil {
				issues = append(issues, spa.analyzeFunction(node, filename, fset, poolVars)...)
			}

		case *ast.CallExpr:
			issues = append(issues, spa.analyzePoolUsage(node, filename, fset, poolVars)...)
		}
		return true
	})

	return issues
}

func (spa *SyncPoolAnalyzer) analyzeFunction(fn *ast.FuncDecl, filename string, fset *token.FileSet, poolVars map[string]bool) []Issue {
	var issues []Issue

	// Check for frequent allocations that could use sync.Pool
	allocCount := 0
	var allocTypes []string

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			// Check for make() calls
			if ident, ok := node.Fun.(*ast.Ident); ok && ident.Name == "make" {
				allocCount++
				if len(node.Args) > 0 {
					if t := spa.getTypeString(node.Args[0]); t != "" {
						allocTypes = append(allocTypes, t)
					}
				}
			}

			// Check for new() calls
			if ident, ok := node.Fun.(*ast.Ident); ok && ident.Name == "new" {
				allocCount++
			}

		case *ast.CompositeLit:
			// Check for struct literals
			allocCount++
		}
		return true
	})

	// If function has many allocations and is likely hot path
	if allocCount > 3 && spa.isHotPath(fn.Name.Name) {
		pos := fset.Position(fn.Pos())
		issues = append(issues, Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       "MISSING_SYNC_POOL",
			Severity:   SeverityMedium,
			Message:    "Frequent allocations in hot path - consider sync.Pool",
			Suggestion: "Use sync.Pool to reuse objects and reduce GC pressure",
		})
	}

	// Check for buffer allocations
	issues = append(issues, spa.checkBufferAllocations(fn, filename, fset)...)

	// Check for improper pool usage
	issues = append(issues, spa.checkImproperPoolUsage(fn, filename, fset, poolVars)...)

	return issues
}

func (spa *SyncPoolAnalyzer) analyzePoolUsage(call *ast.CallExpr, filename string, fset *token.FileSet, poolVars map[string]bool) []Issue {
	var issues []Issue

	// Check for sync.Pool.Get without Put
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			if poolVars[ident.Name] && sel.Sel.Name == "Get" {
				// Check if there's a corresponding Put
				if !spa.hasCorrespondingPut(call, ident.Name) {
					pos := fset.Position(call.Pos())
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "POOL_GET_WITHOUT_PUT",
						Severity:   SeverityHigh,
						Message:    "sync.Pool.Get() without corresponding Put()",
						Suggestion: "Always return objects to pool with defer pool.Put(obj)",
					})
				}
			}

			// Check for Put with nil
			if poolVars[ident.Name] && sel.Sel.Name == "Put" {
				if len(call.Args) > 0 {
					if ident, ok := call.Args[0].(*ast.Ident); ok && ident.Name == "nil" {
						pos := fset.Position(call.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "POOL_PUT_NIL",
							Severity:   SeverityMedium,
							Message:    "Putting nil into sync.Pool",
							Suggestion: "Don't put nil values into pool",
						})
					}
				}
			}
		}
	}

	return issues
}

func (spa *SyncPoolAnalyzer) checkBufferAllocations(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			// Check for bytes.Buffer allocations
			if spa.isBytesBufferAllocation(call) {
				if spa.isInLoop(call) || spa.isHotPath(fn.Name.Name) {
					pos := fset.Position(call.Pos())
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "BUFFER_ALLOCATION_IN_HOT_PATH",
						Severity:   SeverityMedium,
						Message:    "bytes.Buffer allocation in hot path",
						Suggestion: "Use sync.Pool to reuse buffers: var bufferPool = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}",
					})
				}
			}

			// Check for []byte allocations
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "make" {
				if len(call.Args) > 0 {
					if _, ok := call.Args[0].(*ast.ArrayType); ok {
						if spa.isInLoop(call) || spa.isHotPath(fn.Name.Name) {
							pos := fset.Position(call.Pos())
							issues = append(issues, Issue{
								File:       filename,
								Line:       pos.Line,
								Column:     pos.Column,
								Position:   pos,
								Type:       "BYTE_SLICE_ALLOCATION_IN_HOT_PATH",
								Severity:   SeverityMedium,
								Message:    "Frequent []byte allocation - consider buffer pool",
								Suggestion: "Use sync.Pool to reuse byte slices",
							})
						}
					}
				}
			}
		}
		return true
	})

	return issues
}

func (spa *SyncPoolAnalyzer) checkImproperPoolUsage(fn *ast.FuncDecl, filename string, fset *token.FileSet, poolVars map[string]bool) []Issue {
	var issues []Issue

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		// Check for storing pool objects beyond function scope
		if assign, ok := n.(*ast.AssignStmt); ok {
			for _, rhs := range assign.Rhs {
				if call, ok := rhs.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						if ident, ok := sel.X.(*ast.Ident); ok {
							if poolVars[ident.Name] && sel.Sel.Name == "Get" {
								// Check if assigned to a field (escapes function)
								for _, lhs := range assign.Lhs {
									if _, ok := lhs.(*ast.SelectorExpr); ok {
										pos := fset.Position(assign.Pos())
										issues = append(issues, Issue{
											File:       filename,
											Line:       pos.Line,
											Column:     pos.Column,
											Position:   pos,
											Type:       "POOL_OBJECT_ESCAPES",
											Severity:   SeverityHigh,
											Message:    "Pool object stored in field - may not be returned",
											Suggestion: "Don't store pool objects in struct fields",
										})
									}
								}
							}
						}
					}
				}
			}
		}

		// Check for modifying pool objects after Put
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					if poolVars[ident.Name] && sel.Sel.Name == "Put" {
						if len(call.Args) > 0 {
							// Check if object is used after Put
							if objIdent, ok := call.Args[0].(*ast.Ident); ok {
								if spa.isUsedAfterPut(fn.Body, objIdent.Name, call) {
									pos := fset.Position(call.Pos())
									issues = append(issues, Issue{
										File:       filename,
										Line:       pos.Line,
										Column:     pos.Column,
										Position:   pos,
										Type:       "USE_AFTER_POOL_PUT",
										Severity:   SeverityHigh,
										Message:    "Object used after returning to pool",
										Suggestion: "Don't use objects after Put() - they may be reused",
									})
								}
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

// Helper functions
func (spa *SyncPoolAnalyzer) isSyncPool(expr ast.Expr) bool {
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			return ident.Name == "sync" && sel.Sel.Name == "Pool"
		}
	}
	return false
}

func (spa *SyncPoolAnalyzer) isHotPath(funcName string) bool {
	hotPathKeywords := []string{"Handle", "Process", "Parse", "Serialize", "Render", "Execute", "Serve"}
	for _, keyword := range hotPathKeywords {
		if strings.Contains(funcName, keyword) {
			return true
		}
	}
	return false
}

func (spa *SyncPoolAnalyzer) isBytesBufferAllocation(call *ast.CallExpr) bool {
	// Check for bytes.NewBuffer or new(bytes.Buffer)
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			return ident.Name == "bytes" && sel.Sel.Name == "NewBuffer"
		}
	}

	if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "new" {
		if len(call.Args) > 0 {
			if sel, ok := call.Args[0].(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					return ident.Name == "bytes" && sel.Sel.Name == "Buffer"
				}
			}
		}
	}

	return false
}

func (spa *SyncPoolAnalyzer) getTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.ArrayType:
		return "[]byte"
	case *ast.MapType:
		return "map"
	case *ast.Ident:
		return t.Name
	}
	return ""
}

func (spa *SyncPoolAnalyzer) hasCorrespondingPut(getCall *ast.CallExpr, poolName string) bool {
	// Simplified - would need to track control flow
	return false
}

func (spa *SyncPoolAnalyzer) isUsedAfterPut(body *ast.BlockStmt, objName string, putCall *ast.CallExpr) bool {
	// Simplified - would need to track usage after the Put call
	return false
}

func (spa *SyncPoolAnalyzer) isInLoop(node ast.Node) bool {
	// Simplified - would need proper context tracking
	return false
}
