package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"unsafe"
)

// CPUCacheAnalyzer detects CPU cache-unfriendly patterns and suggests optimizations
type CPUCacheAnalyzer struct {
	TypeInfo *types.Info
}

// NewCPUCacheAnalyzer creates a new CPU cache analyzer
func NewCPUCacheAnalyzer() Analyzer {
	return &CPUCacheAnalyzer{}
}

// Name returns the name of this analyzer
func (c *CPUCacheAnalyzer) Name() string {
	return "CPUCache"
}

// Analyze performs CPU cache analysis on the AST
func (c *CPUCacheAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
	file, ok := node.(*ast.File)
	if !ok || file == nil {
		return nil
	}

	var issues []*Issue

	ast.Inspect(
		file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.TypeSpec:
				if structType, ok := node.Type.(*ast.StructType); ok {
					structIssues := c.analyzeStructForCacheOptimization(node.Name.Name, structType, fset)
					issues = append(issues, structIssues...)
				}
			case *ast.FuncDecl:
				funcIssues := c.analyzeFunctionForCachePatterns(node, fset)
				issues = append(issues, funcIssues...)
			case *ast.RangeStmt:
				rangeIssues := c.analyzeRangeForCachePatterns(node, fset)
				issues = append(issues, rangeIssues...)
			}
			return true
		},
	)

	return issues
}

// analyzeStructForCacheOptimization analyzes struct layout for cache optimization
func (c *CPUCacheAnalyzer) analyzeStructForCacheOptimization(structName string, structType *ast.StructType, fset *token.FileSet) []*Issue {
	var issues []*Issue

	if structType.Fields == nil || len(structType.Fields.List) == 0 {
		return issues
	}

	// Check for potential false sharing
	issues = append(issues, c.checkFalseSharing(structName, structType, fset)...)

	// Check for cache line size optimization
	issues = append(issues, c.checkCacheLineSize(structName, structType, fset)...)

	// Check for data type optimization
	issues = append(issues, c.checkDataTypeOptimization(structName, structType, fset)...)

	// Check for struct of arrays vs array of structs pattern
	issues = append(issues, c.checkSoAvsAoS(structName, structType, fset)...)

	return issues
}

// checkFalseSharing detects potential false sharing issues
func (c *CPUCacheAnalyzer) checkFalseSharing(structName string, structType *ast.StructType, fset *token.FileSet) []*Issue {
	var issues []*Issue
	const cacheLineSize = 64 // bytes

	var currentOffset int64
	var concurrentFields []struct {
		name   string
		offset int64
		size   int64
	}

	// Calculate field offsets and identify concurrent access fields
	for _, field := range structType.Fields.List {
		fieldType := c.getFieldType(field.Type)
		size := c.getTypeSize(fieldType)
		alignment := c.getTypeAlignment(fieldType)

		// Add padding for alignment
		padding := (alignment - currentOffset%alignment) % alignment
		currentOffset += padding

		fieldName := c.getFieldName(field)

		// Check if field name suggests concurrent access (common patterns)
		if c.isConcurrentField(fieldName) {
			concurrentFields = append(
				concurrentFields, struct {
					name   string
					offset int64
					size   int64
				}{fieldName, currentOffset, size},
			)
		}

		currentOffset += size
	}

	// Check for false sharing between concurrent fields
	for i := 0; i < len(concurrentFields); i++ {
		for j := i + 1; j < len(concurrentFields); j++ {
			field1 := concurrentFields[i]
			field2 := concurrentFields[j]

			// Check if fields are in the same cache line
			cacheLine1 := field1.offset / cacheLineSize
			cacheLine2 := field2.offset / cacheLineSize

			if cacheLine1 == cacheLine2 {
				pos := fset.Position(structType.Pos())
				issue := &Issue{
					Type:     IssueCacheFalseSharing,
					Severity: SeverityLevelHigh,
					Line:     pos.Line,
					Column:   pos.Column,
					Message: fmt.Sprintf(
						"Potential false sharing in struct '%s': fields '%s' and '%s' share cache line %d",
						structName, field1.name, field2.name, cacheLine1,
					),
					Suggestion: "Add padding between concurrent-access fields or reorganize struct layout",
					Position:   pos,
				}
				issues = append(issues, issue)
			}
		}
	}

	return issues
}

