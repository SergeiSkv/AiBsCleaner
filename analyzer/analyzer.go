package analyzer

import (
	"go/ast"
	"go/token"
	"hash/fnv"
	"strconv"
	"sync"
	"time"

	"github.com/SergeiSkv/AiBsCleaner/cache"
)

// AnalysisCache caches analysis results to avoid re-analyzing unchanged files
type AnalysisCache struct {
	mu      sync.RWMutex
	results map[string]CacheEntry
	maxAge  time.Duration
}

// Object pools for reducing allocations
var (
	issueSlicePool = sync.Pool{
		New: func() interface{} {
			// Return as interface{} for proper type assertion
			s := make([]*Issue, 0, 20)
			return &s
		},
	}
)

type CacheEntry struct {
	Hash      string
	Issues    []*Issue
	Timestamp time.Time
}

var (
	// Global hybrid cache instance
	globalHybridCache *cache.HybridCache

	// Legacy cache for compatibility
	globalCache = &AnalysisCache{
		results: make(map[string]CacheEntry),
		maxAge:  15 * time.Minute,
	}
)

func init() {
	// Initialize hybrid cache with 1000 items in memory
	var err error
	globalHybridCache, err = cache.NewHybridCache("", 1000)
	if err != nil {
		// Fallback to simple cache if hybrid cache fails
		globalHybridCache = nil
	}
}

// AnalyzeAll is the main entry point for code analysis with caching support
func AnalyzeAll(filename string, file *ast.File, fset *token.FileSet) []*Issue {
	return Analyze(filename, file, fset, nil)
}

// Analyze is the main entry point with configuration support
func Analyze(
	filename string, file *ast.File, fset *token.FileSet, enabledAnalyzers map[string]bool,
) []*Issue {
	// Check for nil input
	if file == nil {
		return []*Issue{}
	}

	// Check cache first
	if cachedIssues, ok := checkCache(filename, file); ok {
		return cachedIssues
	}

	// Get issue slice from pool with type assertion check
	poolItem := issueSlicePool.Get()
	issuesPtr, ok := poolItem.(*[]*Issue)
	if !ok {
		// Fallback if type assertion fails
		issues := make([]*Issue, 0, 20)
		issuesPtr = &issues
	}
	issues := (*issuesPtr)[:0]
	// Don't clear issues in defer - we're returning them!
	// Just return the slice to pool for reuse
	defer func() {
		issueSlicePool.Put(issuesPtr)
	}()

	// Build list of analyzers to run based on configuration
	type analyzerEntry struct {
		name string
		fn   func() Analyzer
	}

	allAnalyzers := []analyzerEntry{
		// Performance analyzers (unique to this tool)
		{"loop", NewLoopAnalyzer},
		{"deferoptimization", NewDeferOptimizationAnalyzer},
		{"slice", NewSliceAnalyzer},
		{"map", NewMapAnalyzer},
		{"reflection", NewReflectionAnalyzer},
		{"interface", NewInterfaceAnalyzer},
		{"regex", NewRegexAnalyzer},
		{"time", NewTimeAnalyzer},
		{"memoryleak", NewMemoryLeakAnalyzer},
		{"database", NewDatabaseAnalyzer},

		// Specialized analyzers (not covered by standard linters)
		{"apimisuse", NewAPIMisuseAnalyzer},
		{"aibullshit", NewAIBullshitAnalyzer},
		{"goroutine", NewGoroutineAnalyzer},
		{"nilptr", NewNilPtrAnalyzer},
		{"channel", NewChannelAnalyzer},
		{"httpclient", NewHTTPClientAnalyzer},
		{"context", NewContextAnalyzer},
		{"racecondition", NewRaceConditionAnalyzer},
		{"concurrencypatterns", NewConcurrencyPatternsAnalyzer},
		{"networkpatterns", NewNetworkPatternsAnalyzer},
		{"cpuoptimization", NewCPUOptimizationAnalyzer},
		{"gcpressure", NewGCPressureAnalyzer},
		{"syncpool", NewSyncPoolAnalyzer},

		// New performance analyzers
		{"cgo", NewCGOAnalyzer},
		{"serialization", NewSerializationAnalyzer},
		{"crypto", NewCryptoAnalyzer},
		{"httpreuse", NewHTTPReuseAnalyzer},
		{"iobuffer", NewIOBufferAnalyzer},

		// Security/privacy (specialized)
		{"privacy", NewPrivacyAnalyzer},

		// Testing (usually noisy, disabled by default in config)
		{"testcoverage", NewTestCoverageAnalyzer},
	}

	// Only create and run enabled analyzers
	analyzersRun := 0
	for _, entry := range allAnalyzers {
		// If no config provided, run all analyzers
		if enabledAnalyzers == nil || enabledAnalyzers[entry.name] {
			analyzer := entry.fn()
			analyzerIssues := analyzer.Analyze(file, fset)
			if len(analyzerIssues) > 0 {
				analyzersRun++
			}
			issues = append(issues, analyzerIssues...)
		}
	}

	// Update cache with results
	updateCache(filename, file, issues)

	return issues
}

