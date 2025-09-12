package analyzer

import (
	"go/ast"
	"go/token"
)

type NilPtrAnalyzer struct {
	checkedVars     map[string]bool
	assignedVars    map[string]bool
	functionReturns map[string][]bool // Track which return values can be nil
}

func NewNilPtrAnalyzer() Analyzer {
	return &NilPtrAnalyzer{
		checkedVars:     make(map[string]bool, 50),
		assignedVars:    make(map[string]bool, 50),
		functionReturns: make(map[string][]bool, 50),
	}
}

func (npa *NilPtrAnalyzer) Name() string {
	return "NilPtrAnalyzer"
}

func (npa *NilPtrAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
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

	// Reset state for each file
	npa.checkedVars = make(map[string]bool, 50)
	npa.assignedVars = make(map[string]bool, 50)

	// First pass: collect function return types
	ast.Inspect(
		astNode, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				npa.analyzeFunctionReturns(fn)
			}
			return true
		},
	)

	// Second pass: analyze for nil pointer issues
	ast.Inspect(
		astNode, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				issues = append(issues, npa.analyzeFunction(node, filename, fset)...)
			case *ast.FuncLit:
				issues = append(issues, npa.analyzeFuncLit(node, filename, fset)...)
			}
			return true
		},
	)

	return issues
}

func (npa *NilPtrAnalyzer) analyzeFunction(fn *ast.FuncDecl, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	if fn.Body == nil {
		return issues
	}

	// Track nil checks and assignments within function scope
	localChecked := make(map[string]bool, 10)
	localAssigned := make(map[string]bool, 10)

	// Track type switch assignments to avoid flagging them as unchecked type assertions
	typeSwitchAssignments := npa.findTypeSwitchAssignments(fn.Body)

	// AnalyzeAll the function body for nil pointer issues
	issues = append(issues, npa.analyzeFunctionBody(fn, filename, fset, localChecked, localAssigned, typeSwitchAssignments)...)

	return issues
}

func (npa *NilPtrAnalyzer) findTypeSwitchAssignments(body *ast.BlockStmt) map[*ast.AssignStmt]bool {
	typeSwitchAssignments := make(map[*ast.AssignStmt]bool)

	ast.Inspect(
		body, func(n ast.Node) bool {
			if typeSwitch, ok := n.(*ast.TypeSwitchStmt); ok {
				if typeSwitch.Assign != nil {
					if assignStmt, ok := typeSwitch.Assign.(*ast.AssignStmt); ok {
						typeSwitchAssignments[assignStmt] = true
					}
				}
			}
			return true
		},
	)

	return typeSwitchAssignments
}

func (npa *NilPtrAnalyzer) analyzeFunctionBody(
	fn *ast.FuncDecl, filename string, fset *token.FileSet,
	localChecked, localAssigned map[string]bool, typeSwitchAssignments map[*ast.AssignStmt]bool,
) []*Issue {
	var issues []*Issue

	ast.Inspect(
		fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				issues = append(issues, npa.analyzeAssignment(node, localAssigned, filename, fset, typeSwitchAssignments)...)
			case *ast.IfStmt:
				npa.analyzeIfStatement(node, localChecked)
			case *ast.SelectorExpr:
				issues = append(issues, npa.checkSelectorExpr(node, localChecked, localAssigned, filename, fset)...)
			case *ast.IndexExpr:
				issues = append(issues, npa.checkIndexExpr(node, localChecked, localAssigned, filename, fset)...)
			case *ast.RangeStmt:
				issues = append(issues, npa.checkRangeStmt(node, localChecked, localAssigned, filename, fset)...)
			case *ast.CallExpr:
				issues = append(issues, npa.checkCallExpr(node, localChecked, localAssigned, filename, fset)...)
			}
			return true
		},
	)

	// Check for missing nil checks on parameters
	issues = append(issues, npa.checkFunctionParams(fn, localChecked, filename, fset)...)

	return issues
}

func (npa *NilPtrAnalyzer) checkSelectorExpr(
	node *ast.SelectorExpr, localChecked, localAssigned map[string]bool, filename string, fset *token.FileSet,
) []*Issue {
	var issues []*Issue

	ident, ok := node.X.(*ast.Ident)
	if !ok {
		return issues
	}

	if !localChecked[ident.Name] && npa.isPotentialNil(ident.Name, localAssigned) {
		pos := fset.Position(node.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueNilCheck,
				Severity:   SeverityLevelHigh,
				Message:    "Potential nil pointer dereference: " + ident.Name,
				Suggestion: "Add nil check before accessing field",
			},
		)
	}
	return issues
}

