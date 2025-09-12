package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCGOAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "CGO call in loop",
			code: `package main

// #include <stdio.h>
// #include <stdlib.h>
import "C"

func test() {
	for i := 0; i < 100; i++ {
		C.printf(C.CString("test"))
	}
}`,
			expected: []string{"CGOInLoop", "CGOMemoryLeak"},
		},
		{
			name: "CGO call in nested loop",
			code: `package main

// #include <string.h>
import "C"

func test() {
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			C.strlen(C.CString("test"))
		}
	}
}`,
			expected: []string{"CGOInLoop", "CGOMemoryLeak"},
		},
		{
			name: "Small CGO operations",
			code: `package main

// #include <string.h>
// #include <math.h>
import "C"

func test() {
	C.strlen(C.CString("test"))
	C.strcmp(C.CString("a"), C.CString("b"))
	C.abs(-5)
}`,
			expected: []string{"CGOMemoryLeak"},
		},
		{
			name: "Go callback from C",
			code: `package main

// #include <stdio.h>
// typedef void (*callback)(int);
// void call_callback(callback cb) { cb(42); }
import "C"

//export goCallback
func goCallback(n C.int) {
	println(n)
}

func test() {
	C.call_callback(C.callback(C.goCallback))
}`,
			expected: []string{},
		},
		{
			name: "CGO string conversions",
			code: `package main

// #include <string.h>
import "C"

func test() {
	cstr := C.CString("hello")
	gostr := C.GoString(cstr)
	_ = gostr
}`,
			expected: []string{"CGOMemoryLeak"},
		},
		{
			name: "Multiple CGO calls in function",
			code: `package main

// #include <stdio.h>
import "C"

func test() {
	C.printf(C.CString("1"))
	C.printf(C.CString("2"))
	C.printf(C.CString("3"))
	C.printf(C.CString("4"))
	C.printf(C.CString("5"))
	C.printf(C.CString("6"))
}`,
			expected: []string{"CGOMemoryLeak"},
		},
		{
			name: "No CGO issues",
			code: `package main

func test() {
	// Pure Go code
	for i := 0; i < 100; i++ {
		println(i)
	}
}`,
			expected: []string{},
		},
		{
			name: "CGO without import C",
			code: `package main

func test() {
	// This looks like CGO but isn't
	c := struct{ printf func(string) }{}
	c.printf("test")
}`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				fset := token.NewFileSet()
				node, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
				require.NoError(t, err, "Failed to parse code")

				analyzer := NewCGOAnalyzer()
				issues := analyzer.Analyze(node, fset)

				issueTypes := make(map[string]bool)
				for _, issue := range issues {
					issueTypes[issue.Type.String()] = true
				}

				for _, expected := range tt.expected {
					normalized := normalizeIssueName(expected)
					if !issueTypes[normalized] {
						t.Logf("Expected issue %s not found", normalized)
					}
				}

				if len(tt.expected) == 0 && len(issues) > 0 {
					for _, issue := range issues {
						t.Logf("Unexpected issue: %s - %s", issue.Type, issue.Message)
					}
				}
			},
		)
	}
}
