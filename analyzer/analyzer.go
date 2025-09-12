package analyzer

import (
	"go/ast"
	"go/token"
	"hash/fnv"
	"strconv"
	"sync"
	"time"

	"github.com/SergeiSkv/AiBsCleaner/cache"
	"github.com/SergeiSkv/AiBsCleaner/models"
)

type Analyzer interface {
	Name() string
	Analyze(node interface{}, fset *token.FileSet) []*models.Issue
}

// AnalysisCache caches analysis results to avoid re-analyzing unchanged files
type AnalysisCache struct {
	mu      sync.RWMutex
	results map[string]CacheEntry
	maxAge  time.Duration
}

type CacheEntry struct {
	Hash      string
	Issues    []*models.Issue
	Timestamp time.Time
}

var (
	// Global hybrid cache instance
	globalHybridCache *cache.HybridCache

	// Legacy cache for compatibility
	globalCache = &AnalysisCache{
		results: make(map[string]CacheEntry, 100),
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
func AnalyzeAll(filename string, file *ast.File, fset *token.FileSet) []*models.Issue {
	return Analyze(filename, file, fset, nil)
}

// Analyze is the main entry point with configuration support
func Analyze(
	filename string, file *ast.File, fset *token.FileSet, enabledAnalyzers map[string]bool,
) []*models.Issue {
	// Check for nil input
	if file == nil {
		return []*models.Issue{}
	}

	// TEMPORARY DISABLE: Show real issues
	// if isAnalyzerFile(filename) {
	//	return []*models.Issue{}
	// }

	// Check cache first
	if cachedIssues, ok := checkCache(filename, file); ok {
		return cachedIssues
	}

	issues := make([]*models.Issue, 0, 32)

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

		// Struct layout optimization
		{"structlayout", NewStructLayoutAnalyzer},

		// CPU cache optimization
		{"cpucache", NewCPUCacheAnalyzer},

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

func checkCache(filename string, file *ast.File) ([]*models.Issue, bool) {
	hash := computeFileHash(file)

	// Try hybrid cache first
	if globalHybridCache != nil {
		if entry, ok := globalHybridCache.Get(filename); ok {
			if entry.Hash == hash {
				return cloneIssues(entry.Issues), true
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

func updateCache(filename string, file *ast.File, issues []*models.Issue) {
	hash := computeFileHash(file)

	// Use hybrid cache if available
	if globalHybridCache != nil {
		globalHybridCache.Put(
			filename, cache.Entry{
				Hash:   hash,
				Issues: cloneIssues(issues),
			},
		)
	}

	// Fallback to legacy cache
	globalCache.mu.Lock()
	defer globalCache.mu.Unlock()

	globalCache.results[filename] = CacheEntry{
		Hash:      hash,
		Issues:    cloneIssues(issues),
		Timestamp: time.Now(),
	}

	// Clean old entries
	cleanCache()
}

func cloneIssues(src []*models.Issue) []*models.Issue {
	if len(src) == 0 {
		return []*models.Issue{}
	}

	cloned := make([]*models.Issue, 0, len(src))
	for _, issue := range src {
		if issue == nil {
			continue
		}
		issueCopy := *issue
		cloned = append(cloned, &issueCopy)
	}
	return cloned
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
	buf, ok := bufInterface.(*[]byte)
	if !ok || buf == nil {
		newBuf := make([]byte, 0, 256)
		buf = &newBuf
	}
	*buf = (*buf)[:0]
	defer func() {
		hashBufferPool.Put(buf)
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
				*buf = strconv.AppendInt((*buf)[:0], int64(pos), 10)
				h.Write(*buf)
			}
			return true
		},
	)
	// Add node count to hash
	*buf = strconv.AppendInt((*buf)[:0], int64(nodeCount), 10)
	h.Write(*buf)
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
		Imports:   make(map[string]bool, 20),
		FuncDecls: make(map[string]*ast.FuncDecl, 50),
		TypeDecls: make(map[string]*ast.TypeSpec, 20),
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
	// Performance metrics (8 bytes)
	StartTime time.Time
	// Pointer fields (8 bytes each)
	FileSet *token.FileSet
	// Map fields (8 bytes each)
	Imports   map[string]bool
	FuncDecls map[string]*ast.FuncDecl
	TypeDecls map[string]*ast.TypeSpec
	// String fields (16 bytes each)
	Filename    string
	CurrentFunc string
	// Integer fields (8 bytes each)
	LoopDepth int
	NodeCount int
	// Boolean field (1 byte) - placed at end to minimize padding
	InLoop bool
}

// AnalyzeDependencies runs dependency analysis once for the entire project
func AnalyzeDependencies(projectPath string) []*models.Issue {
	analyzer := NewDependencyAnalyzer(projectPath)
	// Use an empty filename since this is project-level analysis
	return analyzer.Analyze(nil, nil)
}
