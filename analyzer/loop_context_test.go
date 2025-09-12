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

func TestIsInLoop(t *testing.T) {
	testCases := []struct {
		name     string
		code     string
		expected map[string]bool // maps function name to whether it's in loop
	}{
		{
			name: "simple for loop",
			code: `package test
func main() {
	for i := 0; i < 10; i++ {
		println(i) // in loop
	}
	println("done") // not in loop
}`,
			expected: map[string]bool{
				"println_1": true,  // first println is in loop
				"println_2": false, // second println is not
			},
		},
		{
			name: "nested loops",
			code: `package test
func main() {
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			println(i, j) // in nested loop
		}
	}
}`,
			expected: map[string]bool{
				"println": true,
			},
		},
		{
			name: "range loop",
			code: `package test
func main() {
	slice := []int{1, 2, 3}
	for _, v := range slice {
		println(v) // in loop
	}
}`,
			expected: map[string]bool{
				"println": true,
			},
		},
		{
			name: "no loops",
			code: `package test
func main() {
	println("hello")
	println("world")
}`,
			expected: map[string]bool{
				"println_1": false,
				"println_2": false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, "test.go", tc.code, parser.ParseComments)
			require.NoError(t, err)

			var printlnCalls []*ast.CallExpr
			ast.Inspect(node, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == funcPrintln {
						printlnCalls = append(printlnCalls, call)
					}
				}
				return true
			})

			// Test each println call
			for i, call := range printlnCalls {
				inLoop := IsInLoop(node, call)

				// For simplicity, we just check if the results make sense
				switch tc.name {
				case "simple for loop":
					if i == 0 {
						assert.True(t, inLoop, "First println should be in loop")
					} else {
						assert.False(t, inLoop, "Second println should not be in loop")
					}
				case "no loops":
					assert.False(t, inLoop, "println should not be in loop when there are no loops")
				case "nested loops", "range loop":
					assert.True(t, inLoop, "println should be in loop")
				}
			}
		})
	}
}

func TestGetLoopDepth(t *testing.T) {
	testCases := []struct {
		name           string
		code           string
		expectedDepths []int // expected depth for each println call
	}{
		{
			name: "no loops",
			code: `package test
func main() {
	println("depth 0")
}`,
			expectedDepths: []int{0},
		},
		{
			name: "single loop",
			code: `package test
func main() {
	for i := 0; i < 10; i++ {
		println("depth 1")
	}
}`,
			expectedDepths: []int{1},
		},
		{
			name: "nested loops",
			code: `package test
func main() {
	for i := 0; i < 10; i++ {
		println("depth 1")
		for j := 0; j < 10; j++ {
			println("depth 2")
			for k := 0; k < 10; k++ {
				println("depth 3")
			}
		}
	}
}`,
			expectedDepths: []int{1, 2, 3},
		},
		{
			name: "mixed depths",
			code: `package test
func main() {
	println("depth 0")
	for i := 0; i < 10; i++ {
		println("depth 1")
	}
	println("depth 0 again")
}`,
			expectedDepths: []int{0, 1, 0},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, "test.go", tc.code, parser.ParseComments)
			require.NoError(t, err)

			var printlnCalls []*ast.CallExpr
			ast.Inspect(node, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == funcPrintln {
						printlnCalls = append(printlnCalls, call)
					}
				}
				return true
			})

			require.Len(t, printlnCalls, len(tc.expectedDepths), "Number of println calls should match expected")

			for i, call := range printlnCalls {
				depth := GetLoopDepth(node, call)
				assert.Equal(t, tc.expectedDepths[i], depth, "Loop depth for println %d should be %d", i, tc.expectedDepths[i])
			}
		})
	}
}

func TestLoopAnalyzerWithContext(t *testing.T) {
	code := `package test

func main() {
	println("outside")
	
	for i := 0; i < 10; i++ {
		println("level 1")
		
		for j := 0; j < 10; j++ {
			println("level 2")
			
			for k := 0; k < 10; k++ {
				println("level 3")
			}
		}
	}
	
	for range []int{1, 2, 3} {
		println("range loop")
	}
}`

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	require.NoError(t, err)

	ctx := NewAnalyzerWithContext(node)
	assert.NotNil(t, ctx)

	var printlnCalls []*ast.CallExpr
	var printlnMessages []string

	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		ident, ok := call.Fun.(*ast.Ident)
		if !ok || ident.Name != funcPrintln {
			return true
		}

		printlnCalls = append(printlnCalls, call)
		// Extract the string literal argument
		if len(call.Args) > 0 {
			if lit, ok := call.Args[0].(*ast.BasicLit); ok {
				printlnMessages = append(printlnMessages, lit.Value)
			}
		}
		return true
	})

	expectedResults := []struct {
		message string
		inLoop  bool
		depth   int
	}{
		{`"outside"`, false, 0},
		{`"level 1"`, true, 1},
		{`"level 2"`, true, 2},
		{`"level 3"`, true, 3},
		{`"range loop"`, true, 1},
	}

	require.Len(t, printlnCalls, len(expectedResults), "Should have correct number of println calls")

	for i, call := range printlnCalls {
		inLoop := ctx.IsNodeInLoop(call)
		depth := ctx.GetNodeLoopDepth(call)

		assert.Equal(t, expectedResults[i].inLoop, inLoop,
			"Call %d (%s) inLoop should be %v", i, expectedResults[i].message, expectedResults[i].inLoop)
		assert.Equal(t, expectedResults[i].depth, depth,
			"Call %d (%s) depth should be %d", i, expectedResults[i].message, expectedResults[i].depth)
	}
}

