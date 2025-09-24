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
				issues = append(issues, a.checkGenDecl(node, fset, isTestFile)...)
			case *ast.AssignStmt:
				issues = append(issues, a.checkAssignment(node, fset, isTestFile)...)
				a.trackDataFlow(node) // Track data flow for encryption detection
			case *ast.BasicLit:
				issues = append(issues, a.checkLiteral(node, fset, isTestFile)...)
			case *ast.CallExpr:
				issues = append(issues, a.checkFunctionCall(node, fset)...)
				a.trackEncryption(node)                                       // Track encryption function calls
				issues = append(issues, a.checkDatabaseWrites(node, fset)...) // Check for unencrypted DB writes
			case *ast.Field:
				issues = append(issues, a.checkStructField(node, fset)...)
			}
			return true
		},
	)

	return issues
}

func (a *PrivacyAnalyzer) checkGenDecl(decl *ast.GenDecl, fset *token.FileSet, isTestFile bool) []Issue {
	var issues []Issue
	if decl.Tok != token.CONST && decl.Tok != token.VAR {
		return nil
	}

	for _, spec := range decl.Specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}

		for i, name := range valueSpec.Names {
			nameStr := name.Name

			// Skip if not sensitive
			if !a.isSensitiveName(nameStr) {
				continue
			}

			// Skip if no value
			if i >= len(valueSpec.Values) {
				continue
			}

			lit, ok := valueSpec.Values[i].(*ast.BasicLit)
			if !ok {
				continue
			}

			// Skip if not string or empty
			if lit.Kind != token.STRING || lit.Value == `""` || lit.Value == `"` {
				continue
			}

			value := strings.Trim(lit.Value, `"`)
			// Skip if template variable
			if strings.HasPrefix(value, "${") || strings.HasPrefix(value, "{{") {
				continue
			}

			var severity = SeverityHigh
			if isTestFile {
				severity = SeverityLow
			}
			issues = append(
				issues, createIssue(
					fset, name.Pos(),
					"PRIVACY_HARDCODED_SECRET",
					"Hardcoded sensitive value in variable: "+nameStr,
					severity,
				),
			)
		}
	}
	return issues
}

func (a *PrivacyAnalyzer) checkAssignment(assign *ast.AssignStmt, fset *token.FileSet, isTestFile bool) []Issue {
	var issues []Issue
	for i, lhs := range assign.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}

		if !a.isSensitiveName(ident.Name) {
			continue
		}

		if i >= len(assign.Rhs) {
			continue
		}

		lit, ok := assign.Rhs[i].(*ast.BasicLit)
		if !ok {
			continue
		}

		if lit.Kind != token.STRING || lit.Value == `""` {
			continue
		}

		value := strings.Trim(lit.Value, `"`)
		if strings.HasPrefix(value, "${") || strings.HasPrefix(value, "{{") || len(value) == 0 {
			continue
		}

		var severity Severity = SeverityHigh
		if isTestFile {
			severity = SeverityLow
		}
		issues = append(
			issues, createIssue(
				fset, ident.Pos(),
				"PRIVACY_HARDCODED_SECRET",
				"Hardcoded sensitive value assigned to: "+ident.Name,
				severity,
			),
		)
	}
	return issues
}

func (a *PrivacyAnalyzer) checkLiteral(lit *ast.BasicLit, fset *token.FileSet, isTestFile bool) []Issue {
	var issues []Issue
	if lit.Kind != token.STRING {
		return nil
	}

	value := strings.Trim(lit.Value, `"`)
	if len(value) < 10 {
		return nil
	}

	// Skip example/placeholder values
	if strings.Contains(value, "example") || strings.Contains(value, "your-") ||
		strings.Contains(value, "xxx") || strings.Contains(value, "...") {
		return nil
	}

	// Check for hardcoded secrets
	if awsKeyPattern.MatchString(value) {
		issues = append(
			issues, createIssue(
				fset, lit.Pos(),
				"PRIVACY_AWS_KEY",
				"Potential AWS access key found in code",
				SeverityHigh,
			),
		)
	}

	if hardcodedJWT.MatchString(value) {
		var severity Severity = SeverityHigh
		if isTestFile {
			severity = SeverityMedium
		}
		issues = append(
			issues, createIssue(
				fset, lit.Pos(),
				"PRIVACY_JWT_TOKEN",
				"Hardcoded JWT token found",
				severity,
			),
		)
	}

	// Check for PII in non-test files
	if !isTestFile {
		if emailPattern.MatchString(value) && !strings.Contains(value, "@example.") {
			issues = append(
				issues, createIssue(
					fset, lit.Pos(),
					"PRIVACY_EMAIL_PII",
					"Email address found in code (potential PII)",
					SeverityMedium,
				),
			)
		}

		if ssnPattern.MatchString(value) {
			issues = append(
				issues, createIssue(
					fset, lit.Pos(),
					"PRIVACY_SSN_PII",
					"SSN pattern found in code (potential PII)",
					SeverityHigh,
				),
			)
		}

		if ccPattern.MatchString(value) && !strings.Contains(value, "0000") {
			issues = append(
				issues, createIssue(
					fset, lit.Pos(),
					"PRIVACY_CREDIT_CARD_PII",
					"Credit card pattern found in code (potential PII)",
					SeverityHigh,
				),
			)
		}
	}
	return issues
}