func checkCache(filename string, file *ast.File) ([]*Issue, bool) {
	hash := computeFileHash(file)

	// Try hybrid cache first
	if globalHybridCache != nil {
		if entry, ok := globalHybridCache.Get(filename); ok {
			if entry.Hash == hash {
				// Convert cache.Issue back to analyzer.Issue
				issues := make([]*Issue, len(entry.Issues))
				for i, cacheIssue := range entry.Issues {
					issueType, _ := IssueTypeString(cacheIssue.Type)
					Severity, _ := SeverityLevelString(cacheIssue.Severity)
					issues[i] = &Issue{
						Type:       issueType,
						Severity:   Severity,
						Line:       cacheIssue.Line,
						Column:     cacheIssue.Column,
						Message:    cacheIssue.Message,
						Suggestion: cacheIssue.Suggestion,
					}
				}
				return issues, true
			}
		}
		return nil, false
	}

	// Fallback to legacy cache
	globalCache.mu.RLock()
	defer globalCache.mu.RUnlock()

	if entry, ok := globalCache.results[filename]; ok {
		if entry.Hash == hash && time.Since(entry.Timestamp) < globalCache.maxAge {
			return entry.Issues, true
		}
	}

	return nil, false
}

func updateCache(filename string, file *ast.File, issues []*Issue) {
	hash := computeFileHash(file)

	// Use hybrid cache if available
	if globalHybridCache != nil {
		// Convert analyzer.Issue to cache.Issue
		cacheIssues := make([]*cache.Issue, len(issues))
		for i, issue := range issues {
			cacheIssues[i] = &cache.Issue{
				Type:       string(rune(issue.Type)),
				Line:       issue.Line,
				Column:     issue.Column,
				Message:    issue.Message,
				Severity:   string(issue.Severity),
				Suggestion: issue.Suggestion,
			}
		}
		globalHybridCache.Put(
			filename, cache.Entry{
				Hash:   hash,
				Issues: cacheIssues,
			},
		)
		return
	}

	// Fallback to legacy cache
	globalCache.mu.Lock()
	defer globalCache.mu.Unlock()

	globalCache.results[filename] = CacheEntry{
		Hash:      hash,
		Issues:    issues,
		Timestamp: time.Now(),
	}

	// Clean old entries
	cleanCache()
}

// Buffer pool for hash computation
var hashBufferPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 256)
		return &b
	},
}

func computeFileHash(file *ast.File) string {
	h := fnv.New64a()
	bufInterface := hashBufferPool.Get()
	bufPtr, ok := bufInterface.(*[]byte)
	var buf []byte
	if ok && bufPtr != nil {
		buf = *bufPtr
	} else {
		buf = make([]byte, 0, 256)
	}
	buf = buf[:0]
	defer func() {
		b := buf[:0]
		hashBufferPool.Put(&b)
	}()

	nodeCount := 0
	ast.Inspect(
		file, func(n ast.Node) bool {
			if n == nil {
				return true
			}
			nodeCount++
			// Use position as part of hash
			pos := n.Pos()
			if pos != token.NoPos {
				// Reuse buffer for position encoding
				buf = strconv.AppendInt(buf[:0], int64(pos), 10)
				h.Write(buf)
			}
			return true
		},
	)
	// Add node count to hash
	buf = strconv.AppendInt(buf[:0], int64(nodeCount), 10)
	h.Write(buf)
	return strconv.FormatUint(h.Sum64(), 36) // Base36 for shorter string
}

func cleanCache() {
	now := time.Now()
	for key, entry := range globalCache.results {
		if now.Sub(entry.Timestamp) > globalCache.maxAge {
			delete(globalCache.results, key)
		}
	}
}

