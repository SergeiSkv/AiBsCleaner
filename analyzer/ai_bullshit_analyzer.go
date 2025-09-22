package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// AIBullshitAnalyzer detects AI-generated bullshit code patterns
type AIBullshitAnalyzer struct {
	name string
}

// NewAIBullshitAnalyzer creates a new AI bullshit detector
func NewAIBullshitAnalyzer() Analyzer {
	return &AIBullshitAnalyzer{
		name: "AI Bullshit Detector",
	}
}

func (a *AIBullshitAnalyzer) Name() string {
	return a.name
}

func (a *AIBullshitAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				issues = append(issues, a.checkOverEngineering(node, fset)...)
				issues = append(issues, a.checkUnnecessaryComplexity(node, fset)...)
				issues = append(issues, a.checkAIPatterns(node, fset)...)
			case *ast.CallExpr:
				issues = append(issues, a.checkUnnecessaryReflection(node, fset)...)
				issues = append(issues, a.checkOverAbstraction(node, fset)...)
			case *ast.GenDecl:
				issues = append(issues, a.checkUnnecessaryInterfaces(node, fset)...)
			}
			return true
		},
	)

	return issues
}

// AI often creates over-engineered solutions for simple tasks
func (a *AIBullshitAnalyzer) checkOverEngineering(fn *ast.FuncDecl, fset *token.FileSet) []Issue {
	var issues []Issue
	pos := fset.Position(fn.Pos())

	if fn.Name == nil || fn.Body == nil {
		return issues
	}

	funcName := fn.Name.Name

	// Check for design patterns that might be overused for simple tasks
	designPatterns := []string{
		"Factory", "Strategy", "Observer", "Visitor", "AbstractFactory",
		"Builder", "Singleton", "Adapter", "Bridge", "Composite",
		"Decorator", "Facade", "Flyweight", "Proxy", "Command",
	}

	for _, pattern := range designPatterns {
		if strings.Contains(funcName, pattern) {
			// If function is simple (few lines) but uses complex patterns
			if len(fn.Body.List) < 5 {
				issues = append(
					issues, Issue{
						File:       pos.Filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "AI_OVER_ENGINEERING",
						Severity:   SeverityHigh,
						Message:    "Over-engineered solution for simple task - AI bullshit detected",
						Suggestion: "Simplify: this function is too simple to need " + pattern + " pattern",
						Code:       "Function: " + funcName,
					},
				)
			}
		}
	}

	return issues
}

// Check unnecessary complexity (AI loves to overcomplicate)
func (a *AIBullshitAnalyzer) checkUnnecessaryComplexity(fn *ast.FuncDecl, fset *token.FileSet) []Issue {
	var issues []Issue
	pos := fset.Position(fn.Pos())

	if fn.Name == nil || fn.Body == nil {
		return issues
	}

	// AI often creates functions with excessive complexity for simple tasks
	// Example: 30 lines of code to check if number is even
	if strings.Contains(fn.Name.Name, "Check") || strings.Contains(fn.Name.Name, "Validate") {
		if len(fn.Body.List) > 20 {
			// Check if it's too complex for simple validation
			hasSimpleLogic := false
			ast.Inspect(
				fn.Body, func(n ast.Node) bool {
					// Look for simple operations (% == != < > && ||)
					if binExpr, ok := n.(*ast.BinaryExpr); ok {
						op := binExpr.Op.String()
						if op == "%" || op == "==" || op == "!=" || op == "<" || op == ">" {
							hasSimpleLogic = true
						}
					}
					return true
				},
			)

			if hasSimpleLogic {
				issues = append(
					issues, Issue{
						File:       pos.Filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "AI_UNNECESSARY_COMPLEXITY",
						Severity:   SeverityHigh,
						Message:    "Unnecessarily complex function for simple logic - AI bullshit",
						Suggestion: "This can probably be done in 1-3 lines, not " + string(rune(len(fn.Body.List))) + " lines",
						Code:       "Function: " + fn.Name.Name,
					},
				)
			}
		}
	}

	return issues
}

// AI-specific patterns
func (a *AIBullshitAnalyzer) checkAIPatterns(fn *ast.FuncDecl, fset *token.FileSet) []Issue {
	var issues []Issue
	pos := fset.Position(fn.Pos())

	if fn.Name == nil || fn.Body == nil {
		return issues
	}

	// AI often creates goroutine + channel for simple operations
	hasGoroutine := false
	hasChannel := false

	ast.Inspect(
		fn.Body, func(n ast.Node) bool {
			// Look for goroutines
			if goStmt, ok := n.(*ast.GoStmt); ok {
				_ = goStmt
				hasGoroutine = true
			}

			// Look for channels
			if callExpr, ok := n.(*ast.CallExpr); ok {
				if ident, ok := callExpr.Fun.(*ast.Ident); ok {
					if ident.Name == "make" && len(callExpr.Args) > 0 {
						if chanType, ok := callExpr.Args[0].(*ast.ChanType); ok {
							_ = chanType
							hasChannel = true
						}
					}
				}
			}
			return true
		},
	)

	// AI bullshit: goroutine + channel for adding two numbers
	if hasGoroutine && hasChannel && len(fn.Body.List) < 10 {
		issues = append(
			issues, Issue{
				File:       pos.Filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       "AI_GOROUTINE_OVERKILL",
				Severity:   SeverityHigh,
				Message:    "Using goroutines and channels for simple operation - AI bullshit",
				Suggestion: "Remove goroutines and channels, do it synchronously",
				Code:       "Function: " + fn.Name.Name,
			},
		)
	}

	return issues
}

