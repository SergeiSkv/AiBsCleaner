package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/SergeiSkv/AiBsCleaner/models"
)

type CryptoAnalyzer struct{}

func NewCryptoAnalyzer() Analyzer {
	return &CryptoAnalyzer{}
}

func (ca *CryptoAnalyzer) Name() string {
	return "Crypto Performance"
}

func (ca *CryptoAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*models.Issue {
	file, ok := node.(*ast.File)
	if !ok {
		return nil
	}

	filename := ""
	if file.Pos().IsValid() {
		filename = fset.Position(file.Pos()).Filename
	}

	issues := make([]*models.Issue, 0, 4)

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		issue := ca.inspectCall(call, fset, filename)
		if issue != nil {
			issues = append(issues, issue)
		}
		return true
	})

	return issues
}

func (ca *CryptoAnalyzer) inspectCall(call *ast.CallExpr, fset *token.FileSet, filename string) *models.Issue {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil
	}

	switch pkgIdent.Name {
	case "rand":
		if sel.Sel.Name == "Read" || sel.Sel.Name == "Int" || sel.Sel.Name == "Intn" || sel.Sel.Name == "Perm" {
			pos := fset.Position(call.Pos())
			return &models.Issue{
				File:       filename,
				Line:       pos.Line,
				Column:     pos.Column,
				Position:   pos,
				Type:       models.IssueInsecureRandom,
				Severity:   models.SeverityLevelHigh,
				Message:    "Using math/rand for randomness; crypto/rand required for security-sensitive values",
				Suggestion: "Replace with crypto/rand when generating keys, tokens, or secrets",
			}
		}
	case "md5", "sha1":
		pos := fset.Position(call.Pos())
		return &models.Issue{
			File:       filename,
			Line:       pos.Line,
			Column:     pos.Column,
			Position:   pos,
			Type:       models.IssueWeakHash,
			Severity:   models.SeverityLevelMedium,
			Message:    "Weak hash algorithm in use (MD5/SHA1)",
			Suggestion: "Use SHA-256/SHA-512 for security or a modern non-crypto hash for checksums",
		}
	}

	return nil
}
