package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/SergeiSkv/AiBsCleaner/models"
	"github.com/stretchr/testify/require"
)

func TestConcurrency_WaitGroupAddInLoop(t *testing.T) {
	code := `package main
import "sync"
func run(wg *sync.WaitGroup, items []int) {
    for range items {
        wg.Add(1)
    }
}`

	issues := runConcurrencyAnalyzer(t, code)
	require.True(t, hasIssue(issues, models.IssueWaitGroupAddInLoop))
}

func TestConcurrency_WaitGroupAddOutsideLoopClean(t *testing.T) {
	code := `package main
import "sync"
func run() {
    var wg sync.WaitGroup
    wg.Add(1)
    go func() { defer wg.Done() }()
    wg.Wait()
}`

	issues := runConcurrencyAnalyzer(t, code)
	require.False(t, hasIssue(issues, models.IssueWaitGroupAddInLoop))
}

func TestConcurrency_GoroutineCapturesRangeVar(t *testing.T) {
	code := `package main
import "fmt"
func run(items []int) {
    for _, item := range items {
        go func() {
            fmt.Println(item)
        }()
    }
}`

	issues := runConcurrencyAnalyzer(t, code)
	require.True(t, hasIssue(issues, models.IssueGoroutineCapturesLoop))
}

func TestConcurrency_GoroutineCopiesRangeVarSafe(t *testing.T) {
	code := `package main
import "fmt"
func run(items []int) {
    for _, item := range items {
        go func(it int) {
            fmt.Println(it)
        }(item)
    }
}`

	issues := runConcurrencyAnalyzer(t, code)
	require.False(t, hasIssue(issues, models.IssueGoroutineCapturesLoop))
}

func TestConcurrency_ContextBackgroundInGoroutine(t *testing.T) {
	code := `package main
import "context"
func run() {
    go func() {
        ctx := context.Background()
        _ = ctx
    }()
}`

	issues := runConcurrencyAnalyzer(t, code)
	require.True(t, hasIssue(issues, models.IssueContextBackgroundInGoroutine))
}

func TestConcurrency_ContextBackgroundOutsideGoroutineAllowed(t *testing.T) {
	code := `package main
import "context"
func run() {
    ctx := context.Background()
    _ = ctx
}`

	issues := runConcurrencyAnalyzer(t, code)
	require.False(t, hasIssue(issues, models.IssueContextBackgroundInGoroutine))
}

func runConcurrencyAnalyzer(t *testing.T, code string) []*models.Issue {
	t.Helper()
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	analyzer := NewConcurrencyPatternsAnalyzer()
	return analyzer.Analyze(node, fset)
}

func hasIssue(issues []*models.Issue, issueType models.IssueType) bool {
	for _, issue := range issues {
		if issue.Type == issueType {
			return true
		}
	}
	return false
}
