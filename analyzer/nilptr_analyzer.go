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

func NewNilPtrAnalyzer() *NilPtrAnalyzer {
	return &NilPtrAnalyzer{
		checkedVars:     make(map[string]bool, 50),
		assignedVars:    make(map[string]bool, 50),
		functionReturns: make(map[string][]bool, 50),
	}
}

func (npa *NilPtrAnalyzer) Name() string {
	return "NilPtrAnalyzer"
}

func (npa *NilPtrAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Reset state for each file
	npa.checkedVars = make(map[string]bool, 50)
	npa.assignedVars = make(map[string]bool, 50)

	// First pass: collect function return types
	ast.Inspect(astNode, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			npa.analyzeFunctionReturns(fn)
		}
		return true
	})

	// Second pass: analyze for nil pointer issues
	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			issues = append(issues, npa.analyzeFunction(node, filename, fset)...)
		case *ast.FuncLit:
			issues = append(issues, npa.analyzeFuncLit(node, filename, fset)...)
		}
		return true
	})

	return issues
}

func (npa *NilPtrAnalyzer) analyzeFunction(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	if fn.Body == nil {
		return issues
	}

	// Track nil checks and assignments within function scope
	localChecked := make(map[string]bool, 10)
	localAssigned := make(map[string]bool, 10)

	// Track type switch assignments to avoid flagging them as unchecked type assertions
	typeSwitchAssignments := npa.findTypeSwitchAssignments(fn.Body)

	// Analyze the function body for nil pointer issues
	issues = append(issues, npa.analyzeFunctionBody(fn, filename, fset, localChecked, localAssigned, typeSwitchAssignments)...)

	return issues
}

// findTypeSwitchAssignments identifies all type switch assignments in a block
func (npa *NilPtrAnalyzer) findTypeSwitchAssignments(body *ast.BlockStmt) map[*ast.AssignStmt]bool {
	typeSwitchAssignments := make(map[*ast.AssignStmt]bool)

	ast.Inspect(body, func(n ast.Node) bool {
		if typeSwitch, ok := n.(*ast.TypeSwitchStmt); ok {
			if typeSwitch.Assign != nil {
				if assignStmt, ok := typeSwitch.Assign.(*ast.AssignStmt); ok {
					typeSwitchAssignments[assignStmt] = true
				}
			}
		}
		return true
	})

	return typeSwitchAssignments
}

// analyzeFunctionBody analyzes the function body for nil pointer issues
func (npa *NilPtrAnalyzer) analyzeFunctionBody(fn *ast.FuncDecl, filename string, fset *token.FileSet,
	localChecked, localAssigned map[string]bool, typeSwitchAssignments map[*ast.AssignStmt]bool) []Issue {

	var issues []Issue

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			issues = append(issues, npa.analyzeAssignment(node, localChecked, localAssigned, filename, fset, typeSwitchAssignments)...)

		case *ast.IfStmt:
			npa.analyzeIfStatement(node, localChecked)

		case *ast.SelectorExpr:
			// Check for potential nil dereference
			if ident, ok := node.X.(*ast.Ident); ok {
				if !localChecked[ident.Name] && npa.isPotentialNil(ident.Name, localAssigned) {
					pos := fset.Position(node.Pos())
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "POTENTIAL_NIL_DEREF",
						Severity:   SeverityHigh,
						Message:    "Potential nil pointer dereference: " + ident.Name,
						Suggestion: "Add nil check before accessing field",
					})
				}
			}

		case *ast.IndexExpr:
			// Check for nil map/slice access
			if ident, ok := node.X.(*ast.Ident); ok {
				if !localChecked[ident.Name] && npa.isPotentialNil(ident.Name, localAssigned) {
					pos := fset.Position(node.Pos())
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "POTENTIAL_NIL_INDEX",
						Severity:   SeverityHigh,
						Message:    "Potential nil map/slice access: " + ident.Name,
						Suggestion: "Check if " + ident.Name + " is nil before indexing",
					})
				}
			}

		case *ast.RangeStmt:
			// Check for range over nil
			if ident, ok := node.X.(*ast.Ident); ok {
				if !localChecked[ident.Name] && npa.isPotentialNil(ident.Name, localAssigned) {
					pos := fset.Position(node.Pos())
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "RANGE_OVER_NIL",
						Severity:   SeverityMedium,
						Message:    "Potential range over nil: " + ident.Name,
						Suggestion: "Check if " + ident.Name + " is nil before ranging",
					})
				}
			}

		case *ast.CallExpr:
			// Check for method calls on potentially nil receivers
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					if !localChecked[ident.Name] && npa.isPotentialNil(ident.Name, localAssigned) {
						pos := fset.Position(node.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "NIL_METHOD_CALL",
							Severity:   SeverityHigh,
							Message:    "Method call on potentially nil receiver: " + ident.Name,
							Suggestion: "Check if " + ident.Name + " is nil before calling method",
						})
					}
				}
			}

			// Skip error return checks - use errcheck linter for this instead
		}
		return true
	})

	// Check for missing nil checks on parameters
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			if npa.isPointerType(field.Type) {
				for _, name := range field.Names {
					if !localChecked[name.Name] && npa.isUsedInFunction(name.Name, fn.Body) {
						pos := fset.Position(name.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "UNCHECKED_PARAM",
							Severity:   SeverityMedium,
							Message:    "Pointer parameter used without nil check: " + name.Name,
							Suggestion: "Add nil check for parameter " + name.Name,
						})
					}
				}
			}
		}
	}

	return issues
}

