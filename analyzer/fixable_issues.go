package analyzer

import "go/token"

// IsFixableIssue returns true if the issue type can be automatically fixed
func IsFixableIssue(issueType string) bool {
	fixableTypes := map[string]bool{
		// String operations
		"STRING_CONCAT_IN_LOOP": true,
		"STRING_CONCAT":         true,

		// Loop optimizations
		"DEFER_IN_LOOP":        true,
		"TIME_NOW_IN_LOOP":     true,
		"TIME_IN_LOOP":         true,
		"REGEX_IN_LOOP":        true,
		"REGEX_COMPILE":        true,
		"JSON_MARSHAL_IN_LOOP": true,
		"SQL_IN_LOOP":          true,
		"ALLOC_IN_LOOP":        true,

		// Slice and map operations
		"APPEND_WITHOUT_CAPACITY": true,
		"SLICE_CAPACITY":          true,
		"MAP_CAPACITY":            true,
		"SLICE_COPY":              true,
		"INEFFICIENT_RANGE":       true,
		"UNNECESSARY_COPY":        true,

		// Nil checking (not error handling - that's for errcheck linter)
		"NIL_CHECK": true,

		// Defer operations
		"MISSING_DEFER_CLOSE":  true,
		"MISSING_DEFER_UNLOCK": true,
		"MISSING_DEFER":        true,
		"DEFER_AT_END":         true,
		"UNNECESSARY_DEFER":    true,

		// Simple optimizations
		"BOUNDS_CHECK_ELIMINATION": true,
		"EMPTY_INTERFACE":          true,
		"INTERFACE_ALLOCATION":     true,

		// These require more complex analysis or LLM
		"AI_GOROUTINE_OVERKILL":    false, // Requires LLM
		"AI_BULLSHIT_CONCURRENCY":  false, // Requires LLM
		"AI_REFLECTION_OVERKILL":   false, // Requires LLM
		"AI_OVER_ENGINEERING":      false, // Requires manual refactoring
		"AI_UNNECESSARY_INTERFACE": false, // Requires design decision
		"HIGH_COMPLEXITY":          false, // Requires refactoring
		"GOROUTINE_LEAK":           false, // Complex analysis needed
		"RACE_CONDITION":           false, // Complex fix
		"MEMORY_LEAK":              false, // Complex analysis needed
	}

	canFix, exists := fixableTypes[issueType]
	return exists && canFix
}

// IsLLMFixableIssue returns true if the issue can be fixed with LLM assistance
func IsLLMFixableIssue(issueType string) bool {
	llmFixableTypes := map[string]bool{
		"AI_GOROUTINE_OVERKILL":     true,
		"AI_BULLSHIT_CONCURRENCY":   true,
		"AI_REFLECTION_OVERKILL":    true,
		"AI_PATTERN_ABUSE":          true,
		"AI_ENTERPRISE_HELLO_WORLD": true,
		"AI_OVERENGINEERED_SIMPLE":  true,
		"HIGH_COMPLEXITY":           true,
		"DUPLICATE_CODE":            true,
		"LONG_FUNCTION":             true,
	}

	canFix, exists := llmFixableTypes[issueType]
	return exists && canFix
}

// GetFixComplexity returns the complexity level of fixing an issue
func GetFixComplexity(issueType string) string {
	complexityMap := map[string]string{
		// Simple fixes
		"STRING_CONCAT_IN_LOOP":   "SIMPLE",
		"DEFER_IN_LOOP":           "SIMPLE",
		"TIME_NOW_IN_LOOP":        "SIMPLE",
		"APPEND_WITHOUT_CAPACITY": "SIMPLE",
		"MAP_CAPACITY":            "SIMPLE",
		"MISSING_DEFER_CLOSE":     "SIMPLE",
		"MISSING_DEFER_UNLOCK":    "SIMPLE",

		// Medium complexity
		"SLICE_COPY":           "MEDIUM",
		"INEFFICIENT_RANGE":    "MEDIUM",
		"REGEX_COMPILE":        "MEDIUM",
		"JSON_MARSHAL_IN_LOOP": "MEDIUM",
		"EMPTY_INTERFACE":      "MEDIUM",
		"INTERFACE_ALLOCATION": "MEDIUM",

		// Complex fixes
		"AI_GOROUTINE_OVERKILL":   "COMPLEX",
		"AI_BULLSHIT_CONCURRENCY": "COMPLEX",
		"AI_REFLECTION_OVERKILL":  "COMPLEX",
		"HIGH_COMPLEXITY":         "COMPLEX",
		"GOROUTINE_LEAK":          "COMPLEX",
		"RACE_CONDITION":          "COMPLEX",
		"MEMORY_LEAK":             "COMPLEX",
		"DUPLICATE_CODE":          "COMPLEX",
	}

	if complexity, exists := complexityMap[issueType]; exists {
		return complexity
	}
	return "UNKNOWN"
}

// CreateIssueWithFixability creates an issue with the CanBeFixed field properly set
func CreateIssueWithFixability(
	filename string,
	line int,
	column int,
	pos token.Position,
	issueType string,
	severity Severity,
	message string,
	suggestion string,
	code string,
) Issue {
	return Issue{
		File:       filename,
		Line:       line,
		Column:     column,
		Position:   pos,
		Type:       issueType,
		Severity:   severity,
		Message:    message,
		Suggestion: suggestion,
		Code:       code,
		CanBeFixed: IsFixableIssue(issueType),
	}
}
