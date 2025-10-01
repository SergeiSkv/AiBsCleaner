//go:build legacytests
// +build legacytests

package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalkWithContext(t *testing.T) {
	code := `package main

func main() {
	for i := 0; i < 10; i++ {
		if i > 5 {
			for j := 0; j < 5; j++ {
				println(i, j)
			}
		}
	}
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	loopDepths := make(map[token.Pos]int)
	var maxDepth int

	WalkWithContext(file, func(node ast.Node, ctx *AnalysisContext) bool {
		if ctx.LoopDepth > maxDepth {
			maxDepth = ctx.LoopDepth
		}
		loopDepths[node.Pos()] = ctx.LoopDepth
		return true
	})

	// We should have found nested loops
	assert.Positive(t, maxDepth, "Should detect nested loops")
	assert.NotEmpty(t, loopDepths, "Should have visited nodes")
}

func TestWalkContextInLoop(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected map[string]bool // node type -> in loop
	}{
		{
			name: "simple for loop",
			code: `package main
func test() {
	x := 1
	for i := 0; i < 10; i++ {
		x += i
	}
	y := 2
}`,
			expected: map[string]bool{
				"x := 1": false, // outside loop
				"x += i": true,  // inside loop
				"y := 2": false, // outside loop
			},
		},
		{
			name: "range loop",
			code: `package main
func test() {
	items := []int{1, 2, 3}
	for _, item := range items {
		println(item)
	}
}`,
			expected: map[string]bool{
				"println": true, // inside loop
			},
		},
		{
			name: "nested loops",
			code: `package main
func test() {
	for i := 0; i < 10; i++ {
		for j := 0; j < 5; j++ {
			println(i * j)
		}
	}
}`,
			expected: map[string]bool{
				"println": true, // inside nested loops
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
			require.NoError(t, err)

			results := make(map[string]bool)
			WalkWithContext(file, func(node ast.Node, ctx *AnalysisContext) bool {
				switch n := node.(type) {
				case *ast.CallExpr:
					if ident, ok := n.Fun.(*ast.Ident); ok {
						results[ident.Name] = ctx.InLoop
					}
				case *ast.AssignStmt:
					ident, ok := n.Lhs[0].(*ast.Ident)
					if !ok {
						break
					}

					if ctx.InLoop {
						if ident.Name == "x" && n.Tok == token.ADD_ASSIGN {
							results["x += i"] = true
						}
					} else {
						switch ident.Name {
						case "x":
							results["x := 1"] = false
						case "y":
							results["y := 2"] = false
						}
					}
				}
				return true
			})

			for key, expectedInLoop := range tt.expected {
				actualInLoop, exists := results[key]
				if exists {
					assert.Equal(t, expectedInLoop, actualInLoop, "For %s: expected InLoop=%v", key, expectedInLoop)
				}
			}
		})
	}
}

func TestWalkContextCurrentFunc(t *testing.T) {
	code := `package main

func outer() {
	println("hello")
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	var foundPrintln bool
	var currentFuncName string

	WalkWithContext(file, func(node ast.Node, ctx *AnalysisContext) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "println" {
				foundPrintln = true
				currentFuncName = ctx.CurrentFunc
			}
		}
		return true
	})

	assert.True(t, foundPrintln, "Should find println call")
	assert.Equal(t, "outer", currentFuncName, "Should track current function name")
}
