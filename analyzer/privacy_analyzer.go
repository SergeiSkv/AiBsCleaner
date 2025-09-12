package analyzer

import (
	"go/ast"
	"go/token"
	"math"
	"regexp"
	"strings"
	"unicode"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type PrivacyAnalyzer struct{}

func NewPrivacyAnalyzer() Analyzer {
	return &PrivacyAnalyzer{}
}

func (p *PrivacyAnalyzer) Name() string {
	return "PrivacyAnalyzer"
}

var (
	secretNamePattern = regexp.MustCompile(`(?i)(secret|password|passwd|token|apikey|api_key|credential)`)
	jwtPattern        = regexp.MustCompile(`^eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]*$`)
	awsKeyPattern     = regexp.MustCompile(`^AKIA[0-9A-Z]{16}$`)

	// Common non-secret values (key names, placeholders, etc.)
	knownNonSecrets = map[string]bool{
		"user":           true,
		"admin":          true,
		"guest":          true,
		"default":        true,
		"test":           true,
		"demo":           true,
		"example":        true,
		"authorization":  true,
		"authentication": true,
		"bearer":         true,
		"basic":          true,
		"proxy_user":     true,
		"username":       true,
		"user_id":        true,
		"session":        true,
		"csrf":           true,
		"xsrf":           true,
	}
)

func (p *PrivacyAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	issues := make([]*models.Issue, 0, 8)
	flagged := make(map[token.Pos]struct{}, 8)

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ValueSpec:
			issues = append(issues, p.inspectValueSpec(node, fset, filename, flagged)...)
		case *ast.AssignStmt:
			issues = append(issues, p.inspectAssign(node, fset, filename, flagged)...)
		case *ast.BasicLit:
			issues = append(issues, p.inspectLiteral(node, fset, filename, flagged)...)
		}
		return true
	})

	return issues
}

func (p *PrivacyAnalyzer) inspectValueSpec(
	spec *ast.ValueSpec, fset *token.FileSet, filename string, flagged map[token.Pos]struct{},
) []*models.Issue {
	issues := make([]*models.Issue, 0, len(spec.Names))
	for i, name := range spec.Names {
		if name == nil {
			continue
		}
		if !secretNamePattern.MatchString(name.Name) {
			continue
		}
		if i >= len(spec.Values) {
			continue
		}
		if lit, ok := spec.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if issue := p.newSecretIssue(lit, name.Name, fset, filename); issue != nil {
				flagged[lit.Pos()] = struct{}{}
				issues = append(issues, issue)
			}
		}
	}
	return issues
}

func (p *PrivacyAnalyzer) inspectAssign(
	assign *ast.AssignStmt, fset *token.FileSet, filename string, flagged map[token.Pos]struct{},
) []*models.Issue {
	issues := make([]*models.Issue, 0, len(assign.Lhs))
	for i, lhs := range assign.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok || !secretNamePattern.MatchString(ident.Name) {
			continue
		}
		if i >= len(assign.Rhs) {
			continue
		}
		if lit, ok := assign.Rhs[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if issue := p.newSecretIssue(lit, ident.Name, fset, filename); issue != nil {
				flagged[lit.Pos()] = struct{}{}
				issues = append(issues, issue)
			}
		}
	}
	return issues
}

func (p *PrivacyAnalyzer) inspectLiteral(
	lit *ast.BasicLit, fset *token.FileSet, filename string, flagged map[token.Pos]struct{},
) []*models.Issue {
	if lit.Kind != token.STRING {
		return nil
	}
	if _, exists := flagged[lit.Pos()]; exists {
		return nil
	}
	value := strings.Trim(lit.Value, `"`)
	if value == "" {
		return nil
	}

	if jwtPattern.MatchString(value) || awsKeyPattern.MatchString(value) {
		pos := fset.Position(lit.Pos())
		return []*models.Issue{
			{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       models.IssuePrivacyHardcodedSecret,
				Severity:   models.SeverityLevelHigh,
				Message:    "Hardcoded credential literal detected",
				Suggestion: "Move secrets to configuration or secret manager",
			},
		}
	}
	return nil
}

func (p *PrivacyAnalyzer) newSecretIssue(lit *ast.BasicLit, name string, fset *token.FileSet, filename string) *models.Issue {
	value := strings.Trim(lit.Value, `"`)
	if value == "" || strings.HasPrefix(value, "${") || strings.HasPrefix(value, "{{") {
		return nil
	}

	// Filter out false positives
	if !isLikelySecret(value, name) {
		return nil
	}

	pos := fset.Position(lit.Pos())
	return &models.Issue{
		File:       filename,
		Line:       pos.Line,
		Column:     pos.Column,
		Position:   pos,
		Type:       models.IssuePrivacyHardcodedSecret,
		Severity:   models.SeverityLevelHigh,
		Message:    "Hardcoded secret assigned to " + name,
		Suggestion: "Load secrets from environment variables or a secrets manager",
	}
}

// isLikelySecret determines if a value is likely a real secret
func isLikelySecret(value, varName string) bool {
	lowerValue := strings.ToLower(value)
	lowerVarName := strings.ToLower(varName)

	// 1. Check known non-secrets (key names, common values)
	if knownNonSecrets[lowerValue] {
		return false
	}

	// 2. Too short to be a real secret (< 8 chars)
	if len(value) < 8 {
		return false
	}

	// 3. If value equals variable name or contains it, it's likely a key name
	// e.g., AuthUserKey = "user" or sessionKey = "session_key"
	if strings.Contains(lowerValue, strings.TrimSuffix(lowerVarName, "key")) {
		return false
	}
	if strings.Contains(lowerValue, strings.ReplaceAll(lowerVarName, "_", "")) {
		return false
	}

	// 4. Check if it's just a simple word (no numbers/special chars = likely a key name)
	if isSimpleWord(value) {
		return false
	}

	// 5. Check entropy - real secrets have higher entropy
	if calculateEntropy(value) < 3.0 {
		return false
	}

	// 6. Contains only lowercase letters = likely a key name
	if isOnlyLowercase(value) && !strings.ContainsAny(value, "0123456789") {
		return false
	}

	return true
}

// isSimpleWord checks if string is just a simple word (letters only, no special chars)
func isSimpleWord(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

// isOnlyLowercase checks if string contains only lowercase letters and underscores
func isOnlyLowercase(s string) bool {
	for _, r := range s {
		if !unicode.IsLower(r) && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

// calculateEntropy calculates Shannon entropy of a string
func calculateEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}

	freq := make(map[rune]int)
	for _, r := range s {
		freq[r]++
	}

	var entropy float64
	length := float64(len(s))
	for _, count := range freq {
		p := float64(count) / length
		entropy -= p * math.Log2(p)
	}

	return entropy
}