func (a *PrivacyAnalyzer) checkFunctionCall(call *ast.CallExpr, fset *token.FileSet) []Issue {
	var issues []Issue

	fun, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return issues
	}

	funcName := fun.Sel.Name

	// Check for logging sensitive data
	if isLoggingFunction(funcName) {
		for _, arg := range call.Args {
			ident, ok := arg.(*ast.Ident)
			if !ok {
				continue
			}
			if !a.isSensitiveName(ident.Name) {
				continue
			}
			issues = append(
				issues, createIssue(
					fset, call.Pos(),
					"PRIVACY_LOGGING_SENSITIVE",
					"Logging potentially sensitive data: "+ident.Name,
					SeverityMedium,
				),
			)
		}
	}

	// Check fmt.Printf/Sprintf for sensitive data
	ident, ok := fun.X.(*ast.Ident)
	if ok && ident.Name == "fmt" {
		if funcName == "Printf" || funcName == "Sprintf" || funcName == "Fprintf" {
			for _, arg := range call.Args[1:] { // Skip format string
				argIdent, ok := arg.(*ast.Ident)
				if !ok {
					continue
				}
				if !a.isSensitiveName(argIdent.Name) {
					continue
				}
				issues = append(
					issues, createIssue(
						fset, call.Pos(),
						"PRIVACY_PRINTING_SENSITIVE",
						"Printing potentially sensitive data: "+argIdent.Name,
						SeverityMedium,
					),
				)
			}
		}
	}
	return issues
}

func (a *PrivacyAnalyzer) checkStructField(field *ast.Field, fset *token.FileSet) []Issue {
	var issues []Issue
	if field.Tag == nil {
		return nil
	}

	tag := strings.Trim(field.Tag.Value, "`")

	// Check for sensitive fields without proper tags
	for _, name := range field.Names {
		if a.isSensitiveName(name.Name) {
			// Check if field is exposed in JSON without omitempty or -
			if strings.Contains(tag, "json:") && !strings.Contains(tag, "json:\"-\"") {
				if !strings.Contains(tag, "omitempty") {
					issues = append(
						issues, createIssue(
							fset, field.Pos(),
							"PRIVACY_EXPOSED_FIELD",
							"Sensitive field exposed in JSON without omitempty: "+name.Name,
							SeverityMedium,
						),
					)
				}
			}
		}
	}
	return issues
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
		call, ok := rhs.(*ast.CallExpr)
		if !ok {
			// Track variable-to-variable assignments
			ident, ok := rhs.(*ast.Ident)
			if !ok || i >= len(assign.Lhs) {
				continue
			}

			lhsIdent, ok := assign.Lhs[i].(*ast.Ident)
			if !ok {
				continue
			}

			if a.encryptedVars[ident.Name] {
				a.encryptedVars[lhsIdent.Name] = true
			}
			if a.userInputVars[ident.Name] {
				a.userInputVars[lhsIdent.Name] = true
			}
			continue
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}

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
func (a *PrivacyAnalyzer) checkDatabaseWrites(call *ast.CallExpr, fset *token.FileSet) []Issue {
	var issues []Issue
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		// Check for database operations
		if isDatabaseOperation(sel.Sel.Name) {
			// Check arguments for sensitive data
			for _, arg := range call.Args {
				issues = append(issues, a.checkDatabaseArgument(arg, fset)...)
			}
		}
	}
	return issues
}

// checkDatabaseArgument checks if a database argument contains unencrypted sensitive data
func (a *PrivacyAnalyzer) checkDatabaseArgument(arg ast.Expr, fset *token.FileSet) []Issue {
	var issues []Issue
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

			issues = append(
				issues, createIssue(
					fset, expr.Pos(),
					"PRIVACY_UNENCRYPTED_DB_WRITE",
					message,
					severity,
				),
			)
		}

	case *ast.CallExpr:
		// Check for direct user input functions
		if sel, ok := expr.Fun.(*ast.SelectorExpr); ok {
			if isUserInputFunction(sel.Sel.Name) {
				issues = append(
					issues, createIssue(
						fset, expr.Pos(),
						"PRIVACY_DIRECT_INPUT_TO_DB",
						"Direct user input to database without encryption: "+sel.Sel.Name,
						SeverityHigh,
					),
				)
			}
		}
	}
	return issues
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
