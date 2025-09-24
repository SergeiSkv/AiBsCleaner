package analyzer

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type DependencyAnalyzer struct {
	projectPath string
	vulnChecker *VulnerabilityChecker
}

func NewDependencyAnalyzer(projectPath string) *DependencyAnalyzer {
	// Initialize with in-memory cache for now
	// In production, use Redis: NewRedisCache("redis://localhost:6379")
	cache := NewRedisCache("")

	return &DependencyAnalyzer{
		projectPath: projectPath,
		vulnChecker: NewVulnerabilityChecker(cache),
	}
}

type Module struct {
	Path     string
	Version  string
	Time     time.Time
	Indirect bool
	Replace  *Module
}

type ModuleInfo struct {
	Module Module
	Deps   []Module
}

var (
	// Deprecated packages
	deprecatedPackages = map[string]string{
		"github.com/golang/protobuf":         "Use google.golang.org/protobuf instead",
		"github.com/dgrijalva/jwt-go":        "Has security vulnerabilities, use github.com/golang-jwt/jwt instead",
		"gopkg.in/mgo.v2":                    "Unmaintained, use go.mongodb.org/mongo-driver instead",
		"github.com/go-kit/kit":              "Use github.com/go-kit/kit/v2 instead",
		"github.com/gorilla/websocket":       "Consider nhooyr.io/websocket for better performance",
		"github.com/satori/go.uuid":          "Use github.com/google/uuid instead",
		"github.com/pborman/uuid":            "Use github.com/google/uuid instead",
		"github.com/juju/errors":             "Consider using standard errors with Go 1.13+",
		"github.com/pkg/errors":              "Consider using standard errors with Go 1.13+",
		"golang.org/x/net/context":           "Use context from standard library instead",
		"golang.org/x/tools/go/ast/astutil":  "Use golang.org/x/tools/go/ast/astutil from newer version",
		"github.com/Sirupsen/logrus":         "Use github.com/sirupsen/logrus (lowercase) instead",
		"github.com/garyburd/redigo":         "Use github.com/gomodule/redigo instead",
		"github.com/denisenkom/go-mssqldb":   "Use github.com/microsoft/go-mssqldb instead",
		"github.com/globalsign/mgo":          "Unmaintained, use go.mongodb.org/mongo-driver instead",
		"github.com/go-sql-driver/mysql":     "Consider using database/sql with context support",
		"github.com/kisielk/sqlstruct":       "Unmaintained, consider alternatives",
		"github.com/jmoiron/sqlx":            "Check for updates, may have newer version",
		"github.com/jinzhu/gorm":             "Use gorm.io/gorm (v2) instead",
		"github.com/russross/blackfriday":    "Use github.com/russross/blackfriday/v2 instead",
		"github.com/microcosm-cc/bluemonday": "Check for updates for security fixes",
	}

	// Known vulnerable versions
	vulnerablePackages = map[string][]string{
		"github.com/dgrijalva/jwt-go":         {"< 4.0.0"},
		"github.com/gin-gonic/gin":            {"< 1.7.7"},  // CVE-2020-28483
		"github.com/beego/beego":              {"< 1.12.2"}, // Multiple CVEs
		"github.com/labstack/echo":            {"< 4.2.0"},  // CVE-2020-26883
		"github.com/ethereum/go-ethereum":     {"< 1.10.8"}, // Multiple security fixes
		"gopkg.in/yaml.v2":                    {"< 2.2.8"},  // CVE-2019-11254
		"gopkg.in/yaml.v3":                    {"< 3.0.0"},
		"github.com/tidwall/gjson":            {"< 1.6.5"},                // CVE-2020-36476
		"github.com/opencontainers/runc":      {"< 1.0.2"},                // Multiple CVEs
		"github.com/docker/docker":            {"< 20.10.9"},              // Multiple security fixes
		"golang.org/x/crypto/ssh":             {"< 0.0.0-20211202192323"}, // CVE-2021-43565
		"golang.org/x/text":                   {"< 0.3.7"},                // CVE-2021-38561
		"github.com/hashicorp/consul":         {"< 1.9.9"},                // Multiple CVEs
		"github.com/prometheus/client_golang": {"< 1.11.1"},               // CVE-2022-21698
	}

	// Problematic licenses
	problematicLicenses = map[string]string{
		"AGPL-3.0":       "Copyleft license, requires source disclosure",
		"GPL-2.0":        "Copyleft license, may affect your code license",
		"GPL-3.0":        "Copyleft license, may affect your code license",
		"LGPL-2.1":       "Weak copyleft, has linking requirements",
		"LGPL-3.0":       "Weak copyleft, has linking requirements",
		"SSPL":           "Not OSI approved, restrictive for SaaS",
		"Commons Clause": "Restricts commercial use",
		"Elastic-2.0":    "Restricts certain commercial uses",
		"BSL-1.1":        "Business Source License, becomes open after time",
	}

	// License patterns in files
	licensePatterns = map[string]*regexp.Regexp{
		"MIT":        regexp.MustCompile(`(?i)MIT License|Permission is hereby granted, free of charge`),
		"Apache-2.0": regexp.MustCompile(`(?i)Apache License|Version 2\.0`),
		"BSD-3":      regexp.MustCompile(`(?i)BSD 3-Clause|Redistribution and use in source and binary forms`),
		"BSD-2":      regexp.MustCompile(`(?i)BSD 2-Clause|Redistributions of source code must retain`),
		"GPL-3.0":    regexp.MustCompile(`(?i)GNU GENERAL PUBLIC LICENSE|Version 3`),
		"GPL-2.0":    regexp.MustCompile(`(?i)GNU GENERAL PUBLIC LICENSE|Version 2`),
		"AGPL-3.0":   regexp.MustCompile(`(?i)GNU AFFERO GENERAL PUBLIC LICENSE`),
		"LGPL":       regexp.MustCompile(`(?i)GNU LESSER GENERAL PUBLIC LICENSE`),
		"MPL-2.0":    regexp.MustCompile(`(?i)Mozilla Public License|Version 2\.0`),
		"ISC":        regexp.MustCompile(`(?i)ISC License|Permission to use, copy, modify`),
	}
)

