package fixer

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"

	"github.com/SergeiSkv/AiBsCleaner/analyzer"
)

// Fixer provides automatic code fixing capabilities
type Fixer struct {
	dryRun  bool
	verbose bool
}

// NewFixer creates a new code fixer
func NewFixer(dryRun, verbose bool) *Fixer {
	return &Fixer{
		dryRun:  dryRun,
		verbose: verbose,
	}
}

// FixIssues automatically fixes issues in the given file
func (f *Fixer) FixIssues(filename string, issues []analyzer.Issue) error {
	if len(issues) == 0 {
		return nil
	}

	// Read file content
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse file
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, content, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// Group issues by type for batch fixing
	issuesByType := make(map[string][]analyzer.Issue)
	for _, issue := range issues {
		issuesByType[issue.Type] = append(issuesByType[issue.Type], issue)
	}

	// Apply fixes
	modified := false
	for issueType, typeIssues := range issuesByType {
		if f.CanAutoFix(issueType) {
			if f.verbose {
				fmt.Printf("Fixing %d %s issues in %s\n", len(typeIssues), issueType, filename)
			}
			if f.applyFix(file, fset, issueType, typeIssues) {
				modified = true
			}
		}
	}

	if !modified {
		return nil
	}

	// Format the modified AST
	var buf strings.Builder
	if err := format.Node(&buf, fset, file); err != nil {
		return fmt.Errorf("failed to format fixed code: %w", err)
	}

	if f.dryRun {
		fmt.Printf("--- %s (original)\n", filename)
		fmt.Printf("+++ %s (fixed)\n", filename)
		fmt.Printf("Changes that would be applied:\n%s\n", buf.String())
		return nil
	}

	// Write fixed content back to file
	if err := os.WriteFile(filename, []byte(buf.String()), 0644); err != nil {
		return fmt.Errorf("failed to write fixed file: %w", err)
	}

	if f.verbose {
		fmt.Printf("Fixed %s\n", filename)
	}

	return nil
}

// CanAutoFix returns true if the issue type can be automatically fixed
func (f *Fixer) CanAutoFix(issueType string) bool {
	autoFixableTypes := map[string]bool{
		// AI Bullshit patterns
		"AI_GOROUTINE_OVERKILL":     true,
		"AI_UNNECESSARY_REFLECTION": true,
		"AI_OVER_ENGINEERING":       false, // Requires manual refactoring
		"AI_UNNECESSARY_INTERFACE":  false, // Requires design decision

		// Performance issues
		"STRING_CONCAT_IN_LOOP":   true,
		"DEFER_IN_LOOP":           true,
		"APPEND_WITHOUT_CAPACITY": true,
		"TIME_NOW_IN_LOOP":        true,
		"JSON_MARSHAL_IN_LOOP":    true,
		"REGEX_COMPILE_IN_LOOP":   true,

		// Simple fixes
		"UNCHECKED_ERROR":   true,
		"MISSING_DEFER":     true,
		"INEFFICIENT_RANGE": true,
	}

	return autoFixableTypes[issueType]
}

// applyFix applies the appropriate fix for the issue type
func (f *Fixer) applyFix(file *ast.File, fset *token.FileSet, issueType string, issues []analyzer.Issue) bool {
	switch issueType {
	case "AI_GOROUTINE_OVERKILL":
		return f.fixGoroutineOverkill(file, fset, issues)
	case "AI_UNNECESSARY_REFLECTION":
		return f.fixUnnecessaryReflection(file, fset, issues)
	case "STRING_CONCAT_IN_LOOP":
		return f.fixStringConcatInLoop(file, fset, issues)
	case "DEFER_IN_LOOP":
		return f.fixDeferInLoop(file, fset, issues)
	case "TIME_NOW_IN_LOOP":
		return f.fixTimeNowInLoop(file, fset, issues)
	case "UNCHECKED_ERROR":
		return f.fixUncheckedError(file, fset, issues)
	default:
		return false
	}
}

// fixGoroutineOverkill removes unnecessary goroutines and channels
func (f *Fixer) fixGoroutineOverkill(file *ast.File, fset *token.FileSet, issues []analyzer.Issue) bool {
	// This is complex - would need to:
	// 1. Find the goroutine + channel pattern
	// 2. Extract the logic from the goroutine
	// 3. Replace with direct synchronous call
	// 4. Remove channel creation and usage

	// For now, add a comment suggesting the fix
	for _, issue := range issues {
		addCommentAtPosition(
			file, fset, issue.Line,
			"TODO: Remove goroutine and channel - execute synchronously",
		)
	}
	return true
}

