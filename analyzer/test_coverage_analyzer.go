package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

type TestCoverageAnalyzer struct {
	functions map[string]bool
	tests     map[string]bool
}

func NewTestCoverageAnalyzer() *TestCoverageAnalyzer {
	return &TestCoverageAnalyzer{
		functions: make(map[string]bool, 50),
		tests:     make(map[string]bool, 50),
	}
}

func (tca *TestCoverageAnalyzer) Name() string {
	return "TestCoverageAnalyzer"
}

func (tca *TestCoverageAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Skip test files for function collection
	if strings.HasSuffix(filename, "_test.go") {
		tca.collectTests(astNode)
		return issues
	}

	// Collect all functions
	tca.collectFunctions(astNode)

	// Analyze each function for test coverage
	ast.Inspect(astNode, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.FuncDecl:
			if tca.isExported(decl.Name.Name) && !tca.hasTest(decl.Name.Name) {
				pos := fset.Position(decl.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "UNTESTED_EXPORT",
					Severity:   SeverityMedium,
					Message:    "Exported function has no test coverage",
					Suggestion: "Add test case for " + decl.Name.Name + " in corresponding _test.go file",
				})
			} else if !tca.hasTest(decl.Name.Name) && tca.isComplexFunction(decl) {
				pos := fset.Position(decl.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "UNTESTED_COMPLEX_FUNCTION",
					Severity:   SeverityMedium,
					Message:    "Complex function lacks test coverage",
					Suggestion: "Add comprehensive tests for complex logic",
				})
			}

			// Check for functions that interact with external resources without tests
			if tca.hasExternalInteraction(decl) && !tca.hasTest(decl.Name.Name) {
				pos := fset.Position(decl.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "UNTESTED_IO_FUNCTION",
					Severity:   SeverityHigh,
					Message:    "Function with I/O operations lacks test coverage",
					Suggestion: "Add tests with mocked dependencies for I/O operations",
				})
			}
		case *ast.TypeSpec:
			if tca.isExported(decl.Name.Name) && !tca.hasTypeTest(decl.Name.Name) {
				pos := fset.Position(decl.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "UNTESTED_TYPE",
					Severity:   SeverityLow,
					Message:    "Exported type has no test coverage",
					Suggestion: "Add test cases for type " + decl.Name.Name,
				})
			}
		}
		return true
	})

	// Check for missing benchmarks for performance-critical functions
	issues = append(issues, tca.checkBenchmarkCoverage(astNode, filename, fset)...)

	// Check for missing example functions for exported APIs
	issues = append(issues, tca.checkExampleCoverage(astNode, filename, fset)...)

	return issues
}

func (tca *TestCoverageAnalyzer) collectFunctions(node ast.Node) {
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			tca.functions[fn.Name.Name] = true
		}
		return true
	})
}

func (tca *TestCoverageAnalyzer) collectTests(node ast.Node) {
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			name := fn.Name.Name
			if strings.HasPrefix(name, "Test") {
				// Extract function name being tested
				testedFunc := strings.TrimPrefix(name, "Test")
				tca.tests[testedFunc] = true
				
				// Also check for lowercase version
				tca.tests[strings.ToLower(testedFunc[:1]) + testedFunc[1:]] = true
			} else if strings.HasPrefix(name, "Benchmark") {
				testedFunc := strings.TrimPrefix(name, "Benchmark")
				tca.tests[testedFunc] = true
			} else if strings.HasPrefix(name, "Example") {
				testedFunc := strings.TrimPrefix(name, "Example")
				tca.tests[testedFunc] = true
			}
		}
		return true
	})
}

func (tca *TestCoverageAnalyzer) hasTest(funcName string) bool {
	return tca.tests[funcName] || tca.tests[strings.Title(funcName)]
}

func (tca *TestCoverageAnalyzer) hasTypeTest(typeName string) bool {
	// Check if any test references this type
	return tca.tests[typeName]
}

func (tca *TestCoverageAnalyzer) isExported(name string) bool {
	if len(name) == 0 {
		return false
	}
	return name[0] >= 'A' && name[0] <= 'Z'
}

func (tca *TestCoverageAnalyzer) isComplexFunction(fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}

	// Count various complexity indicators
	var (
		loops      int
		conditions int
		lines      int
	)

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			loops++
		case *ast.IfStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt:
			conditions++
		case ast.Stmt:
			lines++
		}
		return true
	})

	// Consider function complex if it has multiple loops, many conditions, or is long
	return loops >= MediumComplexityThreshold || conditions >= MaxFunctionParams || lines >= MaxFunctionStatements
}

func (tca *TestCoverageAnalyzer) hasExternalInteraction(fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}

	hasIO := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					// Check for common I/O packages
					ioPackages := []string{"os", "io", "net", "http", "sql", "file", "bufio"}
					for _, pkg := range ioPackages {
						if ident.Name == pkg {
							hasIO = true
							return false
						}
					}
				}
			}
		}
		return true
	})

	return hasIO
}

func (tca *TestCoverageAnalyzer) checkBenchmarkCoverage(node ast.Node, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			if tca.hasPerformanceCriticalOperations(fn) && !tca.hasBenchmark(fn.Name.Name) {
				pos := fset.Position(fn.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "MISSING_BENCHMARK",
					Severity:   SeverityLow,
					Message:    "Performance-critical function lacks benchmark",
					Suggestion: "Add Benchmark" + fn.Name.Name + " to measure performance",
				})
			}
		}
		return true
	})

	return issues
}

func (tca *TestCoverageAnalyzer) hasPerformanceCriticalOperations(fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}

	critical := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if ident, ok := node.Fun.(*ast.Ident); ok {
				// Check for performance-critical operations
				if ident.Name == "sort" || ident.Name == "copy" || 
				   strings.Contains(ident.Name, "Search") {
					critical = true
				}
			}
		case *ast.ForStmt, *ast.RangeStmt:
			// Nested loops are performance-critical
			ast.Inspect(node, func(inner ast.Node) bool {
				if _, ok := inner.(*ast.ForStmt); ok && inner != node {
					critical = true
					return false
				}
				if _, ok := inner.(*ast.RangeStmt); ok && inner != node {
					critical = true
					return false
				}
				return true
			})
		}
		return !critical
	})

	return critical
}

func (tca *TestCoverageAnalyzer) hasBenchmark(funcName string) bool {
	return tca.tests["Benchmark" + funcName]
}

func (tca *TestCoverageAnalyzer) checkExampleCoverage(node ast.Node, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			if tca.isExported(fn.Name.Name) && tca.isPublicAPI(fn) && !tca.hasExample(fn.Name.Name) {
				pos := fset.Position(fn.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "MISSING_EXAMPLE",
					Severity:   SeverityLow,
					Message:    "Public API function lacks example",
					Suggestion: "Add Example" + fn.Name.Name + " to demonstrate usage",
				})
			}
		}
		return true
	})

	return issues
}

func (tca *TestCoverageAnalyzer) isPublicAPI(fn *ast.FuncDecl) bool {
	// Check if function has exported parameters and return values
	if fn.Type.Results != nil && fn.Type.Results.NumFields() > 0 {
		return tca.isExported(fn.Name.Name)
	}
	return false
}

func (tca *TestCoverageAnalyzer) hasExample(funcName string) bool {
	return tca.tests["Example" + funcName]
}