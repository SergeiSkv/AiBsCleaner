package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

// CryptoAnalyzer detects crypto and security performance issues
type CryptoAnalyzer struct{}

func NewCryptoAnalyzer() Analyzer {
	return &CryptoAnalyzer{}
}

func (ca *CryptoAnalyzer) Name() string {
	return "CryptoAnalyzer"
}

func (ca *CryptoAnalyzer) Analyze(node interface{}, fset *token.FileSet) []*Issue {
	var issues []*Issue

	astNode, ok := node.(ast.Node)
	if !ok {
		return issues
	}

	// Get filename from the first position we encounter
	filename := ""
	if astNode.Pos().IsValid() {
		filename = fset.Position(astNode.Pos()).Filename
	}

	// Use context helper for proper loop detection
	ctx := NewAnalyzerWithContext(astNode)

	ast.Inspect(
		astNode, func(n ast.Node) bool {
			node, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			pos := fset.Position(node.Pos())

			// Check for math/rand instead of crypto/rand
			if ca.isWeakRandom(node) {
				issues = append(
					issues, &Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       IssueInsecureRandom,
						Severity:   SeverityLevelHigh,
						Message:    "Using math/rand for security-sensitive operations",
						Suggestion: "Use crypto/rand for cryptographic randomness",
						WhyBad: `math/rand is NOT cryptographically secure:
• Predictable with known seed
• Not suitable for tokens, keys, nonces
• Security vulnerability in production
USE: crypto/rand for any security-related randomness`,
					},
				)
			}

			// Check for crypto operations in loops
			loopDepth := ctx.GetNodeLoopDepth(node)
			inLoop := loopDepth > 0

			if inLoop && ca.isCryptoOperation(node) {
				severity := SeverityLevelMedium
				if loopDepth > 1 {
					severity = SeverityLevelHigh
				}

				issues = append(
					issues, &Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       IssueWeakCrypto,
						Severity:   severity,
						Message:    "Heavy crypto operation inside loop",
						Suggestion: "Batch crypto operations or use streaming cipher modes",
						WhyBad: `Crypto in loops is expensive:
• Hash functions: ~100-1000ns per call
• Encryption: ~1-10μs per block
• Key derivation: ~1-100ms per call
• In nested loops: multiplied overhead
BETTER: Use HMAC for repeated hashing, streaming modes for encryption`,
					},
				)
			}

			// Check for key derivation in hot paths
			if ca.isKeyDerivation(node) {
				if inLoop {
					issues = append(
						issues, &Issue{
							File:       filename,
							Line:       pos.Line,
							Column:     pos.Column,
							Position:   pos,
							Type:       IssueWeakCrypto,
							Severity:   SeverityLevelHigh,
							Message:    "Key derivation (PBKDF2/scrypt/bcrypt) in loop - extremely expensive",
							Suggestion: "Derive keys once and cache them",
							WhyBad: `Key derivation is intentionally slow:
• PBKDF2: ~10-100ms per key
• scrypt: ~100ms-1s per key
• bcrypt: ~50-500ms per hash
IMPACT: Can freeze application in loops`,
						},
					)
				}
			}

			// Check for MD5/SHA1 usage
			if ca.isWeakHash(node) {
				issues = append(
					issues, &Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       IssueWeakHash,
						Severity:   SeverityLevelMedium,
						Message:    "Using weak hash algorithm (MD5/SHA1)",
						Suggestion: "Use SHA256 or SHA3 for security, xxhash/fnv for non-crypto hashing",
						WhyBad: `Weak hash problems:
• MD5/SHA1 are cryptographically broken
• Still slow for non-crypto use (~50-200ns)
• For checksums: use xxhash (5-10ns) or CRC32
• For security: use SHA256/SHA3`,
					},
				)
			}

			// Check for cipher creation in loops
			if inLoop && ca.isCipherCreation(node) {
				issues = append(
					issues, &Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       IssueWeakCrypto,
						Severity:   SeverityLevelMedium,
						Message:    "Creating cipher in loop - reuse instead",
						Suggestion: "Create cipher once and reuse for multiple operations",
						WhyBad: `Cipher creation overhead:
• AES NewCipher: ~500ns-1μs
• Includes key schedule computation
• Memory allocation for cipher state
BETTER: Create once, use many times`,
					},
				)
			}

			// Check for missing HMAC for repeated hashing
			if ca.isRepeatedHashing(node) {
				issues = append(
					issues, &Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       IssueWeakCrypto,
						Severity:   SeverityLevelLow,
						Message:    "Multiple hash operations on similar data",
						Suggestion: "Use HMAC for keyed hashing or hash.Hash interface for streaming",
					},
				)
			}

			// Check for large RSA key sizes
			if ca.hasLargeRSAKey(node) {
				issues = append(
					issues, &Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       IssueWeakCrypto,
						Severity:   SeverityLevelMedium,
						Message:    "RSA key >2048 bits has exponential performance cost",
						Suggestion: "Use 2048-bit RSA or switch to ECDSA for better performance",
						WhyBad: `RSA performance degrades with key size:
• 1024-bit: ~1ms (insecure)
• 2048-bit: ~8ms (standard)
• 4096-bit: ~64ms (8x slower)
• ECDSA P-256: ~0.2ms (40x faster than RSA-2048)`,
					},
				)
			}

			// Check for sync random generation
			if ca.isSyncRandomRead(node) {
				issues = append(
					issues, &Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       IssueWeakCrypto,
						Severity:   SeverityLevelLow,
						Message:    "crypto/rand.Read can block on entropy",
						Suggestion: "Consider buffering random data for high-throughput scenarios",
					},
				)
			}

			// Check for constant time comparison
			if ca.needsConstantTimeComparison(node) {
				issues = append(
					issues, &Issue{
						File:       filename,
						Line:       pos.Line,
						Column:     pos.Column,
						Position:   pos,
						Type:       IssueWeakCrypto,
						Severity:   SeverityLevelHigh,
						Message:    "Non-constant time comparison of secrets",
						Suggestion: "Use subtle.ConstantTimeCompare for secret comparison",
						WhyBad: `Regular comparison leaks timing information:
• Returns early on first difference
• Attackers can guess secrets byte-by-byte
• Use crypto/subtle for constant-time operations`,
					},
				)
			}
			return true
		},
	)

	return issues
}

