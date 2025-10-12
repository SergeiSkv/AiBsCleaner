# Contributing to aiBsCleaner

Thank you for your interest in contributing to aiBsCleaner! We welcome contributions from the community.

## 🚀 Quick Start

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/AiBsCleaner.git
   cd AiBsCleaner
   ```
3. **Set up development environment**:
   ```bash
   make setup
   ```
4. **Create a branch** for your changes:
   ```bash
   git checkout -b feature/your-feature-name
   ```

## 💻 Development Workflow

### Prerequisites

- Go 1.23 or later
- golangci-lint for code quality
- Git for version control

### Building and Testing

```bash
# Build the project
make build

# Run tests
make test

# Run linter
make lint

# Run aiBsCleaner on itself
make analyze

# Generate coverage report
make coverage
```

### Code Style

We follow standard Go conventions:

- Use `gofmt` for formatting
- Follow [Effective Go](https://golang.org/doc/effective_go.html) guidelines
- Write clear, self-documenting code
- Add comments for exported functions and types
- Keep functions small and focused (max 50 lines)
- Avoid deep nesting (max 7 levels)

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run with race detector
go test -race ./...

# Run specific package
go test ./analyzer -v
```

## 📝 Pull Request Process

1. **Update tests**: Add or update tests for your changes
2. **Update documentation**: Update README.md, docs, or comments as needed
3. **Run checks**: Ensure all tests and linters pass
   ```bash
   make check-all
   ```
4. **Commit your changes**: Use [Conventional Commits](https://www.conventionalcommits.org/)
   ```bash
   git commit -m "feat: add new analyzer for X"
   git commit -m "fix: resolve issue with Y"
   git commit -m "docs: update README with Z"
   ```
5. **Push to your fork**:
   ```bash
   git push origin feature/your-feature-name
   ```
6. **Create a Pull Request** on GitHub

### Commit Message Format

We use Conventional Commits:

- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation changes
- `test:` - Test additions or changes
- `refactor:` - Code refactoring
- `perf:` - Performance improvements
- `chore:` - Build process or auxiliary tool changes

**Examples:**
```
feat: add CPU cache analyzer
fix: resolve false positive in loop detection
docs: update installation instructions
test: add tests for goroutine analyzer
perf: optimize AST traversal
```

## 🎯 What to Contribute

### High Priority

- **New Analyzers**: Add detectors for performance issues
- **Test Coverage**: Improve coverage (target: 85%+)
- **Bug Fixes**: Fix issues from GitHub Issues
- **Documentation**: Improve docs, add examples
- **Performance**: Optimize existing analyzers

### Good First Issues

Look for issues labeled `good-first-issue` on GitHub. These are suitable for newcomers.

### Creating New Analyzers

1. Create a new file in `analyzer/` directory:
   ```go
   package analyzer

   type YourAnalyzer struct {
       BaseAnalyzer
   }

   func (a *YourAnalyzer) Analyze(file *ast.File, info *types.Info) []models.Issue {
       // Your implementation
   }
   ```

2. Add to analyzer registry in `analyzer/analyzer.go`
3. Add tests in `analyzer/your_analyzer_test.go`
4. Update configuration schema if needed
5. Document in `performance_error_catalog.md`

### Writing Tests

```go
func TestYourAnalyzer(t *testing.T) {
    analyzer := &YourAnalyzer{}

    // Test positive case
    code := `
        package main
        // ... problematic code
    `

    issues := testutils.Analyze(analyzer, code)
    assert.NotEmpty(t, issues)
    assert.Equal(t, models.YourIssueType, issues[0].Type)

    // Test negative case
    goodCode := `
        package main
        // ... good code
    `

    issues = testutils.Analyze(analyzer, goodCode)
    assert.Empty(t, issues)
}
```

## 🐛 Bug Reports

When filing a bug report, please include:

1. **Description**: Clear description of the issue
2. **Steps to Reproduce**: Minimal code example
3. **Expected Behavior**: What should happen
4. **Actual Behavior**: What actually happens
5. **Environment**: Go version, OS, aiBsCleaner version
6. **Additional Context**: Error messages, logs

**Template:**
```markdown
## Description
Brief description of the bug

## To Reproduce
Steps to reproduce:
1. Run `aibscleaner ...`
2. See error

## Expected Behavior
What you expected to happen

## Actual Behavior
What actually happened

## Environment
- Go version: 1.25
- OS: macOS 14.0
- aiBsCleaner version: v1.0.0

## Additional Context
Error messages, stack traces, etc.
```

## 💡 Feature Requests

For feature requests, please include:

1. **Use Case**: Why is this feature needed?
2. **Proposed Solution**: How should it work?
3. **Alternatives**: What alternatives have you considered?
4. **Additional Context**: Examples, mockups, etc.

## 📚 Code Review Process

All submissions require review. We review PRs for:

- ✅ Code quality and style
- ✅ Test coverage
- ✅ Documentation
- ✅ Performance impact
- ✅ Backward compatibility

Reviewers may request changes before merging.

## 🔒 Security Issues

**Do NOT open public issues for security vulnerabilities.**

Instead, email security concerns to: [INSERT EMAIL]

## 📄 License

By contributing, you agree that your contributions will be licensed under the MIT License.

## 🙏 Recognition

Contributors will be:
- Listed in CHANGELOG.md
- Mentioned in release notes
- Added to GitHub contributors list

## 📞 Getting Help

- **Questions**: Open a GitHub Discussion
- **Chat**: [INSERT DISCORD/SLACK LINK if available]
- **Issues**: GitHub Issues

## Code of Conduct

We follow the [Contributor Covenant](https://www.contributor-covenant.org/) Code of Conduct.

### Our Pledge

We are committed to providing a welcoming and inspiring community for all.

### Our Standards

- ✅ Be respectful and inclusive
- ✅ Provide constructive feedback
- ✅ Focus on what's best for the community
- ❌ No harassment or trolling
- ❌ No offensive comments
- ❌ No personal attacks

---

**Thank you for contributing to aiBsCleaner!** 🎉

Your efforts help make Go code faster and cleaner for everyone.
