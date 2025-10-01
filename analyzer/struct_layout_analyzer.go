package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"
	"unsafe"
)

// StructLayoutAnalyzer detects struct field alignment issues and provides layout visualization
type StructLayoutAnalyzer struct {
	TypeInfo *types.Info
}

// FieldInfo represents information about a struct field
type FieldInfo struct {
	Name      string
	Type      types.Type
	Size      int64
	Alignment int64
	Offset    int64
	IsPointer bool
	IsPadding bool
	Comment   string
}

// StructLayout represents the complete layout of a struct
type StructLayout struct {
	Name        string
	Fields      []FieldInfo
	TotalSize   int64
	Alignment   int64
	WastedBytes int64
	OptimalSize int64
	Suggestions []string
}

// NewStructLayoutAnalyzer creates a new struct layout analyzer
func NewStructLayoutAnalyzer() Analyzer {
	return &StructLayoutAnalyzer{}
}

// Name returns the name of this analyzer
func (s *StructLayoutAnalyzer) Name() string {
	return "StructLayout"
}

// Analyze performs struct layout analysis on the AST
func (s *StructLayoutAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
	file, ok := node.(*ast.File)
	if !ok || file == nil {
		return nil
	}

	var issues []*Issue

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.TypeSpec:
			if structType, ok := node.Type.(*ast.StructType); ok {
				layoutIssues := s.analyzeStruct(node.Name.Name, structType, fset)
				issues = append(issues, layoutIssues...)
			}
		}
		return true
	})

	return issues
}

// analyzeStruct analyzes a single struct for layout issues
func (s *StructLayoutAnalyzer) analyzeStruct(structName string, structType *ast.StructType, fset *token.FileSet) []*Issue {
	var issues []*Issue

	if structType.Fields == nil || len(structType.Fields.List) == 0 {
		return issues
	}

	layout := s.calculateLayout(structName, structType)

	// Check for alignment issues
	if layout.WastedBytes > 0 {
		pos := fset.Position(structType.Pos())

		issue := &Issue{
			Type:       IssueStructLayoutUnoptimized,
			Severity:   SeverityLevelMedium,
			Line:       pos.Line,
			Column:     pos.Column,
			Message:    fmt.Sprintf("Struct '%s' has suboptimal field alignment, wasting %d bytes", structName, layout.WastedBytes),
			Suggestion: s.generateOptimizationSuggestion(layout),
			Position:   pos,
		}
		issues = append(issues, issue)
	}

	// Check for large padding between fields
	for i, field := range layout.Fields {
		if field.IsPadding && field.Size > 4 {
			pos := fset.Position(structType.Pos())
			var message string
			if i+1 < len(layout.Fields) {
				message = fmt.Sprintf("Large padding (%d bytes) detected before field '%s' in struct '%s'", field.Size, layout.Fields[i+1].Name, structName)
			} else {
				message = fmt.Sprintf("Large final padding (%d bytes) detected in struct '%s'", field.Size, structName)
			}
			issue := &Issue{
				Type:       IssueStructLargePadding,
				Severity:   SeverityLevelLow,
				Line:       pos.Line,
				Column:     pos.Column,
				Message:    message,
				Suggestion: "Consider reordering fields to minimize padding",
				Position:   pos,
			}
			issues = append(issues, issue)
		}
	}

	return issues
}

// calculateLayout computes the memory layout of a struct
func (s *StructLayoutAnalyzer) calculateLayout(structName string, structType *ast.StructType) StructLayout {
	layout := StructLayout{
		Name:   structName,
		Fields: make([]FieldInfo, 0),
	}

	var currentOffset int64
	var maxAlignment int64 = 1

	for _, field := range structType.Fields.List {
		fieldType := s.getFieldType(field.Type)
		size := s.getTypeSize(fieldType)
		alignment := s.getTypeAlignment(fieldType)

		if alignment > maxAlignment {
			maxAlignment = alignment
		}

		// Add padding if needed
		padding := s.calculatePadding(currentOffset, alignment)
		if padding > 0 {
			layout.Fields = append(layout.Fields, FieldInfo{
				Name:      fmt.Sprintf("_padding_%d", len(layout.Fields)),
				Size:      padding,
				Offset:    currentOffset,
				IsPadding: true,
			})
			currentOffset += padding
		}

		// Add the actual field
		fieldName := s.getFieldName(field)
		layout.Fields = append(layout.Fields, FieldInfo{
			Name:      fieldName,
			Type:      fieldType,
			Size:      size,
			Alignment: alignment,
			Offset:    currentOffset,
			IsPointer: s.isPointerType(fieldType),
		})

		currentOffset += size
	}

	// Final padding to align struct to its own alignment
	finalPadding := s.calculatePadding(currentOffset, maxAlignment)
	if finalPadding > 0 {
		layout.Fields = append(layout.Fields, FieldInfo{
			Name:      "_final_padding",
			Size:      finalPadding,
			Offset:    currentOffset,
			IsPadding: true,
		})
		currentOffset += finalPadding
	}

	layout.TotalSize = currentOffset
	layout.Alignment = maxAlignment
	layout.OptimalSize = s.calculateOptimalSize(structType)
	layout.WastedBytes = layout.TotalSize - layout.OptimalSize
	layout.Suggestions = s.generateSuggestions(layout)

	return layout
}

