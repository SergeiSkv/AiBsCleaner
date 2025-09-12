package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// IOBufferAnalyzer detects inefficient I/O operations and missing buffering
type IOBufferAnalyzer struct{}

func NewIOBufferAnalyzer() Analyzer {
	return &IOBufferAnalyzer{}
}

func (ia *IOBufferAnalyzer) Name() string {
	return "IOBufferAnalyzer"
}

func (ia *IOBufferAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
	var issues []*Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Get filename from the first position we encounter
	filename := ""
	if astNode.Pos().IsValid() {
		filename = fset.Position(astNode.Pos()).Filename
	}

	// Use context helper for proper loop detection
	ctx := NewAnalyzerWithContext(astNode)

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			node, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			pos := fset.Position(node.Pos())
			loopDepth := ctx.GetNodeLoopDepth(node)
			inLoop := loopDepth > 0

			// Check all I/O issues using helper functions
			ia.checkIOIssues(node, filename, pos, inLoop, &issues)

			return true
		},
	)

	return issues
}

func (ia *IOBufferAnalyzer) checkIOIssues(node *ast.CallExpr, filename string, pos token.Position, inLoop bool, issues *[]*Issue) {
	// Check for unbuffered file operations
	if ia.isUnbufferedFileOp(node) {
		*issues = append(*issues, &Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueUnbufferedIO,
			Severity:   SeverityLevelMedium,
			Message:    "Direct file I/O without buffering is inefficient",
			Suggestion: "Wrap with bufio.Reader/Writer for better performance",
			WhyBad: `Unbuffered I/O problems:
• System call for every Read/Write (~1μs each)
• No read-ahead optimization
• Poor performance on small operations
IMPACT: 10-100x slower than buffered I/O
BETTER: Use bufio.NewReader/NewWriter with appropriate buffer size`,
		})
	}

	// Check for single-byte reads/writes
	if ia.isSingleByteIO(node) {
		severity := SeverityLevelMedium
		if inLoop {
			severity = SeverityLevelHigh
		}
		*issues = append(*issues, &Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueUnbufferedIO,
			Severity:   severity,
			Message:    "Single-byte I/O operations are extremely inefficient",
			Suggestion: "Use buffered I/O or read/write larger chunks",
			WhyBad: `Single-byte I/O overhead:
• System call for each byte (~1μs)
• Context switch overhead
• No CPU cache benefits
IMPACT: 1000x slower than buffered reads
EXAMPLE: Reading 1MB byte-by-byte = 1 million syscalls`,
		})
	}

	// Check for ReadFile/WriteFile in loops
	if ia.isFileReadWriteInLoop(node) && inLoop {
		*issues = append(*issues, &Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueUnbufferedIO,
			Severity:   SeverityLevelHigh,
			Message:    "File open/read/write in loop - extremely inefficient",
			Suggestion: "Open file once, use it multiple times, or batch operations",
			WhyBad: `File operations in loops:
• File open/close overhead (~10-100μs)
• Destroys OS file cache benefits
• File descriptor exhaustion risk
BETTER: Open once, operate many times`,
		})
	}

	// Check other I/O issues
	ia.checkStringAndFormatIssues(node, filename, pos, inLoop, issues)
	ia.checkBufferIssues(node, filename, pos, issues)
	ia.checkFileOperationIssues(node, filename, pos, issues)
}

func (ia *IOBufferAnalyzer) checkStringAndFormatIssues(node *ast.CallExpr, filename string, pos token.Position, inLoop bool, issues *[]*Issue) {
	// Check for inefficient string building from I/O
	if ia.isInefficientStringIO(node) {
		*issues = append(*issues, &Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueUnbufferedIO,
			Severity:   SeverityLevelMedium,
			Message:    "Building strings with += from I/O causes allocations",
			Suggestion: "Use strings.Builder or bytes.Buffer",
			WhyBad: `String concatenation from I/O:
• Allocates new string on each +=
• Copies all previous data
• O(n²) complexity for n operations
BETTER: strings.Builder (3-5x faster)`,
		})
	}

	// Check for fmt.Fprintf in hot paths
	if ia.isFmtInHotPath(node) && inLoop {
		*issues = append(*issues, &Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueUnbufferedIO,
			Severity:   SeverityLevelLow,
			Message:    "fmt.Fprintf in loop has reflection overhead",
			Suggestion: "Use Writer.Write with []byte for hot paths",
			WhyBad: `fmt package overhead:
• Reflection for type checking (~100ns)
• Interface allocations
• Parsing format strings
IN LOOPS: Use direct Write methods`,
		})
	}
}

func (ia *IOBufferAnalyzer) checkBufferIssues(node *ast.CallExpr, filename string, pos token.Position, issues *[]*Issue) {
	// Check for Scanner without buffer size
	if ia.isScannerWithoutBuffer(node) {
		*issues = append(*issues, &Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueUnbufferedIO,
			Severity:   SeverityLevelLow,
			Message:    "Scanner using default buffer may be too small for large lines",
			Suggestion: "Set scanner.Buffer() for large input lines",
		})
	}

	// Check for io.Copy without buffer size hint
	if ia.isCopyWithoutBuffer(node) {
		*issues = append(*issues, &Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueUnbufferedIO,
			Severity:   SeverityLevelLow,
			Message:    "io.Copy uses 32KB buffer by default",
			Suggestion: "Use io.CopyBuffer with sized buffer for large transfers",
			WhyBad: `io.Copy default buffer:
• 32KB may be suboptimal for your use case
• Network: larger buffers reduce syscalls
• SSD: larger buffers improve throughput
TUNE: Based on your I/O patterns`,
		})
	}

	// Check for small buffer sizes
	if ia.hasSmallBuffer(node) {
		*issues = append(*issues, &Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueUnbufferedIO,
			Severity:   SeverityLevelLow,
			Message:    "Buffer size < 4KB is inefficient for most I/O",
			Suggestion: "Use at least 4KB buffers (matches OS page size)",
			WhyBad: `Small buffer problems:
• More system calls
• Poor CPU cache utilization
• OS typically uses 4KB pages
RECOMMENDED: 4KB-64KB for most cases`,
		})
	}
}