// checkCacheLineSize checks if struct fits optimally in cache lines
func (c *CPUCacheAnalyzer) checkCacheLineSize(structName string, structType *ast.StructType, fset *token.FileSet) []*Issue {
	var issues []*Issue
	const cacheLineSize = 64 // bytes

	structSize := c.calculateStructSize(structType)

	// Warn if struct is just slightly larger than cache line multiples
	if structSize > cacheLineSize && structSize <= cacheLineSize+8 {
		wastedBytes := cacheLineSize - (structSize % cacheLineSize)
		if wastedBytes < 8 {
			pos := fset.Position(structType.Pos())
			issue := &Issue{
				Type:     IssueCacheLineWaste,
				Severity: SeverityLevelMedium,
				Line:     pos.Line,
				Column:   pos.Column,
				Message: fmt.Sprintf(
					"Struct '%s' (%d bytes) just exceeds cache line boundary by %d bytes",
					structName, structSize, structSize%cacheLineSize,
				),
				Suggestion: fmt.Sprintf(
					"Consider optimizing to fit in %d bytes (one cache line) or add explicit padding",
					cacheLineSize,
				),
				Position: pos,
			}
			issues = append(issues, issue)
		}
	}

	// Suggest cache line alignment for large structs
	if structSize >= cacheLineSize*2 {
		pos := fset.Position(structType.Pos())
		issue := &Issue{
			Type:     IssueCacheLineAlignment,
			Severity: SeverityLevelLow,
			Line:     pos.Line,
			Column:   pos.Column,
			Message: fmt.Sprintf(
				"Large struct '%s' (%d bytes) may benefit from cache line alignment",
				structName, structSize,
			),
			Suggestion: "Consider adding cache line alignment: `_ [64]byte` padding or using compiler directives",
			Position:   pos,
		}
		issues = append(issues, issue)
	}

	return issues
}

// checkDataTypeOptimization suggests using smaller data types when possible
func (c *CPUCacheAnalyzer) checkDataTypeOptimization(structName string, structType *ast.StructType, fset *token.FileSet) []*Issue {
	var issues []*Issue

	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue
		}

		fieldName := field.Names[0].Name
		fieldType := field.Type

		// Check for oversized integer types
		if ident, ok := fieldType.(*ast.Ident); ok {
			switch ident.Name {
			case "int64":
				if c.couldUseInt32(fieldName) {
					pos := fset.Position(field.Pos())
					issue := &Issue{
						Type:     IssueOversizedType,
						Severity: SeverityLevelLow,
						Line:     pos.Line,
						Column:   pos.Column,
						Message: fmt.Sprintf(
							"Field '%s' in struct '%s' uses int64, consider int32 if range allows",
							fieldName, structName,
						),
						Suggestion: "Use int32 instead of int64 if the value range permits, saves 4 bytes per field",
						Position:   pos,
					}
					issues = append(issues, issue)
				}
			case "int":
				pos := fset.Position(field.Pos())
				issue := &Issue{
					Type:     IssueUnspecificIntType,
					Severity: SeverityLevelLow,
					Line:     pos.Line,
					Column:   pos.Column,
					Message: fmt.Sprintf(
						"Field '%s' in struct '%s' uses unspecific 'int' type",
						fieldName, structName,
					),
					Suggestion: "Use specific sized type (int32/int64) for better cache optimization and portability",
					Position:   pos,
				}
				issues = append(issues, issue)
			}
		}
	}

	return issues
}

