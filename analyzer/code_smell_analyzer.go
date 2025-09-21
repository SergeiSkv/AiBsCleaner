package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// CodeSmellAnalyzer detects "bullshit code" - bad practices, code smells, and lazy programming
type CodeSmellAnalyzer struct{}

func NewCodeSmellAnalyzer() *CodeSmellAnalyzer {
	return &CodeSmellAnalyzer{}
}

func (csa *CodeSmellAnalyzer) Name() string {
	return "CodeSmellAnalyzer"
}

func (csa *CodeSmellAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(astNode, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			issues = append(issues, csa.analyzeFunction(node, filename, fset)...)
		case *ast.GenDecl:
			issues = append(issues, csa.analyzeDeclaration(node, filename, fset)...)
		case *ast.IfStmt:
			issues = append(issues, csa.analyzeIfStatement(node, filename, fset)...)
		case *ast.CallExpr:
			issues = append(issues, csa.analyzeCall(node, filename, fset)...)
		case *ast.BasicLit:
			issues = append(issues, csa.analyzeLiteral(node, filename, fset)...)
		}
		return true
	})

	return issues
}

func (csa *CodeSmellAnalyzer) analyzeFunction(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	// Check for too long functions (code smell)
	if fn.Body != nil && len(fn.Body.List) > MaxFunctionStatements {
		pos := fset.Position(fn.Pos())
		issues = append(issues, Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       "GOD_FUNCTION",
			Severity:   SeverityHigh,
			Message:    "Function is too long (>" + fmt.Sprintf("%d", MaxFunctionStatements) + " statements) - split into smaller functions",
			Suggestion: "Break down this function into smaller, focused functions",
		})
	}

	// Check for too many parameters
	if fn.Type.Params != nil && len(fn.Type.Params.List) > MaxFunctionParams {
		pos := fset.Position(fn.Pos())
		issues = append(issues, Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       "TOO_MANY_PARAMS",
			Severity:   SeverityMedium,
			Message:    "Function has too many parameters - consider using a struct",
			Suggestion: "Group related parameters into a configuration struct",
		})
	}

	// Check for single-letter variable names - but be more permissive in library code
	if fn.Body != nil {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				for _, lhs := range node.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok {
						// Allow common single-letter variables in library code
						// Extended list based on common patterns in real libraries
						allowedSingleLetter := []string{"i", "j", "k", "_", "n", "m", "x", "y", "z", "s", "b", "c", "r", "w", "v", "p", "q", "t", "e", "d", "h", "l", "f", "u", "o", "a", "g"}
						isAllowed := false
						for _, allowed := range allowedSingleLetter {
							if ident.Name == allowed {
								isAllowed = true
								break
							}
						}
						if len(ident.Name) == 1 && !isAllowed {
							pos := fset.Position(ident.Pos())
							issues = append(issues, Issue{
								File:       filename,
								Line:       pos.Line,
								Column:     pos.Column,
								Position:   pos,
								Type:       "LAZY_NAMING",
								Severity:   SeverityLow,
								Message:    "Uncommon single-letter variable name '" + ident.Name + "' - consider descriptive names",
								Suggestion: "Use meaningful variable names for better readability",
							})
						}
					}
				}
			case *ast.CallExpr:
				// Check for fmt.Println debugging (should use proper logging)
				if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
					if ident, ok := sel.X.(*ast.Ident); ok {
						if ident.Name == "fmt" && (sel.Sel.Name == "Println" || sel.Sel.Name == "Printf" || sel.Sel.Name == "Print") {
							pos := fset.Position(node.Pos())
							issues = append(issues, Issue{
								File:       filename,
								Line:       pos.Line,
								Column:     pos.Column,
								Position:   pos,
								Type:       "CONSOLE_LOG_DEBUGGING",
								Severity:   SeverityMedium,
								Message:    "Using fmt.Print for debugging - use proper logging",
								Suggestion: "Use a logging library (log, zap, logrus) instead of fmt.Print",
							})
						}
					}
				}
			}
			return true
		})
	}

	// Skip lazy error handling check for now - too many false positives in library code
	// Library code often has valid patterns that look "lazy" but are appropriate

	// Check for TODO/FIXME/HACK comments (technical debt)
	if fn.Doc != nil {
		if fn.Doc != nil {
			for _, comment := range fn.Doc.List {
				upper := strings.ToUpper(comment.Text)
				if strings.Contains(upper, "TODO") || strings.Contains(upper, "FIXME") ||
					strings.Contains(upper, "HACK") || strings.Contains(upper, "XXX") {
					pos := fset.Position(comment.Pos())
					issues = append(issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "TECHNICAL_DEBT",
						Severity:   SeverityMedium,
						Message:    "Unresolved TODO/FIXME/HACK comment",
						Suggestion: "Address technical debt or create a ticket to track it",
					})
				}
			}
		}
	}

	return issues
}