func (npa *NilPtrAnalyzer) checkIndexExpr(
	node *ast.IndexExpr, localChecked, localAssigned map[string]bool, filename string, fset *token.FileSet,
) []*Issue {
	var issues []*Issue

	ident, ok := node.X.(*ast.Ident)
	if !ok {
		return issues
	}

	if !localChecked[ident.Name] && npa.isPotentialNil(ident.Name, localAssigned) {
		pos := fset.Position(node.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueNilCheck,
				Severity:   SeverityLevelHigh,
				Message:    "Potential nil map/slice access: " + ident.Name,
				Suggestion: "Check if " + ident.Name + " is nil before indexing",
			},
		)
	}
	return issues
}

func (npa *NilPtrAnalyzer) checkRangeStmt(
	node *ast.RangeStmt, localChecked, localAssigned map[string]bool, filename string, fset *token.FileSet,
) []*Issue {
	var issues []*Issue

	ident, ok := node.X.(*ast.Ident)
	if !ok {
		return issues
	}

	if !localChecked[ident.Name] && npa.isPotentialNil(ident.Name, localAssigned) {
		pos := fset.Position(node.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueNilCheck,
				Severity:   SeverityLevelMedium,
				Message:    "Potential range over nil: " + ident.Name,
				Suggestion: "Check if " + ident.Name + " is nil before ranging",
			},
		)
	}
	return issues
}

func (npa *NilPtrAnalyzer) checkCallExpr(
	node *ast.CallExpr, localChecked, localAssigned map[string]bool, filename string, fset *token.FileSet,
) []*Issue {
	var issues []*Issue

	sel, ok := node.Fun.(*ast.SelectorExpr)
	if !ok {
		return issues
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return issues
	}

	if !localChecked[ident.Name] && npa.isPotentialNil(ident.Name, localAssigned) {
		pos := fset.Position(node.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueNilCheck,
				Severity:   SeverityLevelHigh,
				Message:    "Method call on potentially nil receiver: " + ident.Name,
				Suggestion: "Check if " + ident.Name + " is nil before calling method",
			},
		)
	}
	return issues
}

func (npa *NilPtrAnalyzer) checkFunctionParams(
	fn *ast.FuncDecl, localChecked map[string]bool, filename string, fset *token.FileSet,
) []*Issue {
	var issues []*Issue

	if fn.Type.Params == nil || !fn.Name.IsExported() {
		return issues
	}

	for _, field := range fn.Type.Params.List {
		if !npa.isPointerType(field.Type) || npa.isStandardLibraryType(field.Type) {
			continue
		}

		for _, name := range field.Names {
			if !localChecked[name.Name] && npa.isUsedInFunction(name.Name, fn.Body) {
				pos := fset.Position(name.Pos())
				issues = append(
					issues, &Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       IssueNilCheck,
						Severity:   SeverityLevelMedium,
						Message:    "Pointer parameter used without nil check: " + name.Name,
						Suggestion: "Add nil check for parameter " + name.Name,
					},
				)
			}
		}
	}

	return issues
}

func (npa *NilPtrAnalyzer) analyzeFuncLit(fn *ast.FuncLit, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	if fn.Body == nil {
		return issues
	}

	// Track assignments within function literal scope
	localAssigned := make(map[string]bool, 10)

	// Track type switch assignments to avoid flagging them as unchecked type assertions
	typeSwitchAssignments := make(map[*ast.AssignStmt]bool)

	// First, identify all type switch assignments
	ast.Inspect(
		fn.Body, func(n ast.Node) bool {
			if typeSwitch, ok := n.(*ast.TypeSwitchStmt); ok {
				if typeSwitch.Assign != nil {
					if assignStmt, ok := typeSwitch.Assign.(*ast.AssignStmt); ok {
						typeSwitchAssignments[assignStmt] = true
					}
				}
			}
			return true
		},
	)

	// AnalyzeAll assignments and function calls in function literal
	ast.Inspect(
		fn.Body, func(n ast.Node) bool {
			if node, ok := n.(*ast.AssignStmt); ok {
				issues = append(issues, npa.analyzeAssignment(node, localAssigned, filename, fset, typeSwitchAssignments)...)
				// Skip error checks - use errcheck linter instead
			}
			return true
		},
	)

	return issues
}

