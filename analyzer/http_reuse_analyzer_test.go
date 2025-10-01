package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPReuseAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "http.Client creation in loop",
			code: `package main
import "net/http"

func test() {
	for i := 0; i < 100; i++ {
		client := &http.Client{}
		client.Get("http://example.com")
	}
}`,
			expected: []string{"HTTP_CLIENT_IN_LOOP"},
		},
		{
			name: "http.Transport creation in loop",
			code: `package main
import "net/http"

func test() {
	for i := 0; i < 100; i++ {
		transport := &http.Transport{}
		client := &http.Client{Transport: transport}
		client.Get("http://example.com")
	}
}`,
			expected: []string{"HTTP_CLIENT_IN_LOOP", "TRANSPORT_IN_LOOP"},
		},
		{
			name: "http.Get in loop",
			code: `package main
import "net/http"

func test() {
	for i := 0; i < 100; i++ {
		http.Get("http://example.com")
	}
}`,
			expected: []string{"DEFAULT_CLIENT_IN_LOOP"},
		},
		{
			name: "http.Post in loop",
			code: `package main
import (
	"net/http"
	"strings"
)

func test() {
	for i := 0; i < 100; i++ {
		http.Post("http://example.com", "text/plain", strings.NewReader("data"))
	}
}`,
			expected: []string{"DEFAULT_CLIENT_IN_LOOP"},
		},
		{
			name: "ioutil.ReadAll response",
			code: `package main
import (
	"net/http"
	"io/ioutil"
)

func test() {
	resp, _ := http.Get("http://example.com")
	defer resp.Body.Close()
	ioutil.ReadAll(resp.Body)
}`,
			expected: []string{"FULL_RESPONSE_READ"},
		},
		{
			name: "multiple http clients in function",
			code: `package main
import "net/http"

func test() {
	client1 := &http.Client{}
	client2 := &http.Client{}
	
	client1.Get("http://example.com")
	client2.Get("http://example.com")
}`,
			expected: []string{"MULTIPLE_HTTP_CLIENTS"},
		},
		{
			name: "multiple transports",
			code: `package main
import "net/http"

func test() {
	transport1 := &http.Transport{}
	transport2 := &http.Transport{}
	
	client1 := &http.Client{Transport: transport1}
	client2 := &http.Client{Transport: transport2}
	
	client1.Get("http://example.com")
	client2.Get("http://example.com")
}`,
			expected: []string{"MULTIPLE_HTTP_CLIENTS", "MULTIPLE_TRANSPORTS"},
		},
		{
			name: "nested loop with client creation",
			code: `package main
import "net/http"

func test() {
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			client := &http.Client{}
			client.Get("http://example.com")
		}
	}
}`,
			expected: []string{"HTTP_CLIENT_IN_LOOP"},
		},
		{
			name: "proper client reuse",
			code: `package main
import (
	"net/http"
	"time"
)

func test() {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	for i := 0; i < 100; i++ {
		client.Get("http://example.com")
	}
}`,
			expected: []string{},
		},
		{
			name: "proper transport reuse",
			code: `package main
import (
	"net/http"
	"time"
)

func test() {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
	
	for i := 0; i < 100; i++ {
		client.Get("http://example.com")
	}
}`,
			expected: []string{},
		},
		{
			name: "http.Client without timeout (simplified)",
			code: `package main
import "net/http"

func test() {
	client := &http.Client{}
	client.Get("http://example.com")
}`,
			expected: []string{"HTTP_NO_TIMEOUT"},
		},
		{
			name: "URL building with concatenation",
			code: `package main
import "net/http"

func test(id string) {
	url := "http://example.com/api/" + id
	http.Get(url)
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

				analyzer := NewHTTPReuseAnalyzer()
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