func (ia *IOBufferAnalyzer) checkFileOperationIssues(node *ast.CallExpr, filename string, pos token.Position, issues *[]*Issue) {
	// Check for reading entire file into memory
	if ia.isReadingEntireFile(node) {
		*issues = append(*issues, &Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueUnbufferedIO,
			Severity:   SeverityLevelMedium,
			Message:    "Reading entire file into memory with ReadFile",
			Suggestion: "Use streaming I/O for large files",
			WhyBad: `Reading entire file problems:
• Memory usage equals file size
• OOM risk for large files
• No progressive processing
• GC pressure from large allocation
BETTER: Process in chunks with bufio`,
		})
	}

	// Check for Flush missing after buffered writes
	if ia.isMissingFlush(node) {
		*issues = append(*issues, &Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueUnbufferedIO,
			Severity:   SeverityLevelHigh,
			Message:    "Buffered writer without Flush() may lose data",
			Suggestion: "Always call Flush() on buffered writers, preferably with defer",
		})
	}

	// Check for sync.Once file operations
	if ia.needsSyncOnce(node) {
		*issues = append(*issues, &Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       IssueUnbufferedIO,
			Severity:   SeverityLevelMedium,
			Message:    "File opened multiple times for same operation",
			Suggestion: "Use sync.Once for one-time file operations",
		})
	}
}

func (ia *IOBufferAnalyzer) isUnbufferedFileOp(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		// Check for direct os.File operations
		if ident, ok := sel.X.(*ast.Ident); ok {
			// Look for file.Read, file.Write without bufio
			if strings.Contains(ident.Name, "file") || strings.Contains(ident.Name, "File") {
				method := sel.Sel.Name
				return method == methodRead || method == methodWrite || method == "ReadAt" || method == "WriteAt"
			}
		}
	}
	return false
}

func (ia *IOBufferAnalyzer) isSingleByteIO(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	method := sel.Sel.Name

	// Check for ReadByte/WriteByte methods
	if method == "ReadByte" || method == "WriteByte" {
		return ia.isNotBufferedIO(sel)
	}

	// Check for Read/Write that could be single-byte operations
	if method == methodRead || method == methodWrite {
		return ia.isLikelyFileOperation(sel)
	}

	return false
}

func (ia *IOBufferAnalyzer) isNotBufferedIO(sel *ast.SelectorExpr) bool {
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return true // Assume unbuffered if we can't determine
	}
	return !strings.Contains(ident.Name, "buf")
}

func (ia *IOBufferAnalyzer) isLikelyFileOperation(sel *ast.SelectorExpr) bool {
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	// Simple heuristic: assume single-byte if it looks like a file operation
	return strings.Contains(strings.ToLower(ident.Name), "file")
}

func (ia *IOBufferAnalyzer) isFileReadWriteInLoop(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	if ident.Name == pkgOS {
		method := sel.Sel.Name
		return method == methodOpen || method == methodCreate || method == "OpenFile"
	}

	if ident.Name == pkgIOutil || ident.Name == pkgIO {
		method := sel.Sel.Name
		return method == methodReadFile || method == "WriteFile"
	}

	return false
}

func (ia *IOBufferAnalyzer) isInefficientStringIO(call *ast.CallExpr) bool {
	// Check for string concatenation with I/O results
	// This would need data flow analysis for accuracy
	return false
}

func (ia *IOBufferAnalyzer) isFmtInHotPath(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == pkgFmt {
			method := sel.Sel.Name
			return strings.HasPrefix(method, "Fprint")
		}
	}
	return false
}

func (ia *IOBufferAnalyzer) isScannerWithoutBuffer(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "bufio" {
			return sel.Sel.Name == "NewScanner"
		}
	}
	return false
}

func (ia *IOBufferAnalyzer) isCopyWithoutBuffer(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == pkgIO {
			return sel.Sel.Name == "Copy"
		}
	}
	return false
}

func (ia *IOBufferAnalyzer) isReadingEntireFile(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			if ident.Name == pkgIOutil || ident.Name == pkgOS {
				method := sel.Sel.Name
				return method == methodReadFile || method == methodReadAll
			}
		}
	}
	return false
}

func (ia *IOBufferAnalyzer) isMissingFlush(call *ast.CallExpr) bool {
	// Would need to track bufio.Writer creation and usage
	// Simplified check
	return false
}

func (ia *IOBufferAnalyzer) hasSmallBuffer(call *ast.CallExpr) bool {
	// Check for buffer size in make() or NewReaderSize/NewWriterSize
	if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == funcMake {
		if len(call.Args) >= 2 {
			// Check if it's []byte with small size
			if lit, ok := call.Args[1].(*ast.BasicLit); ok {
				// Would need to evaluate the literal value
				return strings.Contains(lit.Value, "256") ||
					strings.Contains(lit.Value, "512") ||
					strings.Contains(lit.Value, "1024")
			}
		}
	}
	return false
}

func (ia *IOBufferAnalyzer) needsSyncOnce(call *ast.CallExpr) bool {
	// Would need to track repeated file operations
	// Simplified version
	return false
}