// getFieldType extracts type information from field
func (s *StructLayoutAnalyzer) getFieldType(expr ast.Expr) types.Type {
	// This is a simplified type extraction - in a real implementation,
	// you would use types.Info to get accurate type information
	switch t := expr.(type) {
	case *ast.Ident:
		return s.getBasicType(t.Name)
	case *ast.StarExpr:
		return types.NewPointer(s.getFieldType(t.X))
	case *ast.ArrayType:
		if t.Len != nil {
			if basic, ok := t.Len.(*ast.BasicLit); ok {
				if length, err := strconv.Atoi(basic.Value); err == nil {
					return types.NewArray(s.getFieldType(t.Elt), int64(length))
				}
			}
		}
		return types.NewSlice(s.getFieldType(t.Elt))
	case *ast.SelectorExpr:
		return s.getBasicType("interface{}")
	}
	return s.getBasicType("interface{}")
}

// getBasicType returns a basic type based on name
func (s *StructLayoutAnalyzer) getBasicType(name string) types.Type {
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

// getTypeSize returns the size of a type in bytes
func (s *StructLayoutAnalyzer) getTypeSize(t types.Type) int64 {
	switch typ := t.(type) {
	case *types.Basic:
		return s.getBasicTypeSize(typ)
	case *types.Pointer:
		return int64(unsafe.Sizeof(uintptr(0)))
	case *types.Array:
		return typ.Len() * s.getTypeSize(typ.Elem())
	case *types.Slice:
		return int64(unsafe.Sizeof([]int{}))
	case *types.Map:
		return int64(unsafe.Sizeof(map[int]int{}))
	case *types.Chan:
		return int64(unsafe.Sizeof(make(chan int)))
	case *types.Interface:
		return int64(unsafe.Sizeof((*interface{})(nil)))
	default:
		return 8 // Default size for unknown types
	}
}

// getBasicTypeSize returns the size of basic types
func (s *StructLayoutAnalyzer) getBasicTypeSize(t *types.Basic) int64 {
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
	case types.Uintptr:
		return int64(unsafe.Sizeof(uintptr(0)))
	case types.String:
		return int64(unsafe.Sizeof(""))
	default:
		return 8
	}
}

// getTypeAlignment returns the alignment requirement of a type
func (s *StructLayoutAnalyzer) getTypeAlignment(t types.Type) int64 {
	switch typ := t.(type) {
	case *types.Basic:
		return s.getBasicTypeAlignment(typ)
	case *types.Pointer:
		return int64(unsafe.Alignof(uintptr(0)))
	case *types.Array:
		return s.getTypeAlignment(typ.Elem())
	case *types.Slice:
		return int64(unsafe.Alignof([]int{}))
	case *types.Map:
		return int64(unsafe.Alignof(map[int]int{}))
	case *types.Chan:
		return int64(unsafe.Alignof(make(chan int)))
	case *types.Interface:
		return int64(unsafe.Alignof((*interface{})(nil)))
	default:
		return 8
	}
}

// getBasicTypeAlignment returns alignment for basic types
func (s *StructLayoutAnalyzer) getBasicTypeAlignment(t *types.Basic) int64 {
	size := s.getBasicTypeSize(t)
	if size >= 8 {
		return 8
	}
	return size
}

// calculatePadding calculates padding needed for alignment
func (s *StructLayoutAnalyzer) calculatePadding(offset, alignment int64) int64 {
	remainder := offset % alignment
	if remainder == 0 {
		return 0
	}
	return alignment - remainder
}

// getFieldName extracts field name from ast.Field
func (s *StructLayoutAnalyzer) getFieldName(field *ast.Field) string {
	if len(field.Names) > 0 {
		return field.Names[0].Name
	}
	// Anonymous field
	if ident, ok := field.Type.(*ast.Ident); ok {
		return ident.Name
	}
	return "anonymous"
}