func (a *DependencyAnalyzer) Analyze(filename string, node interface{}, fset *token.FileSet) []Issue {
	var issues []Issue

	file, ok := node.(*ast.File)
	if !ok {
		return issues
	}

	// Check imports in the file
	for _, imp := range file.Imports {
		if imp.Path != nil {
			importPath := strings.Trim(imp.Path.Value, `"`)
			issues = append(issues, a.checkImport(importPath, fset.Position(imp.Pos()))...)
		}
	}

	// Check go.mod file once per project
	if a.shouldCheckGoMod(filename) {
		issues = append(issues, a.checkGoMod()...)
		issues = append(issues, a.checkGoSum()...)
	}

	return issues
}

func (a *DependencyAnalyzer) checkImport(importPath string, pos token.Position) []Issue {
	var issues []Issue

	// Check for deprecated packages
	if msg, deprecated := deprecatedPackages[importPath]; deprecated {
		issues = append(
			issues, Issue{
				Type:     "DEPENDENCY_DEPRECATED",
				Message:  "Deprecated package: " + importPath + ". " + msg,
				Position: pos,
				Severity: SeverityMedium,
			},
		)
	}

	// Check for C imports (cgo)
	if importPath == "C" {
		issues = append(
			issues, Issue{
				Type:     "DEPENDENCY_CGO",
				Message:  "Using cgo can cause portability and security issues",
				Position: pos,
				Severity: SeverityLow,
			},
		)
	}

	// Check for unsafe package
	if importPath == "unsafe" {
		issues = append(
			issues, Issue{
				Type:     "DEPENDENCY_UNSAFE",
				Message:  "Using unsafe package bypasses Go's type safety",
				Position: pos,
				Severity: SeverityMedium,
			},
		)
	}

	// Check for internal packages from other modules
	if strings.Contains(importPath, "/internal/") && !strings.HasPrefix(importPath, a.getModulePath()) {
		issues = append(
			issues, Issue{
				Type:     "DEPENDENCY_INTERNAL",
				Message:  "Importing internal package from another module: " + importPath,
				Position: pos,
				Severity: SeverityHigh,
			},
		)
	}

	return issues
}

