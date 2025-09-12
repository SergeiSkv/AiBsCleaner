package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// TestCoverageAnalyzer detects missing test coverage
type TestCoverageAnalyzer struct {
	// Track what functions have tests
	tests     map[string]bool
	functions map[string]bool
}

func NewTestCoverageAnalyzer() Analyzer {
	return &TestCoverageAnalyzer{
		tests:     make(map[string]bool),
		functions: make(map[string]bool),
	}
}

func (tca *TestCoverageAnalyzer) Name() string {
	return "Test Coverage Analysis"
}

func (tca *TestCoverageAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
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

	// Skip test files for function collection
	if strings.HasSuffix(filename, "_test.go") {
		tca.collectTests(astNode)
		return issues
	}

	// Collect all functions
	tca.collectFunctions(astNode)

	// AnalyzeAll each function for test coverage
	ast.Inspect(
		astNode, func(n ast.Node) bool {
			switch decl := n.(type) {
			case *ast.FuncDecl:
				issues = append(issues, tca.analyzeFunctionCoverage(decl, filename, fset)...)
			case *ast.TypeSpec:
				issues = append(issues, tca.analyzeTypeCoverage(decl, filename, fset)...)
			}
			return true
		},
	)

	// Check for missing benchmarks for performance-critical functions
	issues = append(issues, tca.checkBenchmarkCoverage(astNode, filename, fset)...)

	// Check for missing example functions for exported APIs
	issues = append(issues, tca.checkExampleCoverage(astNode, filename, fset)...)

	return issues
}

func (tca *TestCoverageAnalyzer) analyzeFunctionCoverage(decl *ast.FuncDecl, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue
	pos := fset.Position(decl.Pos())
	funcName := decl.Name.Name

	if tca.isExported(funcName) && !tca.hasTest(funcName) {
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueMissingTest,
				Severity:   SeverityLevelMedium,
				Message:    "Exported function has no test coverage",
				Suggestion: "Add test case for " + funcName + " in corresponding _test.go file",
			},
		)

		// Check if it's also a complex function
		if tca.isComplexFunction(decl) {
			issues = append(
				issues, &Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       IssueMissingTest,
					Severity:   SeverityLevelHigh,
					Message:    "Complex function lacks test coverage",
					Suggestion: "Add comprehensive tests for complex logic",
				},
			)
		}

		// Check if function has error handling
		if tca.hasErrorHandling(decl) {
			issues = append(
				issues, &Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       IssueMissingTest,
					Severity:   SeverityLevelHigh,
					Message:    "Function with error handling lacks test coverage",
					Suggestion: "Add tests for both success and error paths",
				},
			)
		}
	}

	// Check for functions that interact with external resources without tests
	if tca.hasExternalInteraction(decl) && !tca.hasTest(funcName) {
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueMissingTest,
				Severity:   SeverityLevelHigh,
				Message:    "Function with I/O operations lacks test coverage",
				Suggestion: "Add tests with mocked dependencies for I/O operations",
			},
		)
	}

	return issues
}

func (tca *TestCoverageAnalyzer) analyzeTypeCoverage(decl *ast.TypeSpec, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	if tca.isExported(decl.Name.Name) && !tca.hasTypeTest(decl.Name.Name) {
		pos := fset.Position(decl.Pos())
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueMissingTest,
				Severity:   SeverityLevelLow,
				Message:    "Exported type has no test coverage",
				Suggestion: "Add test cases for type " + decl.Name.Name,
			},
		)
	}

	return issues
}

func (tca *TestCoverageAnalyzer) collectTests(node ast.Node) {
	ast.Inspect(
		node, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				if strings.HasPrefix(fn.Name.Name, "Test") && fn.Name.IsExported() {
					// Extract the function name being tested
					// TestFuncName -> FuncName
					testName := strings.TrimPrefix(fn.Name.Name, "Test")
					tca.tests[testName] = true
					// Also try lowercase version
					if testName != "" {
						lowerName := strings.ToLower(testName[:1]) + testName[1:]
						tca.tests[lowerName] = true
					}
				}
			}
			return true
		},
	)
}

func (tca *TestCoverageAnalyzer) collectFunctions(node ast.Node) {
	ast.Inspect(
		node, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				tca.functions[fn.Name.Name] = true
			}
			return true
		},
	)
}

func (tca *TestCoverageAnalyzer) hasTest(funcName string) bool {
	// Check both the original name and capitalized version
	if tca.tests[funcName] {
		return true
	}
	// Capitalize first letter for test name matching
	if funcName != "" {
		capitalized := strings.ToUpper(funcName[:1]) + funcName[1:]
		return tca.tests[capitalized]
	}
	return false
}

func (tca *TestCoverageAnalyzer) hasTypeTest(typeName string) bool {
	// Check if any test references this type
	for testName := range tca.tests {
		if strings.Contains(testName, typeName) {
			return true
		}
	}
	return false
}

