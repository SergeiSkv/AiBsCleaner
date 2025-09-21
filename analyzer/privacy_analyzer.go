package analyzer

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"
)

type PrivacyAnalyzer struct {
	// Track encrypted variables for data flow analysis
	encryptedVars map[string]bool
	// Track variables that come from direct user input
	userInputVars map[string]bool
}

func NewPrivacyAnalyzer() *PrivacyAnalyzer {
	return &PrivacyAnalyzer{
		encryptedVars: make(map[string]bool),
		userInputVars: make(map[string]bool),
	}
}

var (
	// Patterns for sensitive data
	apiKeyPattern     = regexp.MustCompile(`(?i)(api[_-]?key|apikey|api_secret)`)
	passwordPattern   = regexp.MustCompile(`(?i)(password|passwd|pwd|pass)`)
	tokenPattern      = regexp.MustCompile(`(?i)(token|auth|bearer|jwt|oauth)`)
	secretPattern     = regexp.MustCompile(`(?i)(secret|private[_-]?key|priv_key)`)
	credentialPattern = regexp.MustCompile(`(?i)(credential|cred|username)`)
	dbPattern         = regexp.MustCompile(`(?i)(database_url|db_url|connection_string|conn_str|dsn)`)
	awsPattern        = regexp.MustCompile(`(?i)(aws_access_key|aws_secret|aws_key)`)
	sshPattern        = regexp.MustCompile(`(?i)(ssh_key|id_rsa|private_key)`)

	// Patterns for actual secrets in code
	hardcodedAPIKey = regexp.MustCompile(`[A-Za-z0-9]{32,}`)
	hardcodedJWT    = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*`)
	hardcodedBearer = regexp.MustCompile(`Bearer\s+[A-Za-z0-9_-]{20,}`)
	awsKeyPattern   = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)

	// PII patterns
	emailPattern = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	phonePattern = regexp.MustCompile(`(\+\d{1,3}[-.\s]?)?\(?\d{1,4}\)?[-.\s]?\d{1,4}[-.\s]?\d{1,9}`)
	ssnPattern   = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	ccPattern    = regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`)
	ipPattern    = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`)
)

func (a *PrivacyAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Reset tracking for new file
	a.encryptedVars = make(map[string]bool)
	a.userInputVars = make(map[string]bool)

	// Skip test files for some checks
	isTestFile := strings.HasSuffix(filename, "_test.go")

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.GenDecl:
				a.checkGenDecl(node, fset, &issues, isTestFile)
			case *ast.AssignStmt:
				a.checkAssignment(node, fset, &issues, isTestFile)
				a.trackDataFlow(node) // Track data flow for encryption detection
			case *ast.BasicLit:
				a.checkLiteral(node, fset, &issues, isTestFile)
			case *ast.CallExpr:
				a.checkFunctionCall(node, fset, &issues)
				a.trackEncryption(node)                    // Track encryption function calls
				a.checkDatabaseWrites(node, fset, &issues) // Check for unencrypted DB writes
			case *ast.Field:
				a.checkStructField(node, fset, &issues)
			}
			return true
		},
	)

	return issues
}

func (a *PrivacyAnalyzer) checkGenDecl(decl *ast.GenDecl, fset *token.FileSet, issues *[]Issue, isTestFile bool) {
	if decl.Tok != token.CONST && decl.Tok != token.VAR {
		return
	}

	for _, spec := range decl.Specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}

		for i, name := range valueSpec.Names {
			nameStr := name.Name

			// Check for sensitive variable names
			if a.isSensitiveName(nameStr) {
				// Check if it's hardcoded
				if i < len(valueSpec.Values) {
					if lit, ok := valueSpec.Values[i].(*ast.BasicLit); ok {
						if lit.Kind == token.STRING && lit.Value != `""` && lit.Value != `"` {
							value := strings.Trim(lit.Value, `"`)
							if !strings.HasPrefix(value, "${") && !strings.HasPrefix(value, "{{") {
								var severity = SeverityHigh
								if isTestFile {
									severity = SeverityLow
								}
								*issues = append(*issues, createIssue(fset, name.Pos(),
									"PRIVACY_HARDCODED_SECRET",
									"Hardcoded sensitive value in variable: "+nameStr,
									severity))
							}
						}
					}
				}
			}
		}
	}
}

