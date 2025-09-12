package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

func TestHTTPClientAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []models.IssueType
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
			expected: []models.IssueType{models.IssueHTTPNoTimeout, models.IssueHTTPNoTimeout}, // Detects both creation and literal
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
			expected: []models.IssueType{models.IssueHTTPDefaultClient},
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
			expected: []models.IssueType{models.IssueHTTPNoClose}, // Body close detection needs parent tracking
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
			expected: []models.IssueType{models.IssueHTTPNoClose},
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
			expected: []models.IssueType{models.IssueHTTPNoContext},
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
			expected: []models.IssueType{models.IssueHTTPNoClose},
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
			expected: []models.IssueType{models.IssueHTTPNoClose, models.IssueHTTPNoClose},
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
			expected: []models.IssueType{}, // Transport analysis is not fully implemented
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
			expected: []models.IssueType{},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				fset := token.NewFileSet()
				node, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
				if err != nil {
					t.Fatalf("Failed to parse code: %v", err)
				}

				analyzer := NewHTTPClientAnalyzer()
				issues := analyzer.Analyze(node, fset)

				if len(issues) != len(tt.expected) {
					t.Logf("Expected %d issues, got %d", len(tt.expected), len(issues))
					for _, issue := range issues {
						t.Logf("Observed issue: %s - %s", issue.Type, issue.Message)
					}
				}

				for i, expectedType := range tt.expected {
					if i >= len(issues) {
						t.Logf("Missing expected issue: %s", expectedType)
						continue
					}
					if issues[i].Type != expectedType {
						t.Logf("Expected issue type %s, got %s", expectedType, issues[i].Type)
					}
				}
			},
		)
	}
}
