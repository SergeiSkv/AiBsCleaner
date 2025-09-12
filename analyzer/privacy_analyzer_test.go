package analyzer

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrivacyAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected []string
	}{
		{
			name: "hardcoded API key",
			code: `package main
const apiKey = "sk-1234567890abcdef1234567890abcdef"
func test() {
	useAPI(apiKey)
}`,
			expected: []string{"PRIVACY_HARDCODED_SECRET"},
		},
		{
			name: "hardcoded password",
			code: `package main
var password = "mysecretpassword123"
func test() {
	authenticate(password)
}`,
			expected: []string{"PRIVACY_HARDCODED_SECRET"},
		},
		{
			name: "hardcoded JWT token",
			code: `package main
const token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
func test() {
	authorize(token)
}`,
			expected: []string{"PRIVACY_JWT_TOKEN"},
		},
		{
			name: "hardcoded AWS access key",
			code: `package main
const awsKey = "AKIAIOSFODNN7EXAMPLE"
func test() {
	awsClient(awsKey)
}`,
			expected: []string{"PRIVACY_AWS_KEY"},
		},
		{
			name: "email address in code",
			code: `package main
const adminEmail = "admin@company.com"
func test() {
	sendAlert(adminEmail)
}`,
			expected: []string{"PRIVACY_EMAIL_PII"},
		},
		{
			name: "SSN in code",
			code: `package main
const testSSN = "123-45-6789"
func test() {
	validateSSN(testSSN)
}`,
			expected: []string{"PRIVACY_SSN_PII"},
		},
		{
			name: "credit card number in code",
			code: `package main
const testCard = "4532-1234-5678-9012"
func test() {
	processPayment(testCard)
}`,
			expected: []string{"PRIVACY_CREDIT_CARD_PII"},
		},
		{
			name: "logging sensitive data",
			code: `package main
import "log"
func test(apiKey string) {
	log.Printf("Using API key: %s", apiKey)
}`,
			expected: []string{"PRIVACY_LOGGING_SENSITIVE"},
		},
		{
			name: "printing sensitive data with fmt",
			code: `package main
import "fmt"
func test(password string) {
	fmt.Printf("Password is: %s", password)
}`,
			expected: []string{"PRIVACY_PRINTING_SENSITIVE"},
		},
		{
			name: "sensitive struct field without proper JSON tag",
			code: `package main
type User struct {
	Name     string ` + "`json:\"name\"`" + `
	Password string ` + "`json:\"password\"`" + `
}`,
			expected: []string{"PRIVACY_EXPOSED_FIELD"},
		},
		{
			name: "sensitive field with omitempty - no issue",
			code: `package main
type User struct {
	Name     string ` + "`json:\"name\"`" + `
	Password string ` + "`json:\"password,omitempty\"`" + `
}`,
			expected: []string{},
		},
		{
			name: "sensitive field excluded from JSON - no issue",
			code: `package main
type User struct {
	Name     string ` + "`json:\"name\"`" + `
	Password string ` + "`json:\"-\"`" + `
}`,
			expected: []string{},
		},
		{
			name: "template variables - no issue",
			code: `package main
const apiKey = "${API_KEY}"
const dbUrl = "{{DATABASE_URL}}"
func test() {
	connect(apiKey, dbUrl)
}`,
			expected: []string{},
		},
		{
			name: "example values - no issue",
			code: `package main
const exampleKey = "your-api-key-here"
const placeholder = "xxx-xxx-xxxx"
func test() {
	configure(exampleKey, placeholder)
}`,
			expected: []string{},
		},
		{
			name: "test file with secrets - lower severity",
			code: `package main
const testPassword = "testpass123"
func TestLogin(t *testing.T) {
	login("testuser", testPassword)
}`,
			expected: []string{"PRIVACY_HARDCODED_SECRET"}, // Still detected but with lower severity
		},
		{
			name: "multiple privacy issues",
			code: `package main
import (
	"fmt"
	"log"
)

const (
	apiKey = "sk-1234567890abcdef"
	dbPassword = "mysecretpass"
)

type Config struct {
	ApiKey   string ` + "`json:\"api_key\"`" + `
	Secret   string ` + "`json:\"secret\"`" + `
}

func test(token string) {
	log.Printf("Token: %s", token)
	fmt.Printf("API key: %s", apiKey)
}`,
			expected: []string{
				"PRIVACY_HARDCODED_SECRET",
				"PRIVACY_EXPOSED_FIELD",
				"PRIVACY_LOGGING_SENSITIVE",
				"PRIVACY_PRINTING_SENSITIVE",
			},
		},
		{
			name: "proper security practices - no issues",
			code: `package main
import (
	"os"
	"log"
)

type Config struct {
	ApiKey   string ` + "`json:\"-\"`" + `
	Secret   string ` + "`json:\"secret,omitempty\"`" + `
	PublicKey string ` + "`json:\"public_key\"`" + `
}

func test() {
	apiKey := os.Getenv("API_KEY")
	if apiKey != "" {
		log.Println("API key loaded from environment")
		useAPI(apiKey)
	}
}`,
			expected: []string{},
		},
		{
			name: "database operations with sensitive data",
			code: `package main
import "database/sql"

func storeUser(db *sql.DB, email, password string) {
	db.Exec("INSERT INTO users (email, password) VALUES (?, ?)", email, password)
}`,
			expected: []string{"PRIVACY_UNENCRYPTED_DB_WRITE"},
		},
		{
			name: "encrypted data storage - no sensitive data",
			code: `package main
import (
	"database/sql"
	"golang.org/x/crypto/bcrypt"
)

func storeUser(db *sql.DB, name, password string) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	db.Exec("INSERT INTO users (name, password_hash) VALUES (?, ?)", name, hashedPassword)
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

				analyzer := NewPrivacyAnalyzer()
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
				}
			},
		)
	}
}
