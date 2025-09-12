package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSerializationAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "JSON Marshal in loop",
			code: `package main
import "encoding/json"

func test() {
	for i := 0; i < 100; i++ {
		data := struct{Name string}{Name: "test"}
		json.Marshal(data)
	}
}`,
			expected: []string{"JSON.MARSHAL_IN_LOOP"},
		},
		{
			name: "XML Marshal in nested loop",
			code: `package main
import "encoding/xml"

func test() {
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			data := struct{Name string}{Name: "test"}
			xml.Marshal(data)
		}
	}
}`,
			expected: []string{"XML.MARSHAL_IN_LOOP"},
		},
		{
			name: "Encoder creation in loop",
			code: `package main
import (
	"encoding/json"
	"os"
)

func test() {
	for i := 0; i < 100; i++ {
		enc := json.NewEncoder(os.Stdout)
		enc.Encode(i)
	}
}`,
			expected: []string{"ENCODER_CREATION_IN_LOOP"},
		},
		{
			name: "Marshal to string conversion",
			code: `package main
import "encoding/json"

func test() {
	data := struct{Name string}{Name: "test"}
	b, _ := json.Marshal(data)
	s := string(b)
	_ = s
}`,
			expected: []string{"MARSHAL_TO_STRING"},
		},
		{
			name: "Pretty print JSON",
			code: `package main
import "encoding/json"

func test() {
	data := struct{Name string}{Name: "test"}
	json.MarshalIndent(data, "", "  ")
}`,
			expected: []string{"PRETTY_PRINT_OVERHEAD"},
		},
		{
			name: "Base64 encoding in loop",
			code: `package main
import "encoding/base64"

func test() {
	for i := 0; i < 100; i++ {
		base64.StdEncoding.EncodeToString([]byte("test"))
	}
}`,
			expected: []string{"BASE64_IN_LOOP"},
		},
		{
			name: "Multiple serialization issues",
			code: `package main
import (
	"encoding/json"
	"encoding/xml"
)

func test() {
	for i := 0; i < 100; i++ {
		data := map[string]interface{}{"key": "value"}
		b, _ := json.Marshal(data)
		s := string(b)
		xml.Marshal(s)
	}
}`,
			expected: []string{"JSON.MARSHAL_IN_LOOP", "INTERFACE_MAP_MARSHAL", "MARSHAL_TO_STRING", "XML.MARSHAL_IN_LOOP"},
		},
		{
			name: "Decoder creation in loop",
			code: `package main
import (
	"encoding/json"
	"strings"
)

func test() {
	for i := 0; i < 100; i++ {
		dec := json.NewDecoder(strings.NewReader("{}"))
		var v interface{}
		dec.Decode(&v)
	}
}`,
			expected: []string{"ENCODER_CREATION_IN_LOOP"},
		},
		{
			name: "YAML in loop",
			code: `package main
import "gopkg.in/yaml.v2"

func test() {
	for i := 0; i < 100; i++ {
		data := struct{Name string}{Name: "test"}
		yaml.Marshal(data)
	}
}`,
			expected: []string{"YAML.MARSHAL_IN_LOOP"},
		},
		{
			name: "No serialization issues",
			code: `package main
import "encoding/json"

func test() {
	// Create encoder once
	enc := json.NewEncoder(os.Stdout)
	
	for i := 0; i < 100; i++ {
		// Reuse encoder
		enc.Encode(i)
	}
}`,
			expected: []string{},
		},
		{
			name: "Marshal outside loop - proper usage",
			code: `package main
import "encoding/json"

func test() {
	data := struct{Name string}{Name: "test"}
	b, _ := json.Marshal(data)
	s := string(b) // Convert once
	
	for i := 0; i < 100; i++ {
		// Use marshaled data without repeated conversion
		println(s)
	}
}`,
			expected: []string{"MARSHAL_TO_STRING"}, // Still has the string conversion, but not in loop
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				fset := token.NewFileSet()
				node, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
				require.NoError(t, err, "Failed to parse code")

				analyzer := NewSerializationAnalyzer()
				issues := analyzer.Analyze(node, fset)

				issueTypes := make(map[string]bool)
				for _, issue := range issues {
					issueTypes[issue.Type.String()] = true
				}

				for _, expected := range tt.expected {
					assert.True(t, issueTypes[expected], "Expected issue %s not found", expected)
				}

				if len(tt.expected) == 0 {
					assert.Empty(t, issues, "Expected no issues, but found %d", len(issues))
					if len(issues) > 0 {
						for _, issue := range issues {
							t.Logf("  - %s: %s", issue.Type, issue.Message)
						}
					}
				}
			},
		)
	}
}
