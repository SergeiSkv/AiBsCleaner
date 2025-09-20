package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// RaceConditionAnalyzer detects potential race conditions
type RaceConditionAnalyzer struct {
	sharedVars map[string]*SharedVarInfo
	goroutines []*ast.GoStmt
	mutexes    map[string]bool
}

type SharedVarInfo struct {
	Name        string
	Position    token.Position
	Reads       []token.Position
	Writes      []token.Position
	InGoroutine bool
	Protected   bool
}

func NewRaceConditionAnalyzer() *RaceConditionAnalyzer {
	return &RaceConditionAnalyzer{
		sharedVars: make(map[string]*SharedVarInfo),
		mutexes:    make(map[string]bool),
	}
}

func (rca *RaceConditionAnalyzer) Name() string {
	return "RaceConditionAnalyzer"
}

func (rca *RaceConditionAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Reset state
	rca.sharedVars = make(map[string]*SharedVarInfo)
	rca.goroutines = []*ast.GoStmt{}
	rca.mutexes = make(map[string]bool)

	// First pass: collect goroutines and mutex declarations
	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.GoStmt:
			rca.goroutines = append(rca.goroutines, node)
		case *ast.GenDecl:
			rca.collectMutexes(node)
		case *ast.Field:
			// Check struct fields for mutexes
			if node.Type != nil {
				rca.checkMutexType(node.Type)
			}
		}
		return true
	})

	// Second pass: analyze variable access in goroutines
	for _, goStmt := range rca.goroutines {
		rca.analyzeGoroutineAccess(goStmt, fset)
	}

	// Third pass: check for unprotected shared access
	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			issues = append(issues, rca.analyzeAssignment(node, filename, fset)...)
		case *ast.IncDecStmt:
			issues = append(issues, rca.analyzeIncDec(node, filename, fset)...)
		case *ast.FuncDecl:
			issues = append(issues, rca.analyzeFunctionRaces(node, filename, fset)...)
		}
		return true
	})

	// Detect race conditions
	issues = append(issues, rca.detectRaces(filename, fset)...)
	issues = append(issues, rca.detectMapRaces(filename, astNode, fset)...)
	issues = append(issues, rca.detectSliceRaces(filename, astNode, fset)...)

	return issues
}

func (rca *RaceConditionAnalyzer) collectMutexes(decl *ast.GenDecl) {
	for _, spec := range decl.Specs {
		if valueSpec, ok := spec.(*ast.ValueSpec); ok {
			for _, name := range valueSpec.Names {
				if valueSpec.Type != nil {
					if rca.isMutexType(valueSpec.Type) {
						rca.mutexes[name.Name] = true
					}
				}
			}
		}
	}
}

func (rca *RaceConditionAnalyzer) checkMutexType(expr ast.Expr) {
	switch t := expr.(type) {
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			if ident.Name == "sync" && (t.Sel.Name == "Mutex" || t.Sel.Name == "RWMutex") {
				// Found mutex field
			}
		}
	}
}

func (rca *RaceConditionAnalyzer) isMutexType(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name == "sync" && (t.Sel.Name == "Mutex" || t.Sel.Name == "RWMutex")
		}
	case *ast.StarExpr:
		return rca.isMutexType(t.X)
	}
	return false
}

func (rca *RaceConditionAnalyzer) analyzeGoroutineAccess(goStmt *ast.GoStmt, fset *token.FileSet) {
	hasLock := false

	// Check if goroutine uses mutex
	ast.Inspect(goStmt.Call, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "Lock" || sel.Sel.Name == "RLock" {
					hasLock = true
				}
			}
		}

		// Track variable access
		switch node := n.(type) {
		case *ast.Ident:
			if node.Obj != nil && node.Obj.Kind == ast.Var {
				if info, exists := rca.sharedVars[node.Name]; exists {
					info.InGoroutine = true
					info.Protected = hasLock
				} else {
					rca.sharedVars[node.Name] = &SharedVarInfo{
						Name:        node.Name,
						Position:    fset.Position(node.Pos()),
						InGoroutine: true,
						Protected:   hasLock,
					}
				}
			}
		}
		return true
	})
}

func (rca *RaceConditionAnalyzer) analyzeAssignment(assign *ast.AssignStmt, _ string, fset *token.FileSet) []Issue {
	var issues []Issue

	for _, lhs := range assign.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok {
			if info, exists := rca.sharedVars[ident.Name]; exists {
				info.Writes = append(info.Writes, fset.Position(assign.Pos()))
			} else {
				rca.sharedVars[ident.Name] = &SharedVarInfo{
					Name:     ident.Name,
					Position: fset.Position(assign.Pos()),
					Writes:   []token.Position{fset.Position(assign.Pos())},
				}
			}
		}
	}

	return issues
}

func (rca *RaceConditionAnalyzer) analyzeIncDec(stmt *ast.IncDecStmt, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	if ident, ok := stmt.X.(*ast.Ident); ok {
		// Check if it's a shared variable being modified without protection
		inGoroutine := false
		for _, goStmt := range rca.goroutines { //nolint:nestif // Race detection requires multiple checks
			if containsPosition(goStmt, stmt.Pos()) {
				inGoroutine = true
				break
			}
		}

		if inGoroutine {
			pos := fset.Position(stmt.Pos())
			issues = append(issues, Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       "RACE_CONDITION_INCDEC",
				Severity:   SeverityHigh,
				Message:    "Non-atomic increment/decrement in goroutine",
				Suggestion: "Use atomic.AddInt64() or protect with mutex",
			})
		}

		if info, exists := rca.sharedVars[ident.Name]; exists {
			info.Writes = append(info.Writes, fset.Position(stmt.Pos()))
		}
	}

	return issues
}