func (csa *CodeSmellAnalyzer) analyzeDeclaration(decl *ast.GenDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	// Check for global variables - but be lenient with common library patterns
	if decl.Tok == token.VAR {
		for _, spec := range decl.Specs {
			if vspec, ok := spec.(*ast.ValueSpec); ok {
				for _, name := range vspec.Names {
					if name.IsExported() {
						// Allow common library global variables patterns
						if isLibraryGlobalVariable(name.Name) {
							continue
						}
						pos := fset.Position(name.Pos())
						issues = append(issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "GLOBAL_VARIABLE",
							Severity:   SeverityLow, // Reduced severity
							Message:    "Global variable '" + name.Name + "' - consider alternatives",
							Suggestion: "Consider dependency injection or encapsulation",
						})
					}
				}
			}
		}
	}

	return issues
}

func (csa *CodeSmellAnalyzer) analyzeIfStatement(stmt *ast.IfStmt, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	// Check for deeply nested if statements (arrow anti-pattern)
	nestLevel := csa.countIfNesting(stmt, 0)
	if nestLevel > MaxNestedLoops {
		pos := fset.Position(stmt.Pos())
		issues = append(issues, Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       "ARROW_ANTIPATTERN",
			Severity:   SeverityMedium,
			Message:    "Deeply nested if statements (>" + fmt.Sprintf("%d", MaxNestedLoops) + " levels) - refactor using early returns",
			Suggestion: "Use guard clauses and early returns to reduce nesting",
		})
	}

	// Check for if true/false (WTF code)
	if lit, ok := stmt.Cond.(*ast.Ident); ok {
		if lit.Name == "true" || lit.Name == "false" {
			pos := fset.Position(stmt.Pos())
			issues = append(issues, Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       "USELESS_CONDITION",
				Severity:   SeverityHigh,
				Message:    "Condition is always " + lit.Name + " - this is nonsense",
				Suggestion: "Remove the condition or fix the logic",
			})
		}
	}

	// Check for empty else blocks
	if stmt.Else != nil {
		if block, ok := stmt.Else.(*ast.BlockStmt); ok && len(block.List) == 0 {
			pos := fset.Position(stmt.Else.Pos())
			issues = append(issues, Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       "EMPTY_ELSE",
				Severity:   SeverityLow,
				Message:    "Empty else block - remove it",
				Suggestion: "Remove unnecessary empty else block",
			})
		}
	}

	return issues
}

func (csa *CodeSmellAnalyzer) analyzeCall(call *ast.CallExpr, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	// Check for panic() calls (except in main, test files, or assertion libraries)
	if ident, ok := call.Fun.(*ast.Ident); ok {
		if ident.Name == "panic" {
			// Skip test files and assertion libraries
			skipReasons := []string{}
			if strings.Contains(filename, "_test.go") {
				skipReasons = append(skipReasons, "_test.go")
			}
			if strings.Contains(filename, "/test/") {
				skipReasons = append(skipReasons, "/test/")
			}
			if strings.Contains(filename, "assert") {
				skipReasons = append(skipReasons, "assert")
			}
			if strings.Contains(filename, "require") {
				skipReasons = append(skipReasons, "require")
			}
			if strings.Contains(filename, "testify") {
				skipReasons = append(skipReasons, "testify")
			}
			if strings.Contains(filename, "mock") {
				skipReasons = append(skipReasons, "mock")
			}
			if strings.HasSuffix(filename, "/main.go") {
				skipReasons = append(skipReasons, "/main.go")
			}
			if strings.Contains(filename, "/cmd/") {
				skipReasons = append(skipReasons, "/cmd/")
			}
			if strings.Contains(filename, "/examples/") {
				skipReasons = append(skipReasons, "/examples/")
			}

			// fmt.Printf("DEBUG: Found panic in %s, skip reasons: %v\n", filename, skipReasons)

			if len(skipReasons) == 0 {
				pos := fset.Position(call.Pos())
				// fmt.Printf("DEBUG: Creating PANIC_IN_LIBRARY issue for %s:%d\n", filename, pos.Line)
				issue := Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "PANIC_IN_LIBRARY",
					Severity:   SeverityHigh,
					Message:    "Using panic() in library code - return errors instead",
					Suggestion: "Return an error instead of panicking",
				}
				issues = append(issues, issue)
				// fmt.Printf("DEBUG: Issues count after append: %d\n", len(issues))
			}
		}
	}

	// Check for time.Sleep (usually indicates bad design)
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			if ident.Name == "time" && sel.Sel.Name == "Sleep" {
				pos := fset.Position(call.Pos())
				issues = append(issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "SLEEP_INSTEAD_OF_SYNC",
					Severity:   SeverityMedium,
					Message:    "Using time.Sleep - indicates poor synchronization",
					Suggestion: "Use channels, wait groups, or proper synchronization",
				})
			}
		}
	}

	// fmt.Printf("DEBUG: analyzeCall returning %d issues\n", len(issues))
	return issues
}

