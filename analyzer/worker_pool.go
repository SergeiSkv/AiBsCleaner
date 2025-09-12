package analyzer

import (
	"go/ast"
	"go/token"
	"runtime"
	"sync"
)

// WorkerPool manages concurrent file analysis
type WorkerPool struct {
	workers    int
	taskQueue  chan *AnalysisTask
	results    chan []*Issue
	wg         sync.WaitGroup
	bufferPool sync.Pool
}

// AnalysisTask represents a file to be analyzed
type AnalysisTask struct {
	Filename    string
	File        *ast.File
	FileSet     *token.FileSet
	ProjectPath string
}

// NewWorkerPool creates a new worker pool for parallel analysis
func NewWorkerPool(workers int) *WorkerPool {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	wp := &WorkerPool{
		workers:   workers,
		taskQueue: make(chan *AnalysisTask, workers*2),
		results:   make(chan []*Issue, workers*2),
		bufferPool: sync.Pool{
			New: func() interface{} {
				s := make([]*Issue, 0, 50)
				return &s
			},
		},
	}

	// Start workers
	for i := 0; i < workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}

	return wp
}

func (wp *WorkerPool) worker() {
	defer wp.wg.Done()

	for task := range wp.taskQueue {
		issues := wp.analyzeFile(task)
		if len(issues) > 0 {
			wp.results <- issues
		}
	}
}

func (wp *WorkerPool) analyzeFile(task *AnalysisTask) []*Issue {
	if task.File == nil {
		return nil
	}

	// Get buffer from pool
	issuesInterface := wp.bufferPool.Get()
	issuesPtr, ok := issuesInterface.(*[]*Issue)
	var issues []*Issue
	if ok && issuesPtr != nil {
		issues = *issuesPtr
	} else {
		issues = make([]*Issue, 0, 100)
	}
	defer func() {
		// Clear and return to pool
		cleared := issues[:0]
		wp.bufferPool.Put(&cleared)
	}()

	// Use optimized single-pass analysis
	walker := NewUnifiedWalker(task.Filename, task.FileSet)
	return walker.AnalyzeFile(task.File)
}

// Submit adds a task to the queue
func (wp *WorkerPool) Submit(task *AnalysisTask) {
	wp.taskQueue <- task
}

// Results returns the results channel
func (wp *WorkerPool) Results() <-chan []*Issue {
	return wp.results
}

// Close shuts down the worker pool
func (wp *WorkerPool) Close() {
	close(wp.taskQueue)
	wp.wg.Wait()
	close(wp.results)
}

// ParallelAnalyze analyzes multiple files concurrently
func ParallelAnalyze(files map[string]*ast.File, fset *token.FileSet, projectPath string) []*Issue {
	numWorkers := runtime.NumCPU()
	pool := NewWorkerPool(numWorkers)

	// Submit all tasks
	taskCount := 0
	for filename, file := range files {
		pool.Submit(
			&AnalysisTask{
				Filename:    filename,
				File:        file,
				FileSet:     fset,
				ProjectPath: projectPath,
			},
		)
		taskCount++
	}

	// Collect results
	var allIssues []*Issue
	resultsReceived := 0

	// Start result collector
	go func() {
		for issues := range pool.Results() {
			allIssues = append(allIssues, issues...)
			resultsReceived++
			if resultsReceived >= taskCount {
				break
			}
		}
	}()

	// Shutdown pool
	pool.Close()

	return allIssues
}

// BatchProcessor processes files in batches for memory efficiency
type BatchProcessor struct {
	batchSize  int
	workerPool *WorkerPool
	mu         sync.Mutex
	allIssues  []*Issue
}

// NewBatchProcessor creates a processor that handles files in batches
func NewBatchProcessor(batchSize, workers int) *BatchProcessor {
	if batchSize <= 0 {
		batchSize = 10
	}

	return &BatchProcessor{
		batchSize:  batchSize,
		workerPool: NewWorkerPool(workers),
		allIssues:  make([]*Issue, 0, 1000),
	}
}

// ProcessBatch processes a batch of files
func (bp *BatchProcessor) ProcessBatch(files map[string]*ast.File, fset *token.FileSet, projectPath string) {
	batch := make(map[string]*ast.File, bp.batchSize)

	for filename, file := range files {
		batch[filename] = file

		if len(batch) >= bp.batchSize {
			bp.processBatchInternal(batch, fset, projectPath)
			batch = make(map[string]*ast.File, bp.batchSize)
		}
	}

	// Process remaining files
	if len(batch) > 0 {
		bp.processBatchInternal(batch, fset, projectPath)
	}
}

func (bp *BatchProcessor) processBatchInternal(batch map[string]*ast.File, fset *token.FileSet, projectPath string) {
	var wg sync.WaitGroup
	results := make(chan []*Issue, len(batch))

	for filename, file := range batch {
		wg.Add(1)
		go func(fn string, f *ast.File) {
			defer wg.Done()
			task := &AnalysisTask{
				Filename:    fn,
				File:        f,
				FileSet:     fset,
				ProjectPath: projectPath,
			}
			issues := bp.workerPool.analyzeFile(task)
			if len(issues) > 0 {
				results <- issues
			}
		}(filename, file)
	}

	// Collect results in background
	go func() {
		wg.Wait()
		close(results)
	}()

	// Aggregate results
	for issues := range results {
		bp.mu.Lock()
		bp.allIssues = append(bp.allIssues, issues...)
		bp.mu.Unlock()
	}
}

// GetIssues returns all collected issues
func (bp *BatchProcessor) GetIssues() []*Issue {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	return bp.allIssues
}

// Close cleans up the batch processor
func (bp *BatchProcessor) Close() {
	bp.workerPool.Close()
}