func (npa *NilPtrAnalyzer) analyzeFuncLit(fn *ast.FuncLit, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	if fn.Body == nil {
		return issues
	}

	// Track nil checks and assignments within function literal scope
	localChecked := make(map[string]bool, 10)
	localAssigned := make(map[string]bool, 10)

	// Track type switch assignments to avoid flagging them as unchecked type assertions
	typeSwitchAssignments := make(map[*ast.AssignStmt]bool)

	// First, identify all type switch assignments
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if typeSwitch, ok := n.(*ast.TypeSwitchStmt); ok {
			if typeSwitch.Assign != nil {
				if assignStmt, ok := typeSwitch.Assign.(*ast.AssignStmt); ok {
					typeSwitchAssignments[assignStmt] = true
				}
			}
		}
		return true
	})

	// Analyze assignments and function calls in function literal
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			issues = append(issues, npa.analyzeAssignment(node, localChecked, localAssigned, filename, fset, typeSwitchAssignments)...)
			// Skip error checks - use errcheck linter instead
		}
		return true
	})

	return issues
}

func (npa *NilPtrAnalyzer) analyzeAssignment(stmt *ast.AssignStmt, checked, assigned map[string]bool, filename string, fset *token.FileSet, typeSwitchAssignments map[*ast.AssignStmt]bool) []Issue {
	var issues []Issue

	for i, lhs := range stmt.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok {
			if i < len(stmt.Rhs) {
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
		}
	}

	// Check for unchecked type assertions (but skip type switches)
	if !typeSwitchAssignments[stmt] {
		for i := range stmt.Lhs {
			if i < len(stmt.Rhs) {
				if typeAssert, ok := stmt.Rhs[i].(*ast.TypeAssertExpr); ok {
					if len(stmt.Lhs) == 1 { // Single value type assertion
						pos := fset.Position(typeAssert.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "UNCHECKED_TYPE_ASSERTION",
							Severity:   SeverityHigh,
							Message:    "Type assertion without checking success",
							Suggestion: "Use two-value type assertion: value, ok := x.(Type)",
						})
					}
				}
			}
		}
	}

	return issues
}

func (npa *NilPtrAnalyzer) analyzeIfStatement(stmt *ast.IfStmt, checked map[string]bool) {
	// Check for nil checks in if conditions
	if binExpr, ok := stmt.Cond.(*ast.BinaryExpr); ok {
		if binExpr.Op == token.NEQ || binExpr.Op == token.EQL {
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

func (npa *NilPtrAnalyzer) isNilValue(expr ast.Expr) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name == "nil"
	}
	return false
}

func (npa *NilPtrAnalyzer) canReturnNil(call *ast.CallExpr) bool {
	// Check known functions that can return nil
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		nilReturnFuncs := map[string][]string{
			"os":   {"Open", "Create", "OpenFile"},
			"http": {"Get", "Post", "NewRequest"},
			"sql":  {"Open"},
		}

		if ident, ok := sel.X.(*ast.Ident); ok {
			if funcs, exists := nilReturnFuncs[ident.Name]; exists {
				for _, fn := range funcs {
					if sel.Sel.Name == fn {
						return true
					}
				}
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

func (npa *NilPtrAnalyzer) checksVariable(expr ast.Expr, varName string) bool {
	checked := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if binExpr, ok := n.(*ast.BinaryExpr); ok {
			if binExpr.Op == token.NEQ || binExpr.Op == token.EQL {
				if ident, ok := binExpr.X.(*ast.Ident); ok && ident.Name == varName {
					if npa.isNilValue(binExpr.Y) {
						checked = true
						return false
					}
				}
				if ident, ok := binExpr.Y.(*ast.Ident); ok && ident.Name == varName {
					if npa.isNilValue(binExpr.X) {
						checked = true
						return false
					}
				}
			}
		}
		return true
	})
	return checked
}

func (npa *NilPtrAnalyzer) usesVariable(stmt ast.Stmt, varName string) bool {
	used := false
	ast.Inspect(stmt, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Name == varName {
			used = true
			return false
		}
		return true
	})
	return used
}

func (npa *NilPtrAnalyzer) isUsedInFunction(varName string, body *ast.BlockStmt) bool {
	used := false
	ast.Inspect(body, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Name == varName {
			used = true
			return false
		}
		return true
	})
	return used
}