func (a *DependencyAnalyzer) checkGoMod() []Issue {
	var issues []Issue
	// Pre-allocate capacity to reduce allocations
	issues = make([]Issue, 0, 100)
	goModPath := filepath.Join(a.projectPath, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return issues
	}

	file, err := os.Open(goModPath)
	if err != nil {
		return issues
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	inRequire := false
	inReplace := false
	var messageBuilder strings.Builder // Pre-allocate builder for reuse

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "require (" {
			inRequire = true
			continue
		}
		if line == "replace (" {
			inReplace = true
			continue
		}
		if line == ")" {
			inRequire = false
			inReplace = false
			continue
		}

		if inRequire && line != "" && !strings.HasPrefix(line, "//") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				pkg := parts[0]
				version := parts[1]

				// Check deprecated packages
				if msg, deprecated := deprecatedPackages[pkg]; deprecated {
					issues = append(
						issues, Issue{
							Type:     "DEPENDENCY_DEPRECATED",
							Message:  "Deprecated dependency in go.mod: " + pkg + ". " + msg,
							Position: token.Position{Filename: goModPath, Line: lineNum},
							Severity: SeverityMedium,
						},
					)
				}

				// Check for vulnerabilities - extracted to avoid nested loop allocation
				vulnIssues := a.checkPackageVulnerabilities(pkg, version, goModPath, lineNum, &messageBuilder)
				issues = append(issues, vulnIssues...)

				// Check for indirect dependencies
				if strings.Contains(line, "// indirect") {
					if strings.Contains(pkg, "github.com") || strings.Contains(pkg, "golang.org/x") {
						// Only warn about significant indirect dependencies
						issues = append(
							issues, Issue{
								Type:     "DEPENDENCY_INDIRECT",
								Message:  "Large number of indirect dependencies may indicate dependency bloat: " + pkg,
								Position: token.Position{Filename: goModPath, Line: lineNum},
								Severity: SeverityLow,
							},
						)
					}
				}

				// Check for old versions
				if a.isOldVersion(version) {
					issues = append(
						issues, Issue{
							Type:     "DEPENDENCY_OUTDATED",
							Message:  "Potentially outdated dependency: " + pkg + " " + version,
							Position: token.Position{Filename: goModPath, Line: lineNum},
							Severity: SeverityLow,
						},
					)
				}
			}
		}

		if inReplace && line != "" && !strings.HasPrefix(line, "//") {
			// Check for local replacements
			if strings.Contains(line, "=>") && (strings.Contains(line, "../") || strings.Contains(line, "./")) {
				issues = append(
					issues, Issue{
						Type:     "DEPENDENCY_LOCAL_REPLACE",
						Message:  "Local replace directive found - may cause issues in CI/CD: " + line,
						Position: token.Position{Filename: goModPath, Line: lineNum},
						Severity: SeverityMedium,
					},
				)
			}
		}
	}

	return issues
}

func (a *DependencyAnalyzer) checkGoSum() []Issue {
	var issues []Issue
	goSumPath := filepath.Join(a.projectPath, "go.sum")
	if _, err := os.Stat(goSumPath); os.IsNotExist(err) {
		// Use go.mod path for better context
		goModPath := filepath.Join(a.projectPath, "go.mod")
		issues = append(
			issues, Issue{
				Type:    "DEPENDENCY_NO_CHECKSUM",
				Message: "go.sum file missing - dependencies are not locked",
				Position: token.Position{
					Filename: goModPath,
					Line:     1,
					Column:   1,
				},
				Severity: SeverityHigh,
			},
		)
		return issues
	}

	// Check if go.sum is empty
	info, err := os.Stat(goSumPath)
	if err == nil && info.Size() == 0 {
		issues = append(
			issues, Issue{
				Type:    "DEPENDENCY_EMPTY_CHECKSUM",
				Message: "go.sum file is empty - run 'go mod download' to populate",
				Position: token.Position{
					Filename: goSumPath,
					Line:     1,
					Column:   1,
				},
				Severity: SeverityMedium,
			},
		)
	}

	// Check for multiple versions of same package (can indicate conflicts)
	file, err := os.Open(goSumPath)
	if err != nil {
		return issues
	}
	defer file.Close()

	packageVersions := make(map[string][]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			pkg := parts[0]
			version := parts[1]

			// Extract base package name (remove /go.mod suffix)
			pkg = strings.TrimSuffix(pkg, "/go.mod")

			if !strings.HasPrefix(version, "v") {
				continue
			}

			// Extract major.minor version
			versionParts := strings.Split(version, ".")
			if len(versionParts) >= 2 {
				var majorMinorBuilder strings.Builder
				majorMinorBuilder.WriteString(versionParts[0])
				majorMinorBuilder.WriteString(".")
				majorMinorBuilder.WriteString(versionParts[1])
				majorMinor := majorMinorBuilder.String()
				packageVersions[pkg] = appendUnique(packageVersions[pkg], majorMinor)
			}
		}
	}

	// Check for multiple major versions
	for pkg, versions := range packageVersions {
		if len(versions) > 1 {
			// Check if different major versions
			hasDifferentMajor := false
			majors := make(map[string]bool)
			for _, v := range versions {
				major := strings.Split(v, ".")[0]
				majors[major] = true
			}
			if len(majors) > 1 {
				hasDifferentMajor = true
			}

			if hasDifferentMajor {
				var messageBuilder strings.Builder
				messageBuilder.WriteString("Multiple major versions of ")
				messageBuilder.WriteString(pkg)
				messageBuilder.WriteString(" detected: ")
				messageBuilder.WriteString(strings.Join(versions, ", "))

				issues = append(
					issues, Issue{
						Type:    "DEPENDENCY_VERSION_CONFLICT",
						Message: messageBuilder.String(),
						Position: token.Position{
							Filename: goSumPath,
							Line:     1,
							Column:   1,
						},
						Severity: SeverityMedium,
					},
				)
			}
		}
	}

	return issues
}