func (csa *CodeSmellAnalyzer) analyzeLiteral(lit *ast.BasicLit, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	// Check for magic numbers
	if lit.Kind == token.INT || lit.Kind == token.FLOAT {
		value := lit.Value
		// Ignore 0, 1, 2, 10, 100, 1000 as they're commonly acceptable
		if value != "0" && value != "1" && value != "2" && value != "10" &&
			value != "100" && value != "1000" && value != "1024" {
			pos := fset.Position(lit.Pos())
			issues = append(issues, Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       "MAGIC_NUMBER",
				Severity:   SeverityLow,
				Message:    "Magic number " + value + " - use named constant",
				Suggestion: "Define a named constant for this value",
			})
		}
	}

	// Check for hardcoded strings that look like config
	if lit.Kind == token.STRING {
		value := strings.Trim(lit.Value, "\"'`")
		// Check for URLs, IPs, paths
		if (strings.Contains(value, "http://") || strings.Contains(value, "https://") ||
			strings.Contains(value, "localhost") || strings.Contains(value, "127.0.0.1") ||
			strings.HasPrefix(value, "/")) && len(value) > 5 {
			pos := fset.Position(lit.Pos())
			issues = append(issues, Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       "HARDCODED_CONFIG",
				Severity:   SeverityMedium,
				Message:    "Hardcoded configuration value: " + value,
				Suggestion: "Move to configuration file or environment variable",
			})
		}
	}

	return issues
}

func (csa *CodeSmellAnalyzer) checkLazyErrorHandling(body *ast.BlockStmt, issues *[]Issue, filename string, fset *token.FileSet) {
	ast.Inspect(body, func(n ast.Node) bool {
		if ifStmt, ok := n.(*ast.IfStmt); ok {
			// Check for if err != nil { return err } pattern without context
			if csa.isLazyErrorCheck(ifStmt) {
				// Check if it just returns err without wrapping
				block := ifStmt.Body
				if len(block.List) == 1 {
					if ret, ok := block.List[0].(*ast.ReturnStmt); ok && len(ret.Results) == 1 {
						if ident, ok := ret.Results[0].(*ast.Ident); ok && ident.Name == "err" {
							pos := fset.Position(ifStmt.Pos())
							*issues = append(*issues, Issue{
								File:       filename,
								Line:       pos.Line,
								Column:     pos.Column,
								Position:   pos,
								Type:       "LAZY_ERROR_HANDLING",
								Severity:   SeverityLow,
								Message:    "Error returned without context",
								Suggestion: "Wrap error with context: fmt.Errorf(\"failed to X: %w\", err)",
							})
						}
					}
				}
			}
		}
		return true
	})
}

func (csa *CodeSmellAnalyzer) countIfNesting(stmt *ast.IfStmt, currentLevel int) int {
	maxLevel := currentLevel + 1

	// Check the then branch
	if stmt.Body != nil {
		ast.Inspect(stmt.Body, func(n ast.Node) bool {
			if innerIf, ok := n.(*ast.IfStmt); ok && innerIf != stmt {
				level := csa.countIfNesting(innerIf, currentLevel+1)
				if level > maxLevel {
					maxLevel = level
				}
			}
			return true
		})
	}

	// Check the else branch
	if stmt.Else != nil {
		if elseIf, ok := stmt.Else.(*ast.IfStmt); ok {
			level := csa.countIfNesting(elseIf, currentLevel)
			if level > maxLevel {
				maxLevel = level
			}
		}
	}

	return maxLevel
}

func (csa *CodeSmellAnalyzer) isLazyErrorCheck(stmt *ast.IfStmt) bool {
	if binExpr, ok := stmt.Cond.(*ast.BinaryExpr); ok {
		if binExpr.Op == token.NEQ {
			if ident, ok := binExpr.X.(*ast.Ident); ok && ident.Name == "err" {
				if ident, ok := binExpr.Y.(*ast.Ident); ok && ident.Name == "nil" {
					return true
				}
			}
		}
	}
	return false
}

// isLibraryGlobalVariable checks if a global variable is a common library pattern
func isLibraryGlobalVariable(name string) bool {
	// Common library global variable patterns that are acceptable
	commonPatterns := []string{
		"Version", "BuildDate", "GitCommit", // Version info
		"DefaultConfig", "Default", // Default configurations
		"NoOp", "Discard", "Null", // Null objects
		"ErrNotFound", "ErrInvalid", "Err", // Error constants
		"Encoder", "Decoder", "Parser", // Shared instances
		"Registry", "Manager", "Pool", // Registries and pools
		"JSON", "XML", "YAML", "TOML", // Format parsers
		"Logger", "Log", // Loggers
		"Client", "Server", // Default clients
		"Reader", "Writer", // IO objects
	}

	for _, pattern := range commonPatterns {
		if strings.Contains(name, pattern) {
			return true
		}
	}

	// Allow constants-like naming (ALL_CAPS)
	if strings.ToUpper(name) == name && len(name) > 2 {
		return true
	}

	return false
}
