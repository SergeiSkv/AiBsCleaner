package analyzer

import (
	"go/ast"
	"go/token"
	"sync"
)

// UnifiedWalker combines all analyzer passes into a single AST walk
type UnifiedWalker struct {
	analyzers []Analyzer
	issues    []*Issue
	mu        sync.Mutex
	fset      *token.FileSet
	filename  string
}

// NewUnifiedWalker creates a walker that runs all analyzers in a single pass
func NewUnifiedWalker(filename string, fset *token.FileSet) *UnifiedWalker {
	return &UnifiedWalker{
		filename: filename,
		fset:     fset,
		issues:   make([]*Issue, 0, 100), // Pre-allocate for common case
		analyzers: []Analyzer{
			// Performance analyzers
			NewLoopAnalyzer(),
			NewDeferOptimizationAnalyzer(),
			NewSliceAnalyzer(),
			NewMapAnalyzer(),
			NewReflectionAnalyzer(),
			NewInterfaceAnalyzer(),
			NewRegexAnalyzer(),
			NewTimeAnalyzer(),
			NewMemoryLeakAnalyzer(),
			NewDatabaseAnalyzer(),

			// Specialized analyzers
			NewAPIMisuseAnalyzer(),
			NewAIBullshitAnalyzer(),
			NewGoroutineAnalyzer(),
			NewNilPtrAnalyzer(),
			NewChannelAnalyzer(),
			NewHTTPClientAnalyzer(),

			// New performance analyzers
			NewCGOAnalyzer(),
			NewSerializationAnalyzer(),
			NewCryptoAnalyzer(),
			NewHTTPReuseAnalyzer(),
			NewIOBufferAnalyzer(),

			// Security/privacy
			NewPrivacyAnalyzer(),
		},
	}
}

// AnalyzeFile performs all analyses in a single AST walk
func (uw *UnifiedWalker) AnalyzeFile(file *ast.File) []*Issue {
	if file == nil {
		return []*Issue{}
	}

	// Create shared context for all analyzers
	ctx := &UnifiedContext{
		Filename:      uw.filename,
		FileSet:       uw.fset,
		Analyzers:     make(map[string]interface{}),
		LoopStack:     make([]*LoopInfo, 0, 4),
		FuncStack:     make([]*FuncInfo, 0, 4),
		Imports:       make(map[string]string, 10),
		DeferredCalls: make([]*ast.DeferStmt, 0, 10),
	}

	// Pre-process imports for context
	for _, imp := range file.Imports {
		if imp.Path == nil {
			continue
		}

		path := imp.Path.Value
		// Remove quotes
		if len(path) >= 2 {
			path = path[1 : len(path)-1]
		}

		if imp.Name != nil {
			ctx.Imports[imp.Name.Name] = path
		} else {
			// Extract package name from path
			lastSlash := 0
			for i := len(path) - 1; i >= 0; i-- {
				if path[i] == '/' {
					lastSlash = i + 1
					break
				}
			}
			ctx.Imports[path[lastSlash:]] = path
		}
	}

	// Single walk through the AST
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			// Exiting a node
			uw.handleNodeExit(ctx, n)
			return true
		}

		// Entering a node - update context
		uw.updateContext(ctx, n, true)

		// Let each analyzer check this node
		for _, analyzer := range uw.analyzers {
			if checker, ok := analyzer.(NodeChecker); ok {
				if issues := checker.CheckNode(n, ctx); len(issues) > 0 {
					uw.mu.Lock()
					uw.issues = append(uw.issues, issues...)
					uw.mu.Unlock()
				}
			}
		}

		return true
	})

	return uw.issues
}

func (uw *UnifiedWalker) updateContext(ctx *UnifiedContext, n ast.Node, entering bool) {
	switch node := n.(type) {
	case *ast.ForStmt, *ast.RangeStmt:
		if entering {
			ctx.LoopStack = append(ctx.LoopStack, &LoopInfo{
				Node:  n,
				Depth: len(ctx.LoopStack) + 1,
			})
		}
	case *ast.FuncDecl:
		if entering && node.Name != nil {
			ctx.FuncStack = append(ctx.FuncStack, &FuncInfo{
				Name: node.Name.Name,
				Decl: node,
			})
		}
	case *ast.DeferStmt:
		if entering {
			ctx.DeferredCalls = append(ctx.DeferredCalls, node)
		}
	}
}

func (uw *UnifiedWalker) handleNodeExit(ctx *UnifiedContext, n ast.Node) {
	if n == nil {
		return
	}

	switch n.(type) {
	case *ast.ForStmt, *ast.RangeStmt:
		if len(ctx.LoopStack) > 0 {
			ctx.LoopStack = ctx.LoopStack[:len(ctx.LoopStack)-1]
		}
	case *ast.FuncDecl:
		if len(ctx.FuncStack) > 0 {
			ctx.FuncStack = ctx.FuncStack[:len(ctx.FuncStack)-1]
		}
		// Clear deferred calls when exiting function
		ctx.DeferredCalls = ctx.DeferredCalls[:0]
	}
}

// UnifiedContext provides shared context for all analyzers in a single walk
type UnifiedContext struct {
	Filename      string
	FileSet       *token.FileSet
	LoopStack     []*LoopInfo
	FuncStack     []*FuncInfo
	Imports       map[string]string
	DeferredCalls []*ast.DeferStmt
	Analyzers     map[string]interface{} // Analyzer-specific data
}

// LoopInfo contains information about a loop
type LoopInfo struct {
	Node  ast.Node
	Depth int
}

// FuncInfo contains information about a function
type FuncInfo struct {
	Name string
	Decl *ast.FuncDecl
}

// InLoop returns true if currently inside a loop
func (uc *UnifiedContext) InLoop() bool {
	return len(uc.LoopStack) > 0
}

// LoopDepth returns the current loop nesting depth
func (uc *UnifiedContext) LoopDepth() int {
	return len(uc.LoopStack)
}

// CurrentFunc returns the current function name, if any
func (uc *UnifiedContext) CurrentFunc() string {
	if len(uc.FuncStack) > 0 {
		return uc.FuncStack[len(uc.FuncStack)-1].Name
	}
	return ""
}

// HasImport checks if a package is imported
func (uc *UnifiedContext) HasImport(pkg string) bool {
	_, ok := uc.Imports[pkg]
	return ok
}

// NodeChecker interface for analyzers that can check individual nodes
type NodeChecker interface {
	CheckNode(n ast.Node, ctx *UnifiedContext) []*Issue
}

// OptimizedAnalyze performs analysis using the unified walker
func OptimizedAnalyze(filename string, file *ast.File, fset *token.FileSet) []*Issue {
	if file == nil {
		return []*Issue{}
	}

	// Check cache first
	if cachedIssues, ok := checkCache(filename, file); ok {
		return cachedIssues
	}

	// Use unified walker for single-pass analysis
	walker := NewUnifiedWalker(filename, fset)
	issues := walker.AnalyzeFile(file)

	// Update cache with results
	updateCache(filename, file, issues)

	return issues
}
