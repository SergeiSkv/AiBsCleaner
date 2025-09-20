package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// AIBullshitDetector находит РЕАЛЬНЫЕ паттерны AI-generated bullshit кода
type AIBullshitDetector struct{}

func NewAIBullshitDetector() *AIBullshitDetector {
	return &AIBullshitDetector{}
}

func (abd *AIBullshitDetector) Name() string {
	return "AIBullshitDetector"
}

func (abd *AIBullshitDetector) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				issues = append(issues, abd.checkOverEngineeredSimpleFunction(node, filename, fset)...)
				issues = append(issues, abd.checkUnnecessaryConcurrency(node, filename, fset)...)
				issues = append(issues, abd.checkOvercomplicatedLogic(node, filename, fset)...)

			case *ast.CommentGroup:
				issues = append(issues, abd.checkObviousComments(node, filename, fset)...)
			}
			return true
		},
	)

	return issues
}

// Проверка: использование горутин и каналов для простых операций
func (abd *AIBullshitDetector) checkUnnecessaryConcurrency(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	if fn.Body == nil {
		return issues
	}

	// Ищем паттерн: создание канала + горутина + ожидание результата в простой функции
	hasChannel := false
	hasGoroutine := false
	hasMath := false
	functionComplexity := len(fn.Body.List)

	ast.Inspect(
		fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.GoStmt:
				hasGoroutine = true
			case *ast.CallExpr:
				if ident, ok := node.Fun.(*ast.Ident); ok && ident.Name == "make" {
					if len(node.Args) > 0 {
						if _, ok := node.Args[0].(*ast.ChanType); ok {
							hasChannel = true
						}
					}
				}
			case *ast.BinaryExpr:
				// Проверяем, есть ли простые математические операции
				switch node.Op {
				case token.ADD, token.SUB, token.MUL, token.QUO:
					hasMath = true
				}
			}
			return true
		},
	)

	// Если функция простая (<10 строк), но использует горутины и каналы для математики
	if hasGoroutine && hasChannel && functionComplexity < 10 && hasMath {
		pos := fset.Position(fn.Pos())
		issues = append(
			issues, Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       "AI_BULLSHIT_CONCURRENCY",
				Severity:   SeverityHigh,
				Message:    "Using goroutines and channels for simple math - classic AI over-engineering",
				Suggestion: "Just return the result directly, no concurrency needed",
			},
		)
	}

	return issues
}

// Проверка: слишком сложная реализация простой функции
func (abd *AIBullshitDetector) checkOverEngineeredSimpleFunction(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	if fn.Body == nil || fn.Name == nil {
		return issues
	}

	funcName := strings.ToLower(fn.Name.Name)

	// Список простых функций, которые AI любит усложнять
	simplePatterns := map[string]int{
		"add":      3, // Сложение должно быть максимум 3 строки
		"subtract": 3,
		"multiply": 3,
		"divide":   5, // Деление может проверять на 0
		"iseven":   2,
		"isodd":    2,
		"max":      5,
		"min":      5,
		"abs":      5,
		"swap":     3,
	}

	for pattern, maxLines := range simplePatterns {
		if strings.Contains(funcName, pattern) {
			if len(fn.Body.List) > maxLines {
				// Проверяем, не использует ли reflection для простой операции
				hasReflection := false
				ast.Inspect(
					fn.Body, func(n ast.Node) bool {
						if call, ok := n.(*ast.CallExpr); ok {
							if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
								if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "reflect" {
									hasReflection = true
								}
							}
						}
						return true
					},
				)

				if hasReflection {
					pos := fset.Position(fn.Pos())
					issues = append(
						issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "AI_REFLECTION_OVERKILL",
							Severity:   SeverityHigh,
							Message:    "Using reflection for simple " + pattern + " operation",
							Suggestion: "Just use direct operators, not reflection",
						},
					)
				} else if len(fn.Body.List) > maxLines*3 {
					pos := fset.Position(fn.Pos())
					issues = append(
						issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "AI_OVERENGINEERED_SIMPLE",
							Severity:   SeverityMedium,
							Message:    "Function '" + funcName + "' is overcomplicated (>" + string(rune(len(fn.Body.List))) + " lines for simple operation)",
							Suggestion: "Simplify - this should be " + string(rune(maxLines)) + " lines max",
						},
					)
				}
			}
		}
	}

	return issues
}

