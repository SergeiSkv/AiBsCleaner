package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// SerializationAnalyzer detects inefficient JSON/XML/encoding operations
type SerializationAnalyzer struct{}

func NewSerializationAnalyzer() Analyzer {
	return &SerializationAnalyzer{}
}

func (sa *SerializationAnalyzer) Name() string {
	return "SerializationAnalyzer"
}

func (sa *SerializationAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
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

	encodersInFunc := make(map[string]bool)

	// Use AnalyzerWithContext for proper loop detection
	ctx := NewAnalyzerWithContext(astNode)

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			if n == nil {
				return true
			}

			inLoop := ctx.IsNodeInLoop(n)
			loopDepth := ctx.GetNodeLoopDepth(n)

			switch node := n.(type) {
			case *ast.CallExpr:
				pos := fset.Position(node.Pos())

				// Check for Marshal/Unmarshal in loops
				if sa.isSerializationCall(node) {
					issues = append(issues, sa.checkSerializationInLoop(node, pos, filename, inLoop, loopDepth)...)
				}

				// Check for encoder/decoder creation
				if sa.isEncoderCreation(node) {
					if inLoop {
						issues = append(
							issues, &Issue{
								File:       filename,
								Line:       pos.Line,
								Column:     pos.Column,
								Position:   pos,
								Type:       IssueSerializationInLoop,
								Severity:   SeverityLevelHigh,
								Message:    "Creating encoder/decoder in loop wastes resources",
								Suggestion: "Create encoder/decoder once and reuse",
								WhyBad: `Creating encoders in loops:
• Allocates buffers repeatedly
• Loses internal caching benefits
• Prevents connection reuse (for streaming)
IMPACT: Memory allocations + GC pressure`,
							},
						)
					}

					// Track encoder creation
					if funcName := sa.getCurrentFunction(n); funcName != "" {
						encodersInFunc[funcName] = true
					}
				}

				// Check for inefficient patterns
				if sa.isMarshalToString(node) {
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueSerializationInLoop,
							Severity:   SeverityLevelLow,
							Message:    "Converting marshaled []byte to string causes extra allocation",
							Suggestion: "Keep as []byte or write directly to io.Writer",
						},
					)
				}

				// Check for double marshaling
				if sa.isDoubleMarshal(node) {
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueSerializationInLoop,
							Severity:   SeverityLevelMedium,
							Message:    "Double marshaling detected (e.g., JSON of JSON)",
							Suggestion: "Use single marshaling pass with proper struct tags",
						},
					)
				}

				// Check for map[string]interface{} usage
				if sa.usesInterfaceMap(node) {
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueSerializationInLoop,
							Severity:   SeverityLevelMedium,
							Message:    "Marshaling map[string]interface{} is slower than structs",
							Suggestion: "Use strongly-typed structs for better performance",
							WhyBad: `map[string]interface{} marshaling is slow:
• Type assertions for every value
• No compile-time optimization
• Increased reflection overhead
IMPACT: 2-5x slower than struct marshaling`,
						},
					)
				}

				// Check for Pretty/Indent in production
				if sa.isPrettyPrint(node) {
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueSerializationInLoop,
							Severity:   SeverityLevelLow,
							Message:    "Pretty printing adds ~30% overhead",
							Suggestion: "Use pretty printing only for debugging, not production",
						},
					)
				}

				// Check base64 encoding in loops
				if sa.isBase64InLoop(node) && inLoop {
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueSerializationInLoop,
							Severity:   SeverityLevelMedium,
							Message:    "Base64 encoding in loop - consider streaming encoder",
							Suggestion: "Use base64.NewEncoder for streaming operations",
						},
					)
				}

			case *ast.AssignStmt:
				// Check for JSON struct tag issues
				if sa.hasMissingJSONTags(node) {
					pos := fset.Position(node.Pos())
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueSerializationInLoop,
							Severity:   SeverityLevelLow,
							Message:    "Struct fields without json tags use slow field name conversion",
							Suggestion: "Add explicit json tags to avoid runtime field name computation",
						},
					)
				}
			}

			return true
		},
	)

	return issues
}

func (sa *SerializationAnalyzer) checkSerializationInLoop(node *ast.CallExpr, pos token.Position, filename string, inLoop bool, loopDepth int) []*Issue {
	var issues []*Issue

	// Don't flag encoder.Encode() calls - those are meant to be reused
	if inLoop && !sa.isEncoderMethod(node) {
		severity := SeverityLevelMedium
		if loopDepth > 1 {
			severity = SeverityLevelHigh
		}

		opType := sa.getSerializationType(node)
		issues = append(
			issues, &Issue{
				File:     filename,
				Line:     pos.Line,
				Column:   pos.Column,
				Position: pos,
				Type:     IssueSerializationInLoop,
				Severity: severity,
				Message:  fmt.Sprintf("%s operation inside loop causes repeated reflection overhead", opType),
				Suggestion: fmt.Sprintf(
					"Use %s.Encoder/Decoder for streaming or batch operations", strings.Split(opType, ".")[0],
				),
				WhyBad: fmt.Sprintf(
					`%s in loops is inefficient:
• Reflection overhead on every iteration (~1-10μs per call)
• Allocates new buffers each time
• Cannot reuse type information cache
• In nested loops: overhead multiplied
IMPACT: 10-100x slower than streaming encoder
BETTER: Create encoder/decoder outside loop and reuse`, opType,
				),
			},
		)
	}

	// Check for repeated struct marshaling
	if sa.isSameStructMarshaled(node) {
		issues = append(
			issues, &Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       IssueSerializationInLoop,
				Severity:   SeverityLevelMedium,
				Message:    "Same struct type marshaled multiple times",
				Suggestion: "Consider caching marshaled results or using code generation (easyjson, ffjson)",
			},
		)
	}

	return issues
}