// checkSoAvsAoS suggests Struct of Arrays pattern when beneficial
func (c *CPUCacheAnalyzer) checkSoAvsAoS(structName string, structType *ast.StructType, fset *token.FileSet) []*Issue {
	var issues []*Issue

	// Count slice fields in struct
	sliceFields := 0
	totalFields := 0

	for _, field := range structType.Fields.List {
		totalFields += len(field.Names)
		if c.isSliceType(field.Type) {
			sliceFields += len(field.Names)
		}
	}

	// If struct has multiple slice fields, suggest SoA pattern
	if sliceFields >= 2 && totalFields >= 3 {
		pos := fset.Position(structType.Pos())
		issue := &Issue{
			Type:     IssueSoAPattern,
			Severity: SeverityLevelMedium,
			Line:     pos.Line,
			Column:   pos.Column,
			Message: fmt.Sprintf(
				"Struct '%s' has %d slice fields, consider Struct of Arrays (SoA) pattern",
				structName, sliceFields,
			),
			Suggestion: "Convert to SoA: separate slices for each field type for better cache locality during iteration",
			Position:   pos,
		}
		issues = append(issues, issue)
	}

	return issues
}

// analyzeFunctionForCachePatterns analyzes functions for cache-unfriendly patterns
func (c *CPUCacheAnalyzer) analyzeFunctionForCachePatterns(funcDecl *ast.FuncDecl, fset *token.FileSet) []*Issue {
	var issues []*Issue

	if funcDecl.Body == nil {
		return issues
	}

	// Look for nested loops that might cause cache misses
	ast.Inspect(
		funcDecl.Body, func(n ast.Node) bool {
			if rangeStmt, ok := n.(*ast.RangeStmt); ok {
				// Check for nested range loops over different data structures
				ast.Inspect(
					rangeStmt.Body, func(inner ast.Node) bool {
						if innerRange, ok := inner.(*ast.RangeStmt); ok {
							pos := fset.Position(innerRange.Pos())
							issue := &Issue{
								Type:       IssueNestedRangeCache,
								Severity:   SeverityLevelMedium,
								Line:       pos.Line,
								Column:     pos.Column,
								Message:    "Nested range loops may cause cache misses, consider optimizing data access patterns",
								Suggestion: "Restructure to iterate over data in cache-friendly order or use SoA pattern",
								Position:   pos,
							}
							issues = append(issues, issue)
						}
						return true
					},
				)
			}
			return true
		},
	)

	return issues
}

// analyzeRangeForCachePatterns analyzes range statements for cache patterns
func (c *CPUCacheAnalyzer) analyzeRangeForCachePatterns(rangeStmt *ast.RangeStmt, fset *token.FileSet) []*Issue {
	var issues []*Issue

	// Check for range over map in performance-critical sections
	if c.isMapType(rangeStmt.X) {
		pos := fset.Position(rangeStmt.Pos())
		issue := &Issue{
			Type:       IssueMapRangeCache,
			Severity:   SeverityLevelLow,
			Line:       pos.Line,
			Column:     pos.Column,
			Message:    "Range over map has unpredictable memory access pattern",
			Suggestion: "Consider using slice with sorted keys for predictable cache access pattern",
			Position:   pos,
		}
		issues = append(issues, issue)
	}

	return issues
}

// Helper methods

func (c *CPUCacheAnalyzer) getFieldType(expr ast.Expr) types.Type {
	// Simplified type extraction - reuse from struct_layout_analyzer
	switch t := expr.(type) {
	case *ast.Ident:
		return c.getBasicType(t.Name)
	case *ast.StarExpr:
		return types.NewPointer(c.getFieldType(t.X))
	case *ast.ArrayType:
		return types.NewSlice(c.getFieldType(t.Elt))
	}
	return types.Typ[types.UntypedNil]
}

func (c *CPUCacheAnalyzer) getBasicType(name string) types.Type {
	switch name {
	case "bool":
		return types.Typ[types.Bool]
	case "int8":
		return types.Typ[types.Int8]
	case "int16":
		return types.Typ[types.Int16]
	case "int32", "rune":
		return types.Typ[types.Int32]
	case "int64":
		return types.Typ[types.Int64]
	case "int":
		return types.Typ[types.Int]
	case "uint8", "byte":
		return types.Typ[types.Uint8]
	case "uint16":
		return types.Typ[types.Uint16]
	case "uint32":
		return types.Typ[types.Uint32]
	case "uint64":
		return types.Typ[types.Uint64]
	case "uint":
		return types.Typ[types.Uint]
	case "float32":
		return types.Typ[types.Float32]
	case "float64":
		return types.Typ[types.Float64]
	case "string":
		return types.Typ[types.String]
	default:
		return types.Typ[types.UntypedNil]
	}
}