// Проверка: использование паттернов проектирования для тривиальных задач
func (abd *AIBullshitDetector) checkOvercomplicatedLogic(fn *ast.FuncDecl, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	if fn.Body == nil || fn.Name == nil {
		return issues
	}

	funcName := fn.Name.Name

	// AI любит Factory, Strategy, Observer для всего подряд
	overEngineeredPatterns := []string{
		"Factory", "Strategy", "Observer", "Visitor",
		"Builder", "Singleton", "Adapter", "Bridge",
		"AbstractFactory", "Prototype", "Facade",
	}

	for _, pattern := range overEngineeredPatterns {
		if strings.Contains(funcName, pattern) {
			// Проверяем, действительно ли нужен паттерн
			// Если функция возвращает простой тип или делает простую операцию
			if fn.Type.Results != nil && len(fn.Type.Results.List) == 1 {
				// Проверяем сложность функции
				if len(fn.Body.List) < 10 {
					pos := fset.Position(fn.Pos())
					issues = append(
						issues, Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       "AI_PATTERN_ABUSE",
							Severity:   SeverityHigh,
							Message:    "Using " + pattern + " pattern for trivial task",
							Suggestion: "Remove pattern - use simple function instead",
						},
					)
				}
			}
		}
	}

	// Проверка на HelloWorld с паттернами (классика AI bullshit)
	if strings.Contains(strings.ToLower(funcName), "hello") {
		hasInterface := false
		ast.Inspect(
			fn.Body, func(n ast.Node) bool {
				if _, ok := n.(*ast.InterfaceType); ok {
					hasInterface = true
				}
				return true
			},
		)

		if hasInterface || strings.Contains(funcName, "Factory") || strings.Contains(funcName, "Strategy") {
			pos := fset.Position(fn.Pos())
			issues = append(
				issues, Issue{
					File:       filename,
					Line:       pos.Line,
					Column:     pos.Column,
					Position:   pos,
					Type:       "AI_ENTERPRISE_HELLO_WORLD",
					Severity:   SeverityHigh,
					Message:    "Enterprise patterns for Hello World - peak AI bullshit",
					Suggestion: "Just use fmt.Println(\"Hello, World!\")",
				},
			)
		}
	}

	return issues
}

// Проверка очевидных комментариев (AI любит комментировать всё)
func (abd *AIBullshitDetector) checkObviousComments(cg *ast.CommentGroup, filename string, fset *token.FileSet) []Issue {
	var issues []Issue

	for _, comment := range cg.List {
		text := strings.ToLower(comment.Text)

		// Очевидные комментарии, которые любит AI
		obviousPatterns := []struct {
			pattern string
			example string
		}{
			{"// increment", "i++"},
			{"// decrement", "i--"},
			{"// return", "return"},
			{"// add 1", "x + 1"},
			{"// check if", "if"},
			{"// loop through", "for"},
			{"// iterate", "range"},
			{"// initialize", "var"},
			{"// declare", ":="},
			{"// this function", "obvious from function name"},
			{"// this method", "obvious from method name"},
		}

		for _, p := range obviousPatterns {
			if strings.Contains(text, p.pattern) {
				pos := fset.Position(comment.Pos())
				issues = append(
					issues, Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       "AI_CAPTAIN_OBVIOUS",
						Severity:   SeverityLow,
						Message:    "Captain Obvious comment: '" + p.pattern + "'",
						Suggestion: "Delete obvious comment - code is self-documenting",
					},
				)
				break
			}
		}
	}

	return issues
}