func (tca *TestCoverageAnalyzer) isExported(name string) bool {
	if name == "" {
		return false
	}
	return strings.ToUpper(name[:1]) == name[:1]
}

func (tca *TestCoverageAnalyzer) isComplexFunction(fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}

	// Count various complexity indicators
	complexity := 0
	ast.Inspect(
		fn.Body, func(n ast.Node) bool {
			switch n.(type) {
			case *ast.ForStmt:
				complexity++
			case *ast.RangeStmt:
				complexity++
			case *ast.IfStmt:
				complexity++
			case *ast.SwitchStmt:
				complexity++
			case *ast.TypeSwitchStmt:
				complexity++
			case *ast.SelectStmt:
				complexity++
			}
			return true
		},
	)

	return complexity > 3
}

func (tca *TestCoverageAnalyzer) hasExternalInteraction(fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}

	hasExternal := false
	ast.Inspect(
		fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					pkgIdent, ok := sel.X.(*ast.Ident)
					if !ok {
						return true
					}

					// Check for I/O operations
					switch pkgIdent.Name {
					case pkgOS, pkgIO, pkgIOutil, pkgHTTP, "net", "sql", "database":
						hasExternal = true
						return false
					}

					// Check for specific functions
					switch sel.Sel.Name {
					case methodOpen, methodCreate, methodRead, methodWrite,
						methodReadFile, methodDial, methodGet, methodPost:
						hasExternal = true
						return false
					}
				}
			}
			return true
		},
	)

	return hasExternal
}

func (tca *TestCoverageAnalyzer) checkBenchmarkCoverage(node ast.Node, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	// Look for performance-critical functions without benchmarks
	ast.Inspect(
		node, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				if tca.isPerformanceCritical(fn) && !tca.hasBenchmark(fn.Name.Name) {
					pos := fset.Position(fn.Pos())
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueMissingBenchmark,
							Severity:   SeverityLevelLow,
							Message:    "Performance-critical function lacks benchmark",
							Suggestion: "Add Benchmark" + fn.Name.Name + " function",
						},
					)
				}
			}
			return true
		},
	)

	return issues
}

func (tca *TestCoverageAnalyzer) checkExampleCoverage(node ast.Node, filename string, fset *token.FileSet) []*Issue {
	var issues []*Issue

	// Look for exported functions without examples
	ast.Inspect(
		node, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				if tca.isExported(fn.Name.Name) && tca.isPublicAPI(fn) && !tca.hasExample(fn.Name.Name) {
					pos := fset.Position(fn.Pos())
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueMissingExample,
							Severity:   SeverityLevelLow,
							Message:    "Public API function lacks example",
							Suggestion: "Add Example" + fn.Name.Name + " function to demonstrate usage",
						},
					)
				}
			}
			return true
		},
	)

	return issues
}

func (tca *TestCoverageAnalyzer) isPerformanceCritical(fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}

	// Check for loops and heavy operations
	loopCount := 0
	ast.Inspect(
		fn.Body, func(n ast.Node) bool {
			switch n.(type) {
			case *ast.ForStmt, *ast.RangeStmt:
				loopCount++
			}
			return true
		},
	)

	return loopCount > 0
}

func (tca *TestCoverageAnalyzer) hasBenchmark(funcName string) bool {
	benchName := "Benchmark" + funcName
	return tca.tests[benchName]
}

func (tca *TestCoverageAnalyzer) hasExample(funcName string) bool {
	exampleName := "Example" + funcName
	return tca.tests[exampleName]
}

func (tca *TestCoverageAnalyzer) isPublicAPI(fn *ast.FuncDecl) bool {
	// Consider a function as public API if it's exported and has parameters
	if fn.Type.Params == nil {
		return false
	}
	return len(fn.Type.Params.List) > 0 || fn.Type.Results != nil
}

// Helper function to check error handling
func (tca *TestCoverageAnalyzer) hasErrorHandling(fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}

	hasError := false

	// Check for error returns
	if fn.Type != nil && fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			if ident, ok := field.Type.(*ast.Ident); ok && ident.Name == typeError {
				hasError = true
				break
			}
		}
	}

	// Check for error handling in body
	if !hasError {
		return false
	}

	ast.Inspect(
		fn.Body, func(n ast.Node) bool {
			node, ok := n.(*ast.IfStmt)
			if !ok {
				return true
			}

			// Check for error checking pattern: if err != nil
			binaryExpr, ok := node.Cond.(*ast.BinaryExpr)
			if !ok || binaryExpr.Op != token.NEQ {
				return true
			}

			ident, ok := binaryExpr.X.(*ast.Ident)
			if !ok || ident.Name != "err" {
				return true
			}

			ident2, ok := binaryExpr.Y.(*ast.Ident)
			if ok && ident2.Name == nilString {
				hasError = true
				return false
			}
			return true
		},
	)

	return hasError
}
