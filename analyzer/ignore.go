package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// IgnoreChecker checks if issues should be ignored based on comments
type IgnoreChecker struct {
	fset         *token.FileSet
	file         *ast.File
	ignoreRanges map[string][]ignoreRange // key is issue type, empty key means all types
}

type ignoreRange struct {
	startLine int
	endLine   int
	issueType string // empty means ignore all types
}

// NewIgnoreChecker creates a new ignore checker for a file
func NewIgnoreChecker(fset *token.FileSet, file *ast.File) *IgnoreChecker {
	ic := &IgnoreChecker{
		fset:         fset,
		file:         file,
		ignoreRanges: make(map[string][]ignoreRange),
	}
	ic.parseIgnoreComments()
	return ic
}

// parseIgnoreComments finds all abc:ignore comments in the file
func (ic *IgnoreChecker) parseIgnoreComments() {
	// Check all comment groups in the file
	for _, cg := range ic.file.Comments {
		for _, c := range cg.List {
			text := ic.extractCommentText(c.Text)
			if strings.HasPrefix(text, "abc:ignore") {
				ic.processIgnoreDirective(text, c.Pos())
			}
		}
	}
}

// extractCommentText removes comment markers and trims the text
func (ic *IgnoreChecker) extractCommentText(text string) string {
	if strings.HasPrefix(text, "//") {
		text = strings.TrimPrefix(text, "//")
	} else if strings.HasPrefix(text, "/*") {
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSuffix(text, "*/")
	}
	return strings.TrimSpace(text)
}

// processIgnoreDirective processes a single ignore directive
func (ic *IgnoreChecker) processIgnoreDirective(text string, pos token.Pos) {
	position := ic.fset.Position(pos)
	line := position.Line
	parts := strings.Split(text, " ")

	if len(parts) == 1 {
		// Simple abc:ignore - ignores next line for all issue types
		ic.addIgnoreRange("", line+1, line+1)
	} else {
		directive := parts[0]
		var issueTypes []string
		if len(parts) > 1 {
			issueTypes = strings.Split(parts[1], ",")
		}
		ic.processSpecificDirective(directive, issueTypes, line)
	}
}

// addIgnoreRange adds an ignore range for the given issue type
func (ic *IgnoreChecker) addIgnoreRange(issueType string, startLine, endLine int) {
	ir := ignoreRange{
		startLine: startLine,
		endLine:   endLine,
		issueType: issueType,
	}
	ic.ignoreRanges[issueType] = append(ic.ignoreRanges[issueType], ir)
}

// processSpecificDirective handles specific ignore directives
func (ic *IgnoreChecker) processSpecificDirective(directive string, issueTypes []string, line int) {
	switch directive {
	case "abc:ignore-line":
		for _, issueType := range issueTypes {
			ic.addIgnoreRange(strings.TrimSpace(issueType), line, line)
		}
	case "abc:ignore-next-line":
		for _, issueType := range issueTypes {
			ic.addIgnoreRange(strings.TrimSpace(issueType), line+1, line+1)
		}
	case "abc:ignore-file":
		for _, issueType := range issueTypes {
			if issueType == "" {
				issueType = "*" // Ignore all types for entire file
			}
			ic.addIgnoreRange(strings.TrimSpace(issueType), 0, 999999)
		}
	case "abc:ignore":
		// Default behavior
		if len(issueTypes) == 0 {
			ic.addIgnoreRange("", line+1, line+1)
		} else {
			for _, issueType := range issueTypes {
				issueType = strings.TrimSpace(issueType)
				// For loop-related issues, ignore a range of lines
				if strings.Contains(issueType, "_IN_LOOP") || strings.Contains(issueType, "LOOP_") {
					ic.addIgnoreRange(issueType, line+1, line+10)
				} else {
					ic.addIgnoreRange(issueType, line+1, line+1)
				}
			}
		}
	}
}

// ShouldIgnore checks if an issue at a specific line should be ignored
func (ic *IgnoreChecker) ShouldIgnore(issueType string, line int) bool {
	// Check for type-specific ignores
	if ranges, ok := ic.ignoreRanges[issueType]; ok {
		for _, r := range ranges {
			if line >= r.startLine && line <= r.endLine {
				return true
			}
		}
	}

	// Check for general ignores (all types)
	if ranges, ok := ic.ignoreRanges[""]; ok {
		for _, r := range ranges {
			if line >= r.startLine && line <= r.endLine {
				return true
			}
		}
	}

	// Check for file-wide ignore of all types
	if ranges, ok := ic.ignoreRanges["*"]; ok {
		for _, r := range ranges {
			if line >= r.startLine && line <= r.endLine {
				return true
			}
		}
	}

	return false
}

// FilterIssues removes issues that should be ignored based on comments
func FilterIssuesByComments(issues []Issue, fset *token.FileSet, file *ast.File) []Issue {
	if file == nil || fset == nil {
		return issues
	}

	ic := NewIgnoreChecker(fset, file)

	filtered := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		if !ic.ShouldIgnore(issue.Type, issue.Line) {
			filtered = append(filtered, issue)
		}
	}

	return filtered
}

// Example usage in comments:
//
// Ignore next line for all issue types:
// // abc:ignore
// result += "something"  // This line will be ignored
//
// Ignore next line for specific issue type:
// // abc:ignore STRING_CONCAT_IN_LOOP
// result += "something"  // Only STRING_CONCAT_IN_LOOP will be ignored
//
// Ignore multiple issue types:
// // abc:ignore STRING_CONCAT_IN_LOOP,DEFER_IN_LOOP
//
// Ignore current line:
// result += "something"  // abc:ignore-line STRING_CONCAT_IN_LOOP
//
// Ignore entire file for specific issues:
// // abc:ignore-file COMPLEXITY_HIGH
//
// Ignore entire file for all issues:
// // abc:ignore-file *
