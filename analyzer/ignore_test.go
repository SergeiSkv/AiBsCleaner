package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

func parseFile(t *testing.T, src string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	require.NoError(t, err)
	return fset, file
}

func TestIgnoreCheckerGeneralDirective(t *testing.T) {
	src := `package sample

// abc:ignore
func foo() {}
`
	fset, file := parseFile(t, src)
	checker := NewIgnoreChecker(fset, file)

	require.True(t, checker.ShouldIgnore(models.IssueHTTPNoTimeout.String(), 4))
	require.False(t, checker.ShouldIgnore(models.IssueHTTPNoTimeout.String(), 5))
}

func TestIgnoreCheckerSpecificDirectives(t *testing.T) {
	src := `package sample

// abc:ignore-next-line RaceCondition,GoroutineLeak
func first() {}

func second() {
	// abc:ignore-line RaceCondition
}

func third() {
	/* abc:ignore RegexInLoop */
}
`
	fset, file := parseFile(t, src)
	checker := NewIgnoreChecker(fset, file)

	expect := map[string][][2]int{
		models.IssueRaceCondition.String(): {{4, 4}, {7, 7}},
		models.IssueRegexInLoop.String():   {{12, 12}},
	}

	for key, want := range expect {
		ranges := checker.ignoreRanges[key]
		require.Len(t, ranges, len(want), key)
		for i, rg := range ranges {
			require.Equal(t, want[i][0], rg.startLine)
			require.Equal(t, want[i][1], rg.endLine)
		}
	}
	require.NotContains(t, checker.ignoreRanges, models.IssueHTTPNoTimeout.String())
}

func TestIgnoreCheckerLoopSpecificRange(t *testing.T) {
	src := `package sample

// abc:ignore STRING_CONCAT_IN_LOOP
func loop() {}
`
	fset, file := parseFile(t, src)
	checker := NewIgnoreChecker(fset, file)

	for line := 4; line <= 13; line++ {
		require.True(t, checker.ShouldIgnore("STRING_CONCAT_IN_LOOP", line))
	}
	require.False(t, checker.ShouldIgnore("STRING_CONCAT_IN_LOOP", 14))
}

func TestIgnoreCheckerFileWide(t *testing.T) {
	src := `package sample

// abc:ignore-file *
func foo() {}
`
	fset, file := parseFile(t, src)
	checker := NewIgnoreChecker(fset, file)

	require.True(t, checker.ShouldIgnore(models.IssueHTTPNoTimeout.String(), 4))
	require.True(t, checker.ShouldIgnore(models.IssueRaceCondition.String(), 100))
}

func TestFilterIssuesByComments(t *testing.T) {
	src := `package sample

// abc:ignore HTTPNoTimeout
func withIgnore() {}

func withoutIgnore() {}
`
	fset, file := parseFile(t, src)

	issues := []*models.Issue{
		{Line: 4, Type: models.IssueHTTPNoTimeout},
		{Line: 6, Type: models.IssueRaceCondition},
	}

	filtered := FilterIssuesByComments(issues, fset, file)
	require.Len(t, filtered, 1)
	require.Equal(t, 6, filtered[0].Line)
}

func TestFilterIssuesWithNilInputs(t *testing.T) {
	issues := []*models.Issue{{Line: 10, Type: models.IssueHTTPNoTimeout}}

	require.Equal(t, issues, FilterIssuesByComments(issues, nil, nil))
}