func (rca *RaceConditionAnalyzer) analyzeFunctionRaces(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue { //nolint:gocyclo // Race analysis inherently complex
	var issues []Issue

	// Check if function modifies global variables without synchronization
	hasSync := false
	modifiesGlobal := false

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		// Check for sync primitives
		if call, ok := n.(*ast.CallExpr); ok { //nolint:nestif // Sync analysis requires nested checks
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					if ident.Name == "sync" || strings.Contains(sel.Sel.Name, "Lock") {
						hasSync = true
					}
					if ident.Name == "atomic" {
						hasSync = true
					}
				}
			}
		}

		// Check for global variable modification
		if assign, ok := n.(*ast.AssignStmt); ok { //nolint:nestif // Global variable analysis requires nested checks
			for _, lhs := range assign.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok {
					if ident.Obj != nil && ident.Obj.Kind == ast.Var {
						// Check if it's a package-level variable (not local variable)
						if ident.Obj.Decl != nil {
							if valSpec, ok := ident.Obj.Decl.(*ast.ValueSpec); ok {
								// Check if this ValueSpec is at package level (not inside function)
								isPackageLevel := true
								for parent := fn.Body; parent != nil; {
									ast.Inspect(parent, func(node ast.Node) bool {
										if node == valSpec {
											isPackageLevel = false
											return false
										}
										return true
									})
									break
								}
								if isPackageLevel {
									modifiesGlobal = true
								}
							}
						}
					}
				}
			}
		}
		return true
	})

	if modifiesGlobal && !hasSync && fn.Name.IsExported() {
		pos := fset.Position(fn.Pos())
		issues = append(issues, Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       "RACE_CONDITION_GLOBAL",
			Severity:   SeverityHigh,
			Message:    "Exported function modifies global state without synchronization",
			Suggestion: "Add mutex protection or use atomic operations",
		})
	}

	return issues
}

func (rca *RaceConditionAnalyzer) detectRaces(filename string, _ *token.FileSet) []Issue {
	var issues []Issue

	for name, info := range rca.sharedVars {
		if info.InGoroutine && !info.Protected {
			if len(info.Writes) > 0 {
				for _, pos := range info.Writes {
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "RACE_CONDITION",
						Severity:   SeverityHigh,
						Message:    "Variable '" + name + "' accessed in goroutine without synchronization",
						Suggestion: "Use mutex, channel, or atomic operations for safe concurrent access",
					})
				}
			}
		}
	}

	return issues
}

func (rca *RaceConditionAnalyzer) detectMapRaces(filename string, node ast.Node, fset *token.FileSet) []Issue {
	var issues []Issue
	inGoroutine := false

	ast.Inspect(node, func(n ast.Node) bool {
		if _, ok := n.(*ast.GoStmt); ok {
			inGoroutine = true
		}

		// Check for concurrent map access
		if inGoroutine { //nolint:nestif // Map race detection requires nested checks
			switch expr := n.(type) {
			case *ast.IndexExpr:
				// Check if it's a map access
				if ident, ok := expr.X.(*ast.Ident); ok {
					if ident.Obj != nil {
						// Simple heuristic: if variable name contains "map" or type is map
						if strings.Contains(strings.ToLower(ident.Name), "map") {
							pos := fset.Position(expr.Pos())
							issues = append(issues, Issue{
								File:       filename,
								Line:       pos.Line,
								Column:     pos.Column,
								Position:   pos,
								Type:       "CONCURRENT_MAP_ACCESS",
								Severity:   SeverityHigh,
								Message:    "Concurrent map access detected",
								Suggestion: "Use sync.Map or protect map with sync.RWMutex",
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

func (rca *RaceConditionAnalyzer) detectSliceRaces(filename string, node ast.Node, fset *token.FileSet) []Issue {
	var issues []Issue

	// Track slice append operations in goroutines
	ast.Inspect(node, func(n ast.Node) bool {
		if goStmt, ok := n.(*ast.GoStmt); ok { //nolint:nestif // Slice append analysis requires nested checks
			ast.Inspect(goStmt.Call, func(inner ast.Node) bool {
				if call, ok := inner.(*ast.CallExpr); ok {
					if ident, ok := call.Fun.(*ast.Ident); ok {
						if ident.Name == "append" {
							pos := fset.Position(call.Pos())
							issues = append(issues, Issue{
								File:       filename,
								Line:       pos.Line,
								Column:     pos.Column,
								Position:   pos,
								Type:       "CONCURRENT_SLICE_APPEND",
								Severity:   SeverityHigh,
								Message:    "Concurrent append to slice without synchronization",
								Suggestion: "Protect slice operations with mutex or use channels",
							})
						}
					}
				}
				return true
			})
		}
		return true
	})

	return issues
}

func containsPosition(node ast.Node, pos token.Pos) bool {
	return node.Pos() <= pos && pos <= node.End()
}