func TestLoopContextEdgeCases(t *testing.T) {
	t.Run("nil node", func(t *testing.T) {
		code := `package test
func main() {}`

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
		require.NoError(t, err)

		// Test with nil target
		inLoop := IsInLoop(node, nil)
		assert.False(t, inLoop, "nil node should not be in loop")

		depth := GetLoopDepth(node, nil)
		assert.Equal(t, 0, depth, "nil node should have depth 0")
	})

	t.Run("node not in tree", func(t *testing.T) {
		code1 := `package test
func main() {
	for i := 0; i < 10; i++ {}
}`
		code2 := `package test
func other() {
	println("not in main")
}`

		fset := token.NewFileSet()
		node1, err := parser.ParseFile(fset, "test1.go", code1, parser.ParseComments)
		require.NoError(t, err)
		node2, err := parser.ParseFile(fset, "test2.go", code2, parser.ParseComments)
		require.NoError(t, err)

		// Find println in node2
		var printlnCall *ast.CallExpr
		ast.Inspect(node2, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == funcPrintln {
					printlnCall = call
					return false
				}
			}
			return true
		})

		// Check if node from different tree is in loop of node1
		inLoop := IsInLoop(node1, printlnCall)
		assert.False(t, inLoop, "node from different tree should not be in loop")

		depth := GetLoopDepth(node1, printlnCall)
		assert.Equal(t, 0, depth, "node from different tree should have depth 0")
	})

	t.Run("switch statement", func(t *testing.T) {
		// Switch statements are not loops
		code := `package test
func main() {
	x := 1
	switch x {
	case 1:
		println("in switch")
	}
}`

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
		require.NoError(t, err)

		var printlnCall *ast.CallExpr
		ast.Inspect(node, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == funcPrintln {
					printlnCall = call
					return false
				}
			}
			return true
		})

		inLoop := IsInLoop(node, printlnCall)
		assert.False(t, inLoop, "switch statement should not be considered a loop")
	})

	t.Run("if statement", func(t *testing.T) {
		// If statements are not loops
		code := `package test
func main() {
	if true {
		println("in if")
	}
}`

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
		require.NoError(t, err)

		var printlnCall *ast.CallExpr
		ast.Inspect(node, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == funcPrintln {
					printlnCall = call
					return false
				}
			}
			return true
		})

		inLoop := IsInLoop(node, printlnCall)
		assert.False(t, inLoop, "if statement should not be considered a loop")
	})
}

func TestLoopContextComplexScenarios(t *testing.T) {
	t.Run("loop in function literal", func(t *testing.T) {
		code := `package test
func main() {
	f := func() {
		for i := 0; i < 10; i++ {
			println("in closure loop")
		}
	}
	f()
}`

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
		require.NoError(t, err)

		var printlnCall *ast.CallExpr
		ast.Inspect(node, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == funcPrintln {
					printlnCall = call
					return false
				}
			}
			return true
		})

		require.NotNil(t, printlnCall)
		inLoop := IsInLoop(node, printlnCall)
		assert.True(t, inLoop, "println in closure loop should be detected as in loop")

		depth := GetLoopDepth(node, printlnCall)
		assert.Equal(t, 1, depth, "println in closure loop should have depth 1")
	})

	t.Run("parallel loops", func(t *testing.T) {
		code := `package test
func main() {
	for i := 0; i < 10; i++ {
		println("loop 1")
	}
	
	for j := 0; j < 10; j++ {
		println("loop 2")
	}
}`

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
		require.NoError(t, err)

		var printlnCalls []*ast.CallExpr
		ast.Inspect(node, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == funcPrintln {
					printlnCalls = append(printlnCalls, call)
				}
			}
			return true
		})

		require.Len(t, printlnCalls, 2)

		// Both should be in loops with depth 1
		for i, call := range printlnCalls {
			inLoop := IsInLoop(node, call)
			depth := GetLoopDepth(node, call)
			assert.True(t, inLoop, "println %d should be in loop", i)
			assert.Equal(t, 1, depth, "println %d should have depth 1", i)
		}
	})
}

func BenchmarkIsInLoop(b *testing.B) {
	code := `package test
func main() {
	for i := 0; i < 100; i++ {
		for j := 0; j < 100; j++ {
			println(i * j)
		}
	}
}`

	fset := token.NewFileSet()
	node, _ := parser.ParseFile(fset, "test.go", code, parser.ParseComments)

	var printlnCall *ast.CallExpr
	ast.Inspect(node, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == funcPrintln {
				printlnCall = call
				return false
			}
		}
		return true
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsInLoop(node, printlnCall)
	}
}

func BenchmarkGetLoopDepth(b *testing.B) {
	code := `package test
func main() {
	for i := 0; i < 100; i++ {
		for j := 0; j < 100; j++ {
			for k := 0; k < 100; k++ {
				println(i * j * k)
			}
		}
	}
}`

	fset := token.NewFileSet()
	node, _ := parser.ParseFile(fset, "test.go", code, parser.ParseComments)

	var printlnCall *ast.CallExpr
	ast.Inspect(node, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == funcPrintln {
				printlnCall = call
				return false
			}
		}
		return true
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetLoopDepth(node, printlnCall)
	}
}