// AI loves unnecessary reflection
func (a *AIBullshitAnalyzer) checkUnnecessaryReflection(call *ast.CallExpr, fset *token.FileSet) []Issue {
	var issues []Issue
	pos := fset.Position(call.Pos())

	funcName := getFuncNameFromCall(call)

	// If reflection is used for simple operations
	if strings.Contains(funcName, "reflect.") {
		reflectMethods := []string{"ValueOf", "TypeOf", "DeepEqual", "Select", "Call"}
		for _, method := range reflectMethods {
			if strings.Contains(funcName, method) {
				issues = append(
					issues, Issue{
						File:       pos.Filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "AI_UNNECESSARY_REFLECTION",
						Severity:   SeverityHigh,
						Message:    "Using reflection for simple operation - AI overkill",
						Suggestion: "Use direct type operations instead of reflection",
						Code:       "Call: " + funcName,
					},
				)
			}
		}
	}

	return issues
}

// AI creates unnecessary abstraction
func (a *AIBullshitAnalyzer) checkOverAbstraction(call *ast.CallExpr, fset *token.FileSet) []Issue {
	var issues []Issue
	pos := fset.Position(call.Pos())

	funcName := getFuncNameFromCall(call)

	// AI bullshit: creating interfaces for everything
	abstractionWords := []string{"Interface", "Abstract", "Factory", "Manager", "Handler", "Service", "Provider"}

	for _, word := range abstractionWords {
		if strings.Contains(funcName, word) {
			issues = append(
				issues, Issue{
					File:       pos.Filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "AI_OVER_ABSTRACTION",
					Severity:   SeverityMedium,
					Message:    "Potentially over-abstracted code - AI pattern",
					Suggestion: "Consider if this abstraction is really needed",
					Code:       "Call: " + funcName,
				},
			)
		}
	}

	return issues
}

// AI creates interfaces for everything
func (a *AIBullshitAnalyzer) checkUnnecessaryInterfaces(gen *ast.GenDecl, fset *token.FileSet) []Issue {
	var issues []Issue
	pos := fset.Position(gen.Pos())

	if gen.Tok != token.TYPE {
		return issues
	}

	for _, spec := range gen.Specs {
		if typeSpec, ok := spec.(*ast.TypeSpec); ok {
			interfaceName := typeSpec.Name.Name

			// Check for Factory and other over-engineering patterns
			overEngineeredPatterns := []string{
				"Factory", "AbstractFactory", "Builder", "Strategy",
			}

			for _, pattern := range overEngineeredPatterns {
				if strings.Contains(interfaceName, pattern) {
					issues = append(
						issues, Issue{
							File:       pos.Filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "AI_OVER_ENGINEERING",
							Severity:   SeverityHigh,
							Message:    "Over-engineered solution for simple task - AI bullshit detected",
							Suggestion: "Simplify: consider if " + pattern + " pattern is really necessary here",
							Code:       "Type: " + interfaceName,
						},
					)
				}
			}

			if interfaceType, ok := typeSpec.Type.(*ast.InterfaceType); ok {
				// If interface has only one method and simple name
				if len(interfaceType.Methods.List) == 1 {
					// AI bullshit patterns in interface names
					bullshitPatterns := []string{"Provider", "Manager", "Handler", "Service"}

					for _, pattern := range bullshitPatterns {
						if strings.Contains(interfaceName, pattern) {
							issues = append(
								issues, Issue{
									File:       pos.Filename,
									Line:       pos.Line,
									Column:     pos.Column,
									Position:   pos,
									Type:       "AI_UNNECESSARY_INTERFACE",
									Severity:   SeverityMedium,
									Message:    "Single-method interface with generic name - possible AI bullshit",
									Suggestion: "Consider if this interface is really needed or use concrete type",
									Code:       "Interface: " + interfaceName,
								},
							)
						}
					}
				}
			}
		}
	}

	return issues
}

func getFuncNameFromCall(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		if pkg, ok := fun.X.(*ast.Ident); ok {
			return pkg.Name + "." + fun.Sel.Name
		}
		return fun.Sel.Name
	}
	return ""
}