// WalkWithContext provides context-aware AST traversal
func WalkWithContext(node ast.Node, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	ctx := &AnalysisContext{
		Imports:   make(map[string]bool),
		FuncDecls: make(map[string]*ast.FuncDecl),
		TypeDecls: make(map[string]*ast.TypeSpec),
	}

	walkWithContext(node, ctx, fn)
}

func walkWithContext(node ast.Node, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	if node == nil {
		return
	}

	// Call the visitor function first
	if !fn(node, ctx) {
		return
	}

	// Dispatch to specific walker based on node type
	walkNode(node, ctx, fn)
}

func walkNode(node ast.Node, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	// Delegate to specialized handlers based on node category
	switch node := node.(type) {
	// Loop statements
	case *ast.ForStmt, *ast.RangeStmt:
		walkLoopNode(node, ctx, fn)
	// Declaration nodes
	case *ast.FuncDecl, *ast.GenDecl, *ast.ValueSpec:
		walkDeclNode(node, ctx, fn)
	// Statement nodes
	case *ast.BlockStmt, *ast.IfStmt, *ast.AssignStmt, *ast.ReturnStmt:
		walkStmtNode(node, ctx, fn)
	// Expression nodes
	case *ast.CallExpr, *ast.BinaryExpr, *ast.UnaryExpr, *ast.ParenExpr,
		*ast.SelectorExpr, *ast.IndexExpr, *ast.SliceExpr, *ast.ExprStmt:
		walkExprNode(node, ctx, fn)
	// Special case for file
	case *ast.File:
		walkFile(node, ctx, fn)
	}
}

func walkLoopNode(node ast.Node, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	switch n := node.(type) {
	case *ast.ForStmt:
		walkForStmt(n, ctx, fn)
	case *ast.RangeStmt:
		walkRangeStmt(n, ctx, fn)
	}
}

func walkDeclNode(node ast.Node, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	switch n := node.(type) {
	case *ast.FuncDecl:
		walkFuncDecl(n, ctx, fn)
	case *ast.GenDecl:
		walkGenDecl(n, ctx, fn)
	case *ast.ValueSpec:
		walkValueSpec(n, ctx, fn)
	}
}

func walkStmtNode(node ast.Node, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	switch n := node.(type) {
	case *ast.BlockStmt:
		walkBlockStmt(n, ctx, fn)
	case *ast.IfStmt:
		walkIfStmt(n, ctx, fn)
	case *ast.AssignStmt:
		walkAssignStmt(n, ctx, fn)
	case *ast.ReturnStmt:
		walkReturnStmt(n, ctx, fn)
	}
}

func walkExprNode(node ast.Node, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	switch n := node.(type) {
	case *ast.CallExpr:
		walkCallExpr(n, ctx, fn)
	case *ast.BinaryExpr:
		walkBinaryExpr(n, ctx, fn)
	case *ast.UnaryExpr, *ast.ParenExpr, *ast.SelectorExpr:
		walkExpr(node, ctx, fn)
	case *ast.IndexExpr:
		walkIndexExpr(n, ctx, fn)
	case *ast.SliceExpr:
		walkSliceExpr(n, ctx, fn)
	case *ast.ExprStmt:
		walkSimpleExpr(n.X, ctx, fn)
	}
}

func walkExpr(node ast.Node, ctx *AnalysisContext, fn func(n ast.Node, ctx *AnalysisContext) bool) {
	switch n := node.(type) {
	case *ast.UnaryExpr:
		walkSimpleExpr(n.X, ctx, fn)
	case *ast.ParenExpr:
		walkSimpleExpr(n.X, ctx, fn)
	case *ast.SelectorExpr:
		walkSimpleExpr(n.X, ctx, fn)
	}
}

// AnalysisContext provides shared context between analyzers
type AnalysisContext struct {
	Filename    string
	FileSet     *token.FileSet
	InLoop      bool
	LoopDepth   int
	CurrentFunc string

	// Shared state to avoid redundant checks
	Imports   map[string]bool
	FuncDecls map[string]*ast.FuncDecl
	TypeDecls map[string]*ast.TypeSpec

	// Performance metrics
	NodeCount int
	StartTime time.Time
}

// AnalyzeDependencies runs dependency analysis once for the entire project
func AnalyzeDependencies(projectPath string) []*Issue {
	analyzer := NewDependencyAnalyzer(projectPath)
	// Use an empty filename since this is project-level analysis
	return analyzer.Analyze(nil, nil)
}