func (a *PrivacyAnalyzer) checkAssignment(assign *ast.AssignStmt, fset *token.FileSet, issues *[]Issue, isTestFile bool) {
	for i, lhs := range assign.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok {
			if a.isSensitiveName(ident.Name) {
				if i < len(assign.Rhs) {
					if lit, ok := assign.Rhs[i].(*ast.BasicLit); ok {
						if lit.Kind == token.STRING && lit.Value != `""` {
							value := strings.Trim(lit.Value, `"`)
							if !strings.HasPrefix(value, "${") && !strings.HasPrefix(value, "{{") && len(value) > 0 {
								var severity Severity = SeverityHigh
								if isTestFile {
									severity = SeverityLow
								}
								*issues = append(*issues, createIssue(fset, ident.Pos(),
									"PRIVACY_HARDCODED_SECRET",
									"Hardcoded sensitive value assigned to: "+ident.Name,
									severity))
							}
						}
					}
				}
			}
		}
	}
}

func (a *PrivacyAnalyzer) checkLiteral(lit *ast.BasicLit, fset *token.FileSet, issues *[]Issue, isTestFile bool) {
	if lit.Kind != token.STRING {
		return
	}

	value := strings.Trim(lit.Value, `"`)
	if len(value) < 10 {
		return
	}

	// Skip example/placeholder values
	if strings.Contains(value, "example") || strings.Contains(value, "your-") ||
		strings.Contains(value, "xxx") || strings.Contains(value, "...") {
		return
	}

	// Check for hardcoded secrets
	if awsKeyPattern.MatchString(value) {
		*issues = append(*issues, createIssue(fset, lit.Pos(),
			"PRIVACY_AWS_KEY",
			"Potential AWS access key found in code",
			SeverityHigh))
	}

	if hardcodedJWT.MatchString(value) {
		var severity Severity = SeverityHigh
		if isTestFile {
			severity = SeverityMedium
		}
		*issues = append(*issues, createIssue(fset, lit.Pos(),
			"PRIVACY_JWT_TOKEN",
			"Hardcoded JWT token found",
			severity))
	}

	// Check for PII in non-test files
	if !isTestFile {
		if emailPattern.MatchString(value) && !strings.Contains(value, "@example.") {
			*issues = append(*issues, createIssue(fset, lit.Pos(),
				"PRIVACY_EMAIL_PII",
				"Email address found in code (potential PII)",
				SeverityMedium))
		}

		if ssnPattern.MatchString(value) {
			*issues = append(*issues, createIssue(fset, lit.Pos(),
				"PRIVACY_SSN_PII",
				"SSN pattern found in code (potential PII)",
				SeverityHigh))
		}

		if ccPattern.MatchString(value) && !strings.Contains(value, "0000") {
			*issues = append(*issues, createIssue(fset, lit.Pos(),
				"PRIVACY_CREDIT_CARD_PII",
				"Credit card pattern found in code (potential PII)",
				SeverityHigh))
		}
	}
}

func (a *PrivacyAnalyzer) checkFunctionCall(call *ast.CallExpr, fset *token.FileSet, issues *[]Issue) {
	// Check for logging sensitive data
	if fun, ok := call.Fun.(*ast.SelectorExpr); ok {
		funcName := fun.Sel.Name
		if isLoggingFunction(funcName) {
			for _, arg := range call.Args {
				if ident, ok := arg.(*ast.Ident); ok {
					if a.isSensitiveName(ident.Name) {
						*issues = append(*issues, createIssue(fset, call.Pos(),
							"PRIVACY_LOGGING_SENSITIVE",
							"Logging potentially sensitive data: "+ident.Name,
							SeverityMedium))
					}
				}
			}
		}
	}

	// Check fmt.Printf/Sprintf for sensitive data
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "fmt" {
			if sel.Sel.Name == "Printf" || sel.Sel.Name == "Sprintf" || sel.Sel.Name == "Fprintf" {
				for _, arg := range call.Args[1:] { // Skip format string
					if ident, ok := arg.(*ast.Ident); ok {
						if a.isSensitiveName(ident.Name) {
							*issues = append(*issues, createIssue(fset, call.Pos(),
								"PRIVACY_PRINTING_SENSITIVE",
								"Printing potentially sensitive data: "+ident.Name,
								SeverityMedium))
						}
					}
				}
			}
		}
	}
}

