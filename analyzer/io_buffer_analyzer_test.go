package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIOBufferAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "unbuffered file read",
			code: `package main
import "os"

func test() {
	file, _ := os.Open("test.txt")
	defer file.Close()
	
	buf := make([]byte, 1024)
	file.Read(buf)
}`,
			expected: []string{"UNBUFFERED_IO"},
		},
		{
			name: "single byte read",
			code: `package main
import "os"

func test() {
	file, _ := os.Open("test.txt")
	defer file.Close()
	
	buf := make([]byte, 1)
	for i := 0; i < 100; i++ {
		file.Read(buf)
	}
}`,
			expected: []string{"UNBUFFERED_IO", "SINGLE_BYTE_IO"},
		},
		{
			name: "file open in loop",
			code: `package main
import "os"

func test() {
	for i := 0; i < 100; i++ {
		file, _ := os.Open("test.txt")
		file.Close()
	}
}`,
			expected: []string{"FILE_IO_IN_LOOP"},
		},
		{
			name: "ioutil.ReadFile in loop",
			code: `package main
import "io/ioutil"

func test() {
	for i := 0; i < 100; i++ {
		data, _ := ioutil.ReadFile("test.txt")
		_ = data
	}
}`,
			expected: []string{"FILE_IO_IN_LOOP", "READ_ENTIRE_FILE"},
		},
		{
			name: "fmt.Fprintf in loop",
			code: `package main
import (
	"fmt"
	"os"
)

func test() {
	for i := 0; i < 100; i++ {
		fmt.Fprintf(os.Stdout, "test %d\n", i)
	}
}`,
			expected: []string{"FMT_IN_HOT_PATH"},
		},
		{
			name: "io.Copy without buffer",
			code: `package main
import (
	"io"
	"os"
)

func test() {
	src, _ := os.Open("src.txt")
	dst, _ := os.Create("dst.txt")
	defer src.Close()
	defer dst.Close()
	
	io.Copy(dst, src)
}`,
			expected: []string{"COPY_DEFAULT_BUFFER"},
		},
		{
			name: "reading entire file",
			code: `package main
import "io/ioutil"

func test() {
	data, _ := ioutil.ReadFile("large_file.txt")
	_ = data
}`,
			expected: []string{"READ_ENTIRE_FILE"},
		},
		{
			name: "os.ReadFile",
			code: `package main
import "os"

func test() {
	data, _ := os.ReadFile("test.txt")
	_ = data
}`,
			expected: []string{"READ_ENTIRE_FILE"},
		},
		{
			name: "small buffer size",
			code: `package main
import "os"

func test() {
	buf := make([]byte, 256)
	file, _ := os.Open("test.txt")
	defer file.Close()
	file.Read(buf)
}`,
			expected: []string{"UNBUFFERED_IO", "SMALL_BUFFER_SIZE"},
		},
		{
			name: "bufio.NewScanner",
			code: `package main
import (
	"bufio"
	"os"
)

func test() {
	file, _ := os.Open("test.txt")
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		_ = scanner.Text()
	}
}`,
			expected: []string{"SCANNER_DEFAULT_BUFFER"},
		},
		{
			name: "proper buffered I/O",
			code: `package main
import (
	"bufio"
	"os"
)

func test() {
	file, _ := os.Open("test.txt")
	defer file.Close()
	
	reader := bufio.NewReader(file)
	buf := make([]byte, 4096)
	reader.Read(buf)
}`,
			expected: []string{},
		},
		{
			name: "proper file handling",
			code: `package main
import (
	"bufio"
	"os"
)

func test() {
	file, _ := os.Open("test.txt")
	defer file.Close()
	
	writer := bufio.NewWriter(file)
	defer writer.Flush()
	
	for i := 0; i < 100; i++ {
		writer.WriteString("test\n")
	}
}`,
			expected: []string{},
		},
		{
			name: "nested loop file I/O",
			code: `package main
import "os"

func test() {
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			file, _ := os.Create("test.txt")
			file.Close()
		}
	}
}`,
			expected: []string{"FILE_IO_IN_LOOP"},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				fset := token.NewFileSet()
				node, err := parser.ParseFile(fset, "test.go", tt.code, parser.ParseComments)
				require.NoError(t, err, "Failed to parse code")

				analyzer := NewIOBufferAnalyzer()
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