func (ca *CryptoAnalyzer) isWeakRandom(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != pkgRand {
		return false
	}

	funcName := sel.Sel.Name
	randomFuncs := []string{"Int", "Intn", "Float64", "Read", "Uint32", "Uint64"}
	for _, rf := range randomFuncs {
		if funcName == rf {
			return true
		}
	}
	return false
}

func (ca *CryptoAnalyzer) isCryptoOperation(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if ok {
		funcName := sel.Sel.Name
		cryptoOps := []string{
			"New", "Sum256", methodSum, "Write", "Encrypt", "Decrypt",
			"Sign", "Verify", "Seal", "Open",
		}

		for _, op := range cryptoOps {
			if funcName != op && !strings.Contains(funcName, op) {
				continue
			}

			// Check if it's from a crypto package
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}

			cryptoPkgs := []string{"sha256", "sha512", "sha1", "md5", "aes", "des", "rsa", "ecdsa"}
			for _, pkg := range cryptoPkgs {
				if ident.Name == pkg {
					return true
				}
			}
			return true
		}
	}

	// Also check for package function calls like sha256.Sum256
	if ident, ok := call.Fun.(*ast.Ident); ok {
		if strings.Contains(ident.Name, methodSum) || strings.Contains(ident.Name, "Hash") {
			return true
		}
	}

	return false
}

func (ca *CryptoAnalyzer) isKeyDerivation(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	funcName := sel.Sel.Name
	// Check for specific KDF functions
	if funcName != "Key" && funcName != "GenerateFromPassword" &&
		funcName != "CompareHashAndPassword" && funcName != "Cost" {
		return false
	}

	// Check if it's from a KDF package
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	kdfPkgs := []string{"pbkdf2", "scrypt", "bcrypt", "argon2"}
	for _, pkg := range kdfPkgs {
		if ident.Name == pkg {
			return true
		}
	}
	return false
}

func (ca *CryptoAnalyzer) isWeakHash(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	// Check direct weak hash usage
	weakHashes := []string{"md5", "sha1"}
	for _, weak := range weakHashes {
		if strings.EqualFold(ident.Name, weak) {
			return true
		}
	}

	// Check for md5.New() or sha1.New()
	funcName := sel.Sel.Name
	if funcName == "New" || funcName == methodSum {
		if ident.Name == "md5" || ident.Name == "sha1" {
			return true
		}
	}

	return false
}

func (ca *CryptoAnalyzer) isCipherCreation(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	funcName := sel.Sel.Name
	if !strings.Contains(funcName, "NewCipher") && funcName != "NewCipher" &&
		funcName != "NewGCM" && funcName != "NewCTR" {
		return false
	}

	// Check it's from aes or another cipher package
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	cipherPkgs := []string{"aes", "des", "blowfish", "cipher"}
	for _, pkg := range cipherPkgs {
		if ident.Name == pkg {
			return true
		}
	}
	return false
}

func (ca *CryptoAnalyzer) isRepeatedHashing(call *ast.CallExpr) bool {
	// Would need data flow analysis to detect properly
	// Check for hash.Sum in loops as a simple heuristic
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		return sel.Sel.Name == methodSum || sel.Sel.Name == "Write"
	}
	return false
}

func (ca *CryptoAnalyzer) hasLargeRSAKey(call *ast.CallExpr) bool {
	// Check for RSA key generation with large sizes
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "GenerateKey" {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != "rsa" {
		return false
	}

	// Check if the second argument (bits) is > 2048
	if len(call.Args) < 2 {
		return false
	}

	lit, ok := call.Args[1].(*ast.BasicLit)
	if !ok {
		return false
	}

	// Simple check - would need constant evaluation for accuracy
	return strings.Contains(lit.Value, "4096") || strings.Contains(lit.Value, "8192")
}

func (ca *CryptoAnalyzer) isSyncRandomRead(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}

	// Check for crypto/rand.Read
	// Would need import analysis to confirm its crypto/rand
	return ident.Name == "rand" && sel.Sel.Name == "Read"
}

func (ca *CryptoAnalyzer) needsConstantTimeComparison(node *ast.CallExpr) bool {
	// Check for == comparison of potential secrets
	// This is simplified - would need data flow analysis
	if ident, ok := node.Fun.(*ast.Ident); ok {
		if ident.Name == "bytes.Equal" || ident.Name == "hmac.Equal" {
			return false // These are already constant-time
		}
	}
	return false
}