func (a *PrivacyAnalyzer) checkStructField(field *ast.Field, fset *token.FileSet, issues *[]Issue) {
	if field.Tag == nil {
		return
	}

	tag := strings.Trim(field.Tag.Value, "`")

	// Check for sensitive fields without proper tags
	for _, name := range field.Names {
		if a.isSensitiveName(name.Name) {
			// Check if field is exposed in JSON without omitempty or -
			if strings.Contains(tag, "json:") && !strings.Contains(tag, "json:\"-\"") {
				if !strings.Contains(tag, "omitempty") {
					*issues = append(*issues, createIssue(fset, field.Pos(),
						"PRIVACY_EXPOSED_FIELD",
						"Sensitive field exposed in JSON without omitempty: "+name.Name,
						SeverityMedium))
				}
			}
		}
	}
}

func (a *PrivacyAnalyzer) isSensitiveName(name string) bool {
	nameLower := strings.ToLower(name)

	return apiKeyPattern.MatchString(nameLower) ||
		passwordPattern.MatchString(nameLower) ||
		tokenPattern.MatchString(nameLower) ||
		secretPattern.MatchString(nameLower) ||
		credentialPattern.MatchString(nameLower) ||
		dbPattern.MatchString(nameLower) ||
		awsPattern.MatchString(nameLower) ||
		sshPattern.MatchString(nameLower) ||
		strings.Contains(nameLower, "email") ||
		strings.Contains(nameLower, "phone") ||
		strings.Contains(nameLower, "ssn") ||
		strings.Contains(nameLower, "credit_card") ||
		strings.Contains(nameLower, "card_number")
}

func isLoggingFunction(name string) bool {
	loggingFuncs := []string{
		"Print", "Printf", "Println",
		"Info", "Infof", "Infow",
		"Debug", "Debugf", "Debugw",
		"Warn", "Warnf", "Warnw",
		"Error", "Errorf", "Errorw",
		"Fatal", "Fatalf", "Fatalw",
		"Log", "Logf",
	}

	for _, fn := range loggingFuncs {
		if name == fn {
			return true
		}
	}
	return false
}

func (a *PrivacyAnalyzer) Name() string {
	return "Privacy"
}

// Helper function to create an issue with proper position information
func createIssue(fset *token.FileSet, pos token.Pos, issueType string, message string, severity Severity) Issue {
	return Issue{
		Type:     issueType,
		Message:  message,
		Position: fset.Position(pos),
		Severity: severity,
	}
}

// trackDataFlow tracks variable assignments to identify encrypted data and user input
func (a *PrivacyAnalyzer) trackDataFlow(assign *ast.AssignStmt) {
	// Track assignments from user input functions
	for i, rhs := range assign.Rhs {
		if call, ok := rhs.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				// Check for user input methods
				if isUserInputFunction(sel.Sel.Name) {
					// Mark LHS variables as user input
					if i < len(assign.Lhs) {
						if ident, ok := assign.Lhs[i].(*ast.Ident); ok {
							a.userInputVars[ident.Name] = true
						}
					}
				}
				// Check for encryption functions
				if isEncryptionFunction(sel.Sel.Name) {
					// Mark LHS variables as encrypted
					if i < len(assign.Lhs) {
						if ident, ok := assign.Lhs[i].(*ast.Ident); ok {
							a.encryptedVars[ident.Name] = true
							// Remove from userInputVars if it was there
							delete(a.userInputVars, ident.Name)
						}
					}
				}
			}
		}

		// Track variable-to-variable assignments
		if ident, ok := rhs.(*ast.Ident); ok {
			if a.encryptedVars[ident.Name] && i < len(assign.Lhs) {
				if lhsIdent, ok := assign.Lhs[i].(*ast.Ident); ok {
					a.encryptedVars[lhsIdent.Name] = true
				}
			}
			if a.userInputVars[ident.Name] && i < len(assign.Lhs) {
				if lhsIdent, ok := assign.Lhs[i].(*ast.Ident); ok {
					a.userInputVars[lhsIdent.Name] = true
				}
			}
		}
	}
}

// trackEncryption tracks calls to encryption functions
func (a *PrivacyAnalyzer) trackEncryption(call *ast.CallExpr) {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		// Check for encryption functions
		if isEncryptionFunction(sel.Sel.Name) {
			// Mark the result as encrypted (for assignments this will be handled later)
			// This is mainly to track that encryption is happening

			// If this is part of an assignment, the LHS will be marked as encrypted
			// We handle this in trackDataFlow for assignments
		}
	}
}

// checkDatabaseWrites checks for potentially unencrypted sensitive data in database operations
func (a *PrivacyAnalyzer) checkDatabaseWrites(call *ast.CallExpr, fset *token.FileSet, issues *[]Issue) {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		// Check for database operations
		if isDatabaseOperation(sel.Sel.Name) {
			// Check arguments for sensitive data
			for _, arg := range call.Args {
				a.checkDatabaseArgument(arg, fset, issues)
			}
		}
	}
}

