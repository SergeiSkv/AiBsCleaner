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
			text := c.Text

			// Remove comment markers
			if strings.HasPrefix(text, "//") {
				text = strings.TrimPrefix(text, "//")
			} else if strings.HasPrefix(text, "/*") {
				text = strings.TrimPrefix(text, "/*")
				text = strings.TrimSuffix(text, "*/")
			}
			text = strings.TrimSpace(text)

			// Check for abc:ignore directives
			if strings.HasPrefix(text, "abc:ignore") {
				pos := ic.fset.Position(c.Pos())
				line := pos.Line

				// Parse the ignore directive
				parts := strings.Split(text, " ")
				if len(parts) == 1 {
					// abc:ignore - ignores next line for all issue types
					ir := ignoreRange{
						startLine: line + 1,
						endLine:   line + 1,
						issueType: "",
					}
					ic.ignoreRanges[""] = append(ic.ignoreRanges[""], ir)
				} else {
					// abc:ignore TYPE - ignores next line for specific type
					// abc:ignore-line TYPE - ignores current line for specific type
					// abc:ignore-next-line TYPE - ignores next line for specific type
					// abc:ignore-file TYPE - ignores entire file for specific type

					directive := parts[0]
					var issueTypes []string
					if len(parts) > 1 {
						issueTypes = strings.Split(parts[1], ",")
					}

					switch directive {
					case "abc:ignore-line":
						for _, issueType := range issueTypes {
							ir := ignoreRange{
								startLine: line,
								endLine:   line,
								issueType: strings.TrimSpace(issueType),
							}
							ic.ignoreRanges[ir.issueType] = append(ic.ignoreRanges[ir.issueType], ir)
						}
					case "abc:ignore-next-line":
						for _, issueType := range issueTypes {
							ir := ignoreRange{
								startLine: line + 1,
								endLine:   line + 1,
								issueType: strings.TrimSpace(issueType),
							}
							ic.ignoreRanges[ir.issueType] = append(ic.ignoreRanges[ir.issueType], ir)
						}
					case "abc:ignore-file":
						for _, issueType := range issueTypes {
							if issueType == "" {
								issueType = "*" // Ignore all types for entire file
							}
							ir := ignoreRange{
								startLine: 0,
								endLine:   999999, // Large number to cover entire file
								issueType: strings.TrimSpace(issueType),
							}
							ic.ignoreRanges[ir.issueType] = append(ic.ignoreRanges[ir.issueType], ir)
						}
					case "abc:ignore":
						// Default behavior - ignore next line and a range of lines for loop issues
						if len(issueTypes) == 0 {
							// No specific types, ignore all on next line
							ir := ignoreRange{
								startLine: line + 1,
								endLine:   line + 1,
								issueType: "",
							}
							ic.ignoreRanges[""] = append(ic.ignoreRanges[""], ir)
						} else {
							for _, issueType := range issueTypes {
								issueType = strings.TrimSpace(issueType)
								// For loop-related issues, ignore a range of lines
								if strings.Contains(issueType, "_IN_LOOP") || strings.Contains(issueType, "LOOP_") {
									ir := ignoreRange{
										startLine: line + 1,
										endLine:   line + 10, // Cover the loop body
										issueType: issueType,
									}
									ic.ignoreRanges[issueType] = append(ic.ignoreRanges[issueType], ir)
								} else {
									ir := ignoreRange{
										startLine: line + 1,
										endLine:   line + 1,
										issueType: issueType,
									}
									ic.ignoreRanges[issueType] = append(ic.ignoreRanges[issueType], ir)
								}
							}
						}
					}
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