func (sa *SerializationAnalyzer) isSerializationCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	funcName := sel.Sel.Name
	marshalFuncs := []string{
		"Marshal", "MarshalJSON", "MarshalXML", "MarshalText",
		"Unmarshal", "UnmarshalJSON", "UnmarshalXML", "UnmarshalText",
		"Encode", "Decode",
	}

	for _, mf := range marshalFuncs {
		if funcName != mf {
			continue
		}

		// Check if it's from encoding packages
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}

		packages := []string{"json", "xml", "gob", "yaml", "toml", "msgpack"}
		for _, pkg := range packages {
			if strings.Contains(ident.Name, pkg) {
				return true
			}
		}
		return true
	}
	return false
}

func (sa *SerializationAnalyzer) getSerializationType(call *ast.CallExpr) string {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok {
			return ident.Name + "." + sel.Sel.Name
		}
	}
	return "serialization"
}

func (sa *SerializationAnalyzer) isEncoderCreation(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	funcName := sel.Sel.Name
	return funcName == "NewEncoder" || funcName == "NewDecoder"
}

func (sa *SerializationAnalyzer) isMarshalToString(node *ast.CallExpr) bool {
	// Check for string(json.Marshal(...)) pattern
	ident, ok := node.Fun.(*ast.Ident)
	if !ok || ident.Name != "string" || len(node.Args) == 0 {
		return false
	}

	// Check if argument is a marshal call or identifier that could be marshaled data
	if call, ok := node.Args[0].(*ast.CallExpr); ok {
		return sa.isSerializationCall(call)
	}
	// Also check for string(variable) where variable might be marshaled data
	// This is simplified - ideally would track data flow
	if ident, ok := node.Args[0].(*ast.Ident); ok {
		// Simple heuristic: variable names ending with 'b' or containing 'byte'
		return strings.HasSuffix(ident.Name, "b") || strings.Contains(ident.Name, "byte")
	}
	return false
}

func (sa *SerializationAnalyzer) isDoubleMarshal(node *ast.CallExpr) bool {
	// Check if marshaling already marshaled data
	if !sa.isSerializationCall(node) {
		return false
	}

	for _, arg := range node.Args {
		call, ok := arg.(*ast.CallExpr)
		if !ok {
			continue
		}
		if sa.isSerializationCall(call) {
			return true
		}
	}
	return false
}

func (sa *SerializationAnalyzer) usesInterfaceMap(node *ast.CallExpr) bool {
	// Check for map[string]interface{} in marshal calls
	if !sa.isSerializationCall(node) {
		return false
	}

	for _, arg := range node.Args {
		// Look for composite literals that might be maps
		comp, ok := arg.(*ast.CompositeLit)
		if ok {
			// Check if it's a map type
			mapType, ok := comp.Type.(*ast.MapType)
			if ok {
				// Simple heuristic for interface{} type
				ident, ok := mapType.Value.(*ast.InterfaceType)
				if ok {
					// Empty interface (Methods can be nil for empty interface{})
					return ident.Methods == nil || len(ident.Methods.List) == 0
				}
			}
		}
		// Also check for identifiers that might be interface maps
		ident, ok := arg.(*ast.Ident)
		if ok {
			if strings.Contains(strings.ToLower(ident.Name), "data") ||
				strings.Contains(strings.ToLower(ident.Name), "map") ||
				strings.Contains(strings.ToLower(ident.Name), "interface") {
				return true
			}
		}
	}
	return false
}

func (sa *SerializationAnalyzer) isPrettyPrint(node *ast.CallExpr) bool {
	sel, ok := node.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	funcName := sel.Sel.Name
	return funcName == "MarshalIndent" || funcName == "Indent" || funcName == "PrettyPrint"
}

func (sa *SerializationAnalyzer) isBase64InLoop(node *ast.CallExpr) bool {
	sel, ok := node.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check for base64.StdEncoding.EncodeToString pattern
	if sel2, ok := sel.X.(*ast.SelectorExpr); ok {
		ident, ok := sel2.X.(*ast.Ident)
		if ok && ident.Name == pkgBase64 && sel2.Sel.Name == "StdEncoding" {
			funcName := sel.Sel.Name
			return funcName == methodEncodeToString || funcName == methodDecodeString
		}
	}
	// Also check for direct base64.EncodeToString calls
	if ident, ok := sel.X.(*ast.Ident); ok {
		if ident.Name == pkgBase64 {
			funcName := sel.Sel.Name
			return funcName == methodEncodeToString || funcName == methodDecodeString
		}
	}
	return false
}

func (sa *SerializationAnalyzer) isSameStructMarshaled(node *ast.CallExpr) bool {
	// Would need data flow analysis to detect properly
	// Simplified version
	return false
}

func (sa *SerializationAnalyzer) hasMissingJSONTags(node *ast.AssignStmt) bool {
	// Would need to analyze struct definitions
	// Simplified version
	return false
}

func (sa *SerializationAnalyzer) getCurrentFunction(node ast.Node) string {
	// Simplified - would need proper AST traversal
	return ""
}

// Check if this is an encoder/decoder method call (not static marshal)
func (sa *SerializationAnalyzer) isEncoderMethod(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		funcName := sel.Sel.Name
		// Check if it's a method call on an encoder/decoder instance
		if funcName == "Encode" || funcName == "Decode" {
			// If the selector is an identifier (like enc.Encode), it's a method call
			if _, ok := sel.X.(*ast.Ident); ok {
				return true
			}
		}
	}
	return false
}