// checkDatabaseArgument checks if a database argument contains unencrypted sensitive data
func (a *PrivacyAnalyzer) checkDatabaseArgument(arg ast.Expr, fset *token.FileSet, issues *[]Issue) {
	switch expr := arg.(type) {
	case *ast.BasicLit:
		// Check for SQL queries with sensitive fields
		if expr.Kind == token.STRING {
			query := strings.ToLower(strings.Trim(expr.Value, `"`))
			if strings.Contains(query, "insert") || strings.Contains(query, "update") {
				// Check for sensitive field names in query
				if containsSensitiveField(query) {
					// This is the SQL query, now check the values being inserted
					// We'll flag this as potentially problematic
				}
			}
		}

	case *ast.Ident:
		// Check if this identifier is sensitive and not encrypted
		if a.isSensitiveName(expr.Name) && !a.encryptedVars[expr.Name] {
			severity := SeverityHigh
			message := "Potentially unencrypted sensitive data in database operation: " + expr.Name

			// If it comes from direct user input, it's definitely bad
			if a.userInputVars[expr.Name] {
				message = "Unencrypted user input being stored in database: " + expr.Name
			} else if strings.Contains(strings.ToLower(expr.Name), "hash") ||
				strings.Contains(strings.ToLower(expr.Name), "encrypted") {
				// If the name suggests it's encrypted, lower the severity
				severity = SeverityLow
				message = "Verify that " + expr.Name + " is properly encrypted before database storage"
			}

			*issues = append(*issues, createIssue(fset, expr.Pos(),
				"PRIVACY_UNENCRYPTED_DB_WRITE",
				message,
				severity))
		}

	case *ast.CallExpr:
		// Check for direct user input functions
		if sel, ok := expr.Fun.(*ast.SelectorExpr); ok {
			if isUserInputFunction(sel.Sel.Name) {
				*issues = append(*issues, createIssue(fset, expr.Pos(),
					"PRIVACY_DIRECT_INPUT_TO_DB",
					"Direct user input to database without encryption: "+sel.Sel.Name,
					SeverityHigh))
			}
		}
	}
}

// Helper function to check if a function is a user input source
func isUserInputFunction(name string) bool {
	userInputFuncs := []string{
		"FormValue", "PostFormValue", "Query", "QueryParam",
		"Param", "GetString", "GetInt", "Get", "PostForm",
		"FormFile", "MultipartForm", "ParseForm", "ParseMultipartForm",
		"Cookie", "Header", "GetHeader", "Body", "GetRawData",
	}

	for _, fn := range userInputFuncs {
		if name == fn {
			return true
		}
	}
	return false
}

// Helper function to check if a function is an encryption function
func isEncryptionFunction(name string) bool {
	encryptionFuncs := []string{
		"GenerateFromPassword", "HashPassword", "Hash",
		"Encrypt", "EncryptString", "Encode",
		"Sum", "Sum256", "Sum512", // crypto hashes
		"New", "NewHash", "Create", // when used with crypto packages
		"CompareHashAndPassword", "CheckPasswordHash",
		"AESEncrypt", "RSAEncrypt", "Sign",
	}

	for _, fn := range encryptionFuncs {
		if strings.Contains(name, fn) {
			return true
		}
	}
	return false
}

// Helper function to check if a function is a database operation
func isDatabaseOperation(name string) bool {
	dbOps := []string{
		"Exec", "ExecContext", "Query", "QueryRow", "QueryRowContext",
		"QueryContext", "Prepare", "PrepareContext",
		"Create", "Save", "Update", "Updates", "Insert", // ORM methods
		"Set", "HSet", "SetNX", "MSet", // Redis
		"InsertOne", "UpdateOne", "ReplaceOne", // MongoDB
	}

	for _, op := range dbOps {
		if name == op {
			return true
		}
	}
	return false
}

// Helper function to check if a SQL query contains sensitive fields
func containsSensitiveField(query string) bool {
	sensitiveFields := []string{
		"password", "passwd", "pwd", "secret", "token",
		"api_key", "apikey", "ssn", "social_security",
		"credit_card", "card_number", "cvv", "pin",
		"private_key", "priv_key", "email", "phone",
		"address", "dob", "date_of_birth", "salary",
	}

	for _, field := range sensitiveFields {
		if strings.Contains(query, field) {
			return true
		}
	}
	return false
}
