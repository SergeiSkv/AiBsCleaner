package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCryptoAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "math/rand for security",
			code: `package main
import "math/rand"

func generateToken() string {
	token := make([]byte, 32)
	rand.Read(token)
	return string(token)
}`,
			expected: []string{"WEAK_RANDOM"},
		},
		{
			name: "crypto operations in loop",
			code: `package main
import "crypto/sha256"

func test() {
	for i := 0; i < 100; i++ {
		h := sha256.New()
		h.Write([]byte("data"))
		h.Sum(nil)
	}
}`,
			expected: []string{"CRYPTO_IN_LOOP"},
		},
		{
			name: "key derivation in loop",
			code: `package main
import "golang.org/x/crypto/bcrypt"

func test() {
	for i := 0; i < 10; i++ {
		bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	}
}`,
			expected: []string{"KEY_DERIVATION_IN_LOOP"},
		},
		{
			name: "weak hash MD5",
			code: `package main
import "crypto/md5"

func test() {
	h := md5.New()
	h.Write([]byte("data"))
	h.Sum(nil)
}`,
			expected: []string{"WEAK_HASH"},
		},
		{
			name: "weak hash SHA1",
			code: `package main
import "crypto/sha1"

func test() {
	h := sha1.New()
	h.Write([]byte("data"))
	h.Sum(nil)
}`,
			expected: []string{"WEAK_HASH"},
		},
		{
			name: "cipher creation in loop",
			code: `package main
import (
	"crypto/aes"
)

func test(key []byte) {
	for i := 0; i < 100; i++ {
		aes.NewCipher(key)
	}
}`,
			expected: []string{"CIPHER_CREATION_IN_LOOP"},
		},
		{
			name: "large RSA key",
			code: `package main
import (
	"crypto/rand"
	"crypto/rsa"
)

func test() {
	rsa.GenerateKey(rand.Reader, 4096)
}`,
			expected: []string{"LARGE_RSA_KEY"},
		},
		{
			name: "crypto/rand read",
			code: `package main
import "crypto/rand"

func test() {
	b := make([]byte, 32)
	rand.Read(b)
}`,
			expected: []string{"SYNC_RANDOM_READ"},
		},
		{
			name: "nested loop crypto",
			code: `package main
import "crypto/sha256"

func test() {
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			sha256.Sum256([]byte("data"))
		}
	}
}`,
			expected: []string{"CRYPTO_IN_LOOP"},
		},
		{
			name: "PBKDF2 in loop",
			code: `package main
import "golang.org/x/crypto/pbkdf2"
import "crypto/sha256"

func test() {
	for i := 0; i < 10; i++ {
		pbkdf2.Key([]byte("password"), []byte("salt"), 10000, 32, sha256.New)
	}
}`,
			expected: []string{"KEY_DERIVATION_IN_LOOP"},
		},
		{
			name: "scrypt in loop",
			code: `package main
import "golang.org/x/crypto/scrypt"

func test() {
	for i := 0; i < 10; i++ {
		scrypt.Key([]byte("password"), []byte("salt"), 16384, 8, 1, 32)
	}
}`,
			expected: []string{"KEY_DERIVATION_IN_LOOP"},
		},
		{
			name: "proper crypto usage",
			code: `package main
import (
	"crypto/rand"
	"crypto/sha256"
)

func test() {
	// Use crypto/rand for secure random
	b := make([]byte, 32)
	rand.Read(b)
	
	// Hash once, not in loop
	h := sha256.New()
	h.Write([]byte("data"))
	h.Sum(nil)
}`,
			expected: []string{"SYNC_RANDOM_READ"},
		},
		{
			name: "no crypto issues",
			code: `package main

func test() {
	// No crypto operations
	for i := 0; i < 100; i++ {
		println(i)
	}
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

				analyzer := NewCryptoAnalyzer()
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