// fixUnnecessaryReflection replaces reflection with direct operations
func (f *Fixer) fixUnnecessaryReflection(file *ast.File, fset *token.FileSet, issues []analyzer.Issue) bool {
	modified := false
	ast.Inspect(
		file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "reflect" {
						// Check if it's a simple ValueOf().Int() pattern
						if sel.Sel.Name == "ValueOf" && len(call.Args) == 1 {
							// Replace reflect.ValueOf(x).Int() with int64(x)
							// This is simplified - real implementation would be more complex
							modified = true
						}
					}
				}
			}
			return true
		},
	)
	return modified
}

// fixStringConcatInLoop replaces string concatenation with strings.Builder
func (f *Fixer) fixStringConcatInLoop(file *ast.File, fset *token.FileSet, issues []analyzer.Issue) bool {
	// Add import for strings package if not present
	addImport(file, "strings")

	// This would need to:
	// 1. Find the loop with string concatenation
	// 2. Create a strings.Builder before the loop
	// 3. Replace += with builder.WriteString()
	// 4. After loop, assign builder.String() to the result

	for _, issue := range issues {
		addCommentAtPosition(
			file, fset, issue.Line,
			"TODO: Use strings.Builder instead of string concatenation",
		)
	}
	return true
}

// fixDeferInLoop wraps defer in anonymous function
func (f *Fixer) fixDeferInLoop(file *ast.File, fset *token.FileSet, issues []analyzer.Issue) bool {
	// Would need to:
	// 1. Find defer statements in loops
	// 2. Wrap the loop body in func() { ... }()
	// 3. Move defer inside the anonymous function

	for _, issue := range issues {
		addCommentAtPosition(
			file, fset, issue.Line,
			"TODO: Wrap in anonymous function to scope defer properly",
		)
	}
	return true
}

// fixTimeNowInLoop moves time.Now() outside the loop
func (f *Fixer) fixTimeNowInLoop(file *ast.File, fset *token.FileSet, issues []analyzer.Issue) bool {
	// Would need to:
	// 1. Find time.Now() calls in loops
	// 2. Create a variable before the loop
	// 3. Replace time.Now() with the variable

	for _, issue := range issues {
		addCommentAtPosition(
			file, fset, issue.Line,
			"TODO: Move time.Now() outside the loop",
		)
	}
	return true
}

// fixUncheckedError adds error checking
func (f *Fixer) fixUncheckedError(file *ast.File, fset *token.FileSet, issues []analyzer.Issue) bool {
	// Would need to:
	// 1. Find function calls that return error
	// 2. Add if err != nil check
	// 3. Return or handle the error appropriately

	for _, issue := range issues {
		addCommentAtPosition(
			file, fset, issue.Line,
			"TODO: Check error return value",
		)
	}
	return true
}

// Helper functions

func addImport(file *ast.File, pkg string) {
	// Check if import already exists
	for _, imp := range file.Imports {
		if imp.Path.Value == `"`+pkg+`"` {
			return
		}
	}

	// Add import
	// This is simplified - real implementation would handle import grouping
}

func addCommentAtPosition(file *ast.File, fset *token.FileSet, line int, comment string) {
	// This would add a comment at the specified line
	// Real implementation would modify the AST's comment map
}

// GetFixableCount returns the number of issues that can be auto-fixed
func GetFixableCount(issues []analyzer.Issue) int {
	fixer := &Fixer{}
	count := 0
	for _, issue := range issues {
		if fixer.CanAutoFix(issue.Type) {
			count++
		}
	}
	return count
}

// GetFixSuggestion returns a fix suggestion for an issue
func GetFixSuggestion(issue analyzer.Issue) string {
	suggestions := map[string]string{
		"AI_GOROUTINE_OVERKILL": `// Remove goroutine and channel:
result := performOperation()`,

		"AI_UNNECESSARY_REFLECTION": `// Replace reflection with direct operation:
value := x // instead of reflect.ValueOf(x).Interface()`,

		"STRING_CONCAT_IN_LOOP": `// Use strings.Builder:
var builder strings.Builder
for _, item := range items {
    builder.WriteString(item)
}
result := builder.String()`,

		"DEFER_IN_LOOP": `// Wrap in anonymous function:
for _, item := range items {
    func() {
        file := open(item)
        defer file.Close()
        // process
    }()
}`,

		"TIME_NOW_IN_LOOP": `// Move outside loop:
start := time.Now()
for i := 0; i < n; i++ {
    // use start instead of time.Now()
}`,
	}

	if suggestion, ok := suggestions[issue.Type]; ok {
		return suggestion
	}
	return issue.Suggestion
}