func (c *CPUCacheAnalyzer) getTypeSize(t types.Type) int64 {
	switch typ := t.(type) {
	case *types.Basic:
		return c.getBasicTypeSize(typ)
	case *types.Pointer:
		return int64(unsafe.Sizeof(uintptr(0)))
	case *types.Array:
		return typ.Len() * c.getTypeSize(typ.Elem())
	case *types.Slice:
		return int64(unsafe.Sizeof([]int{}))
	default:
		return 8
	}
}

func (c *CPUCacheAnalyzer) getBasicTypeSize(t *types.Basic) int64 {
	switch t.Kind() {
	case types.Bool, types.Int8, types.Uint8:
		return 1
	case types.Int16, types.Uint16:
		return 2
	case types.Int32, types.Uint32, types.Float32:
		return 4
	case types.Int64, types.Uint64, types.Float64:
		return 8
	case types.Int, types.Uint:
		return int64(unsafe.Sizeof(int(0)))
	case types.String:
		return int64(unsafe.Sizeof(""))
	default:
		return 8
	}
}

func (c *CPUCacheAnalyzer) getTypeAlignment(t types.Type) int64 {
	size := c.getTypeSize(t)
	if size >= 8 {
		return 8
	}
	return size
}

func (c *CPUCacheAnalyzer) getFieldName(field *ast.Field) string {
	if len(field.Names) > 0 {
		return field.Names[0].Name
	}
	return "anonymous"
}

func (c *CPUCacheAnalyzer) calculateStructSize(structType *ast.StructType) int64 {
	var currentOffset int64
	var maxAlignment int64 = 1

	for _, field := range structType.Fields.List {
		fieldType := c.getFieldType(field.Type)
		size := c.getTypeSize(fieldType)
		alignment := c.getTypeAlignment(fieldType)

		if alignment > maxAlignment {
			maxAlignment = alignment
		}

		// Add padding for alignment
		padding := (alignment - currentOffset%alignment) % alignment
		currentOffset += padding + size
	}

	// Final padding to align struct to its own alignment
	finalPadding := (maxAlignment - currentOffset%maxAlignment) % maxAlignment
	return currentOffset + finalPadding
}

func (c *CPUCacheAnalyzer) isConcurrentField(fieldName string) bool {
	concurrentPatterns := []string{
		"mutex", "lock", "atomic", "counter", "flag", "state",
		"sync", "concurrent", "shared", "parallel", "worker",
	}

	fieldLower := strings.ToLower(fieldName)
	for _, pattern := range concurrentPatterns {
		if strings.Contains(fieldLower, pattern) {
			return true
		}
	}
	return false
}

func (c *CPUCacheAnalyzer) couldUseInt32(fieldName string) bool {
	// Heuristic: suggest int32 for common counter/index/ID fields
	smallValuePatterns := []string{
		"count", "index", "idx", "id", "num", "size", "len",
		"capacity", "limit", "max", "min", "pos", "offset",
	}

	fieldLower := strings.ToLower(fieldName)
	for _, pattern := range smallValuePatterns {
		if strings.Contains(fieldLower, pattern) {
			return true
		}
	}
	return false
}

func (c *CPUCacheAnalyzer) isSliceType(expr ast.Expr) bool {
	_, ok := expr.(*ast.ArrayType)
	return ok && strings.Contains(fmt.Sprintf("%T", expr), "ArrayType")
}

func (c *CPUCacheAnalyzer) isMapType(expr ast.Expr) bool {
	_, ok := expr.(*ast.MapType)
	return ok
}