func (npa *NilPtrAnalyzer) analyzeAssignment(
	stmt *ast.AssignStmt, assigned map[string]bool, filename string, fset *token.FileSet,
	typeSwitchAssignments map[*ast.AssignStmt]bool,
) []*Issue {
	var issues []*Issue

	for i, lhs := range stmt.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}

		if i >= len(stmt.Rhs) {
			continue
		}

		rhs := stmt.Rhs[i]

		// Check if assigning function call that can return nil
		if call, ok := rhs.(*ast.CallExpr); ok {
			// Mark as potentially nil if function can return nil
			if npa.canReturnNil(call) {
				assigned[ident.Name] = true
			}
		}

		// Check for explicit nil assignment
		if npa.isNilValue(rhs) {
			assigned[ident.Name] = true
		}
	}

	// Check for unchecked type assertions (but skip type switches)
	if typeSwitchAssignments[stmt] {
		return issues
	}

	for i := range stmt.Lhs {
		if i >= len(stmt.Rhs) {
			continue
		}

		typeAssert, ok := stmt.Rhs[i].(*ast.TypeAssertExpr)
		if !ok {
			continue
		}

		if len(stmt.Lhs) == 1 { // Single value type assertion
			pos := fset.Position(typeAssert.Pos())
			issues = append(
				issues, &Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       IssueNilCheck,
					Severity:   SeverityLevelHigh,
					Message:    "Type assertion without checking success",
					Suggestion: "Use two-value type assertion: value, ok := x.(Type)",
				},
			)
		}
	}

	return issues
}

func (npa *NilPtrAnalyzer) analyzeIfStatement(stmt *ast.IfStmt, checked map[string]bool) {
	// Check for nil checks in if conditions
	binExpr, ok := stmt.Cond.(*ast.BinaryExpr)
	if !ok {
		return
	}

	if binExpr.Op != token.NEQ && binExpr.Op != token.EQL {
		return
	}

	var varName string
	isNilCheck := false

	if ident, ok := binExpr.X.(*ast.Ident); ok {
		varName = ident.Name
		if npa.isNilValue(binExpr.Y) {
			isNilCheck = true
		}
	} else if ident, ok := binExpr.Y.(*ast.Ident); ok {
		varName = ident.Name
		if npa.isNilValue(binExpr.X) {
			isNilCheck = true
		}
	}

	if isNilCheck && varName != "" {
		checked[varName] = true
	}
}

func (npa *NilPtrAnalyzer) analyzeFunctionReturns(fn *ast.FuncDecl) {
	if fn.Type.Results == nil {
		return
	}

	canBeNil := []bool{}
	for _, field := range fn.Type.Results.List {
		// Check if return type can be nil (pointer, interface, map, slice, channel, function)
		canBeNil = append(canBeNil, npa.canTypeBeNil(field.Type))
	}

	npa.functionReturns[fn.Name.Name] = canBeNil
}

// Helper methods

func (npa *NilPtrAnalyzer) isPotentialNil(name string, assigned map[string]bool) bool {
	return assigned[name]
}

func (npa *NilPtrAnalyzer) isPointerType(expr ast.Expr) bool {
	_, ok := expr.(*ast.StarExpr)
	return ok
}

func (npa *NilPtrAnalyzer) isStandardLibraryType(expr ast.Expr) bool {
	// Check if type is from standard library (commonly used types that shouldn't need nil checks)
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}

	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	// Common standard library packages where nil checks are usually not needed
	stdPackages := []string{"token", "ast", "types", "testing", "http", "context"}
	for _, pkg := range stdPackages {
		if ident.Name == pkg {
			return true
		}
	}
	return false
}

func (npa *NilPtrAnalyzer) isNilValue(expr ast.Expr) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name == nilString
	}
	return false
}

func (npa *NilPtrAnalyzer) canReturnNil(call *ast.CallExpr) bool {
	// Check known functions that can return nil
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	nilReturnFuncs := map[string][]string{
		"os":   {"Open", "Create", "OpenFile"},
		"http": {"Get", "Post", "NewRequest"},
		"sql":  {"Open"},
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	funcs, exists := nilReturnFuncs[ident.Name]
	if exists {
		for _, fn := range funcs {
			if sel.Sel.Name == fn {
				return true
			}
		}
	}

	// Check if function is known to return pointer/interface
	if ident, ok := call.Fun.(*ast.Ident); ok {
		if returns, exists := npa.functionReturns[ident.Name]; exists {
			for _, canBeNil := range returns {
				if canBeNil {
					return true
				}
			}
		}
	}

	return false
}

func (npa *NilPtrAnalyzer) canTypeBeNil(expr ast.Expr) bool {
	switch expr.(type) {
	case *ast.StarExpr, *ast.InterfaceType, *ast.MapType,
		*ast.ArrayType, *ast.ChanType, *ast.FuncType:
		return true
	case *ast.Ident:
		// Could be an interface or other nilable type
		return true
	}
	return false
}

// 					}
// 				}
// 					}
// 				}
// 			}
// 		}
// 	})
// }

// 		}
// 	})
// }

func (npa *NilPtrAnalyzer) isUsedInFunction(varName string, body *ast.BlockStmt) bool {
	used := false
	ast.Inspect(
		body, func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok && ident.Name == varName {
				used = true
				return false
			}
			return true
		},
	)
	return used
}
