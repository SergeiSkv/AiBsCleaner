package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrivacyAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected int
	}{
		{
			name: "const secret literal",
			code: `package main
const apiSecret = "sk-1234567890abcdef"
func run() { _ = apiSecret }`,
			expected: 1,
		},
		{
			name: "var password assignment",
			code: `package main
var password string
func init() { password = "MyS3cr3tP@ssw0rd!" }`,
			expected: 1,
		},
		{
			name: "jwt literal",
			code: `package main
const token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhIjoiYiJ9.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
func main() { _ = token }`,
			expected: 1,
		},
		{
			name: "aws key literal",
			code: `package main
const accessKey = "AKIAIOSFODNN7EXAMPLE"
func use() { _ = accessKey }`,
			expected: 1,
		},
		{
			name: "placeholder value",
			code: `package main
const apiSecret = "${API_SECRET}"
func use() { _ = apiSecret }`,
			expected: 0,
		},
		{
			name: "key name constant (false positive check)",
			code: `package main
const AuthUserKey = "user"
const SessionKey = "session"
func use() { _, _ = AuthUserKey, SessionKey }`,
			expected: 0,
		},
		{
			name: "short simple word (false positive check)",
			code: `package main
const password = "admin"
func use() { _ = password }`,
			expected: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tc.code, parser.ParseComments)
			require.NoError(t, err)

			analyzer := NewPrivacyAnalyzer()
			issues := analyzer.Analyze(file, fset)
			if len(issues) != tc.expected {
				t.Fatalf("expected %d issues, got %d", tc.expected, len(issues))
			}
		})
	}
}
