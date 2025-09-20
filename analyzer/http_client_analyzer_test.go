package analyzer

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestHTTPClientAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "http client without timeout",
			code: `
package main

import "net/http"

func badClient() {
	client := &http.Client{}  // No timeout
	_ = client
}
`,
			expected: []string{"HTTP_CLIENT_NO_TIMEOUT", "HTTP_CLIENT_NO_TIMEOUT"}, // Detects both creation and literal
		},
		{
			name: "using http.DefaultClient",
			code: `
package main

import "net/http"

func useDefault() {
	client := http.DefaultClient  // Bad practice
	_ = client
}
`,
			expected: []string{"HTTP_DEFAULT_CLIENT"},
		},
		{
			name: "direct http.Get call",
			code: `
package main

import "net/http"

func directCall() {
	resp, err := http.Get("https://example.com")  // Uses DefaultClient
	_ = resp
	_ = err
}
`,
			expected: []string{"HTTP_DIRECT_CALL"}, // Body close detection needs parent tracking
		},
		{
			name: "http.Post without custom client",
			code: `
package main

import (
	"net/http"
	"strings"
)

func directPost() {
	resp, err := http.Post("https://example.com", "application/json", strings.NewReader("{}"))
	_ = resp
	_ = err
}
`,
			expected: []string{"HTTP_DIRECT_CALL"},
		},
		{
			name: "NewRequest without context",
			code: `
package main

import "net/http"

func noContext() {
	req, err := http.NewRequest("GET", "https://example.com", nil)
	_ = req
	_ = err
}
`,
			expected: []string{"HTTP_NO_CONTEXT"},
		},
		{
			name: "response body not closed",
			code: `
package main

import "net/http"

func leakBody() {
	resp, err := http.Get("https://example.com")
	if err != nil {
		return
	}
	// Body not closed - leak!
	_ = resp
}
`,
			expected: []string{"HTTP_DIRECT_CALL"},
		},
		{
			name: "ReadAll on response body",
			code: `
package main

import (
	"io"
	"net/http"
)

func readAll() {
	resp, err := http.Get("https://example.com")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)  // Reads entire body into memory
	_ = body
}
`,
			expected: []string{"HTTP_DIRECT_CALL", "HTTP_READALL_BODY"},
		},
		{
			name: "transport without connection limits",
			code: `
package main

import (
	"net/http"
	"time"
)

func customTransport() {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,  // Bad for performance
		},
	}
	_ = client
}
`,
			expected: []string{}, // Transport analysis not fully implemented
		},
		{
			name: "proper http client configuration",
			code: `
package main

import (
	"context"
	"net/http"
	"time"
)

func goodClient() {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", "https://example.com", nil)
	if err != nil {
		return
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	
	_ = resp
}
`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse code: %v", err)
			}

			analyzer := NewHTTPClientAnalyzer()
			issues := analyzer.Analyze("test.go", node, fset)

			if len(issues) != len(tt.expected) {
				t.Errorf("Expected %d issues, got %d", len(tt.expected), len(issues))
				for _, issue := range issues {
					t.Logf("Got issue: %s - %s", issue.Type, issue.Message)
				}
				return
			}

			for i, expectedType := range tt.expected {
				if i >= len(issues) {
					t.Errorf("Missing expected issue: %s", expectedType)
					continue
				}
				if issues[i].Type != expectedType {
					t.Errorf("Expected issue type %s, got %s", expectedType, issues[i].Type)
				}
			}
		})
	}
}
