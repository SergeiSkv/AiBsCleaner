package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestStructLayoutAnalyzer(t *testing.T) {
	tests := []struct {
		name          string
		code          string
		expectIssues  int
		expectMessage string
	}{
		{
			name: "Suboptimal struct layout",
			code: `package main
			
type SuboptimalStruct struct {
	A bool    // 1 byte
	B int64   // 8 bytes - will need 7 bytes padding after A
	C bool    // 1 byte - will need 7 bytes padding after this
}`,
			expectIssues:  3, // 1 layout issue + 2 large padding issues
			expectMessage: "has suboptimal field alignment",
		},
		{
			name: "Already optimal struct",
			code: `package main

type OptimalStruct struct {
	B int64   // 8 bytes
	A bool    // 1 byte  
	C bool    // 1 byte
}`,
			expectIssues: 1, // Still has final padding but no layout optimization possible
		},
		{
			name: "Empty struct",
			code: `package main

type EmptyStruct struct {
}`,
			expectIssues: 0,
		},
		{
			name: "Single field struct",
			code: `package main

type SingleField struct {
	Value int64
}`,
			expectIssues: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse code: %v", err)
			}

			analyzer := NewStructLayoutAnalyzer()
			issues := analyzer.Analyze(file, fset)

			if len(issues) != tt.expectIssues {
				t.Errorf("Expected %d issues, got %d", tt.expectIssues, len(issues))
				for i, issue := range issues {
					t.Logf("Issue %d: %s", i, issue.Message)
				}
			}

			if tt.expectMessage != "" && len(issues) > 0 {
				found := false
				for _, issue := range issues {
					if containsString(issue.Message, tt.expectMessage) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected issue message containing '%s', but not found", tt.expectMessage)
				}
			}
		})
	}
}

func TestStructLayoutVisualization(t *testing.T) {
	code := `package main

type TestStruct struct {
	A bool
	B int64
	C bool
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}

	analyzer := &StructLayoutAnalyzer{}
	var structType *ast.StructType
	var structName string

	ast.Inspect(file, func(n ast.Node) bool {
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if st, ok := typeSpec.Type.(*ast.StructType); ok {
				structType = st
				structName = typeSpec.Name.Name
				return false
			}
		}
		return true
	})

	if structType == nil {
		t.Fatal("No struct found in test code")
	}

	layout := analyzer.calculateLayout(structName, structType)
	visualization := analyzer.VisualizeLayout(layout)

	if visualization == "" {
		t.Error("Visualization should not be empty")
	}

	t.Logf("Layout visualization:\n%s", visualization)

	// Test that layout contains expected information
	if layout.TotalSize == 0 {
		t.Error("Layout total size should not be zero")
	}

	if layout.WastedBytes == 0 {
		t.Error("Expected some wasted bytes in suboptimal layout")
	}

	if len(layout.Suggestions) == 0 {
		t.Error("Expected optimization suggestions")
	}
}

func TestFieldTypeDetection(t *testing.T) {
	tests := []struct {
		name           string
		code           string
		expectedFields int
	}{
		{
			name: "Various field types",
			code: `package main

type VariousTypes struct {
	BoolField    bool
	IntField     int
	Int64Field   int64
	StringField  string
	PointerField *int
	SliceField   []byte
	ArrayField   [10]byte
}`,
			expectedFields: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse code: %v", err)
			}

			analyzer := &StructLayoutAnalyzer{}
			var structType *ast.StructType
			var structName string

			ast.Inspect(file, func(n ast.Node) bool {
				if typeSpec, ok := n.(*ast.TypeSpec); ok {
					if st, ok := typeSpec.Type.(*ast.StructType); ok {
						structType = st
						structName = typeSpec.Name.Name
						return false
					}
				}
				return true
			})

			if structType == nil {
				t.Fatal("No struct found in test code")
			}

			layout := analyzer.calculateLayout(structName, structType)

			// Count non-padding fields
			actualFields := 0
			for _, field := range layout.Fields {
				if !field.IsPadding {
					actualFields++
				}
			}

			if actualFields != tt.expectedFields {
				t.Errorf("Expected %d fields, got %d", tt.expectedFields, actualFields)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		(len(s) > len(substr) && containsString(s[1:], substr))
}