// checkPackageVulnerabilities checks for vulnerabilities in a package and returns issues
func (a *DependencyAnalyzer) checkPackageVulnerabilities(pkg, version, goModPath string, lineNum int, messageBuilder *strings.Builder) []Issue {
	var vulnIssues []Issue

	// Check for vulnerabilities using VulnerabilityChecker
	if vulns, err := a.vulnChecker.CheckPackage(pkg, version); err == nil && len(vulns) > 0 {
		// Pre-allocate slice for vulnerabilities
		vulnIssues = make([]Issue, 0, len(vulns))

		for _, vuln := range vulns {
			severity := SeverityMedium
			if vuln.Severity == "CRITICAL" || vuln.Severity == "HIGH" {
				severity = SeverityHigh
			} else if vuln.Severity == "LOW" {
				severity = SeverityLow
			}

			messageBuilder.Reset() // Reset the builder for reuse
			messageBuilder.WriteString(fmt.Sprintf("Vulnerable dependency: %s %s", pkg, version))
			if vuln.CVE != "" {
				messageBuilder.WriteString(" (")
				messageBuilder.WriteString(vuln.CVE)
				messageBuilder.WriteString(")")
			}
			if vuln.FixedIn != "" {
				messageBuilder.WriteString(" - fixed in ")
				messageBuilder.WriteString(vuln.FixedIn)
			}

			vulnIssues = append(vulnIssues, Issue{
				Type:     "DEPENDENCY_VULNERABLE",
				Message:  messageBuilder.String(),
				Position: token.Position{Filename: goModPath, Line: lineNum},
				Severity: severity,
			})
		}
	} else {
		// Fallback to static vulnerability data
		if vulnVersions, hasVuln := vulnerablePackages[pkg]; hasVuln {
			for _, vulnVersion := range vulnVersions {
				if a.isVulnerableVersion(version, vulnVersion) {
					messageBuilder.Reset()
					messageBuilder.WriteString("Vulnerable dependency: ")
					messageBuilder.WriteString(pkg)
					messageBuilder.WriteString(" ")
					messageBuilder.WriteString(version)
					messageBuilder.WriteString(" (known vulnerabilities in ")
					messageBuilder.WriteString(vulnVersion)
					messageBuilder.WriteString(")")

					vulnIssues = append(vulnIssues, Issue{
						Type:     "DEPENDENCY_VULNERABLE",
						Message:  messageBuilder.String(),
						Position: token.Position{Filename: goModPath, Line: lineNum},
						Severity: SeverityHigh,
					})
					break
				}
			}
		}
	}

	return vulnIssues
}

func (a *DependencyAnalyzer) shouldCheckGoMod(filename string) bool {
	// Only check go.mod once per analysis run (when we see main.go or the first file)
	return strings.HasSuffix(filename, "main.go") || strings.HasSuffix(filename, filepath.Join(a.projectPath, "main.go"))
}

func (a *DependencyAnalyzer) getModulePath() string {
	goModPath := filepath.Join(a.projectPath, "go.mod")
	file, err := os.Open(goModPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module"))
		}
	}
	return ""
}

func (a *DependencyAnalyzer) isVulnerableVersion(version, vulnSpec string) bool {
	// Simple version comparison - in production would use proper semver library
	if strings.HasPrefix(vulnSpec, "<") {
		vulnVersion := strings.TrimSpace(strings.TrimPrefix(vulnSpec, "<"))
		return compareVersions(version, vulnVersion) < 0
	}
	return false
}

func (a *DependencyAnalyzer) isOldVersion(version string) bool {
	// Check if version is older than 2 years
	// This is a simplified check - in production would parse version date
	if strings.Contains(version, "2019") || strings.Contains(version, "2020") || strings.Contains(version, "2021") {
		return true
	}

	// Check for v0.x.x versions (pre-stable)
	if strings.HasPrefix(version, "v0.") {
		return true
	}

	return false
}

func compareVersions(v1, v2 string) int {
	// Simplified version comparison
	// Returns: -1 if v1 < v2, 0 if equal, 1 if v1 > v2
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	if v1 == v2 {
		return 0
	}
	if v1 < v2 {
		return -1
	}
	return 1
}

func appendUnique(slice []string, item string) []string {
	for _, existing := range slice {
		if existing == item {
			return slice
		}
	}
	return append(slice, item)
}

func (a *DependencyAnalyzer) Name() string {
	return "Dependency"
}