// isPointerType checks if type is a pointer
func (s *StructLayoutAnalyzer) isPointerType(t types.Type) bool {
	_, ok := t.(*types.Pointer)
	return ok
}

// calculateOptimalSize calculates the size if fields were optimally ordered
func (s *StructLayoutAnalyzer) calculateOptimalSize(structType *ast.StructType) int64 {
	// Collect all fields with their sizes and alignments
	type fieldData struct {
		size      int64
		alignment int64
	}

	var fields []fieldData
	var maxAlignment int64 = 1

	for _, field := range structType.Fields.List {
		fieldType := s.getFieldType(field.Type)
		size := s.getTypeSize(fieldType)
		alignment := s.getTypeAlignment(fieldType)

		fields = append(fields, fieldData{size: size, alignment: alignment})
		if alignment > maxAlignment {
			maxAlignment = alignment
		}
	}

	// Sort fields by alignment (descending) then by size (descending) for optimal packing
	sort.Slice(fields, func(i, j int) bool {
		if fields[i].alignment != fields[j].alignment {
			return fields[i].alignment > fields[j].alignment
		}
		return fields[i].size > fields[j].size
	})

	// Calculate optimal layout
	var offset int64
	for _, field := range fields {
		padding := s.calculatePadding(offset, field.alignment)
		offset += padding + field.size
	}

	// Final alignment
	finalPadding := s.calculatePadding(offset, maxAlignment)
	return offset + finalPadding
}

// generateSuggestions creates optimization suggestions
func (s *StructLayoutAnalyzer) generateSuggestions(layout StructLayout) []string {
	var suggestions []string

	if layout.WastedBytes > 0 {
		suggestions = append(suggestions, fmt.Sprintf("Reorder fields to reduce struct size from %d to %d bytes (save %d bytes)",
			layout.TotalSize, layout.OptimalSize, layout.WastedBytes))
	}

	// Suggest field ordering
	nonPaddingFields := make([]FieldInfo, 0)
	for _, field := range layout.Fields {
		if !field.IsPadding {
			nonPaddingFields = append(nonPaddingFields, field)
		}
	}

	if len(nonPaddingFields) > 1 {
		sort.Slice(nonPaddingFields, func(i, j int) bool {
			if nonPaddingFields[i].Alignment != nonPaddingFields[j].Alignment {
				return nonPaddingFields[i].Alignment > nonPaddingFields[j].Alignment
			}
			return nonPaddingFields[i].Size > nonPaddingFields[j].Size
		})

		var orderedNames []string
		for _, field := range nonPaddingFields {
			orderedNames = append(orderedNames, field.Name)
		}
		suggestions = append(suggestions, fmt.Sprintf("Optimal field order: %s", strings.Join(orderedNames, ", ")))
	}

	return suggestions
}

// generateOptimizationSuggestion creates a detailed optimization suggestion
func (s *StructLayoutAnalyzer) generateOptimizationSuggestion(layout StructLayout) string {
	if len(layout.Suggestions) > 0 {
		return layout.Suggestions[0]
	}
	return "Consider using 'go run golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment -fix .' to optimize struct layout"
}

// VisualizeLayout creates a text visualization of struct layout
func (s *StructLayoutAnalyzer) VisualizeLayout(layout StructLayout) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Struct %s Layout (Total: %d bytes, Optimal: %d bytes, Wasted: %d bytes)\n",
		layout.Name, layout.TotalSize, layout.OptimalSize, layout.WastedBytes))
	builder.WriteString("╭" + strings.Repeat("─", 60) + "╮\n")

	for _, field := range layout.Fields {
		if field.IsPadding {
			builder.WriteString(fmt.Sprintf("│ %-20s [PADDING] %2d bytes at offset %2d │\n",
				field.Name, field.Size, field.Offset))
		} else {
			typeStr := "unknown"
			if field.Type != nil {
				typeStr = field.Type.String()
			}
			builder.WriteString(fmt.Sprintf("│ %-20s %-15s %2d bytes at offset %2d │\n",
				field.Name, typeStr, field.Size, field.Offset))
		}
	}

	builder.WriteString("╰" + strings.Repeat("─", 60) + "╯\n")

	if len(layout.Suggestions) > 0 {
		builder.WriteString("\nOptimization Suggestions:\n")
		for i, suggestion := range layout.Suggestions {
			builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, suggestion))
		}
	}

	return builder.String()
}
