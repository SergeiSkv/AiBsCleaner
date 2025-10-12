# aiBsCleaner - Performance-Focused Static Analyzer for Go

[![CI](https://github.com/SergeiSkv/AiBsCleaner/workflows/CI/badge.svg)](https://github.com/SergeiSkv/AiBsCleaner/actions)
[![Go Version](https://img.shields.io/badge/Go-1.23%2B-00ADD8?style=flat&logo=go)](https://go.dev)
[![GoDoc](https://pkg.go.dev/badge/github.com/SergeiSkv/AiBsCleaner)](https://pkg.go.dev/github.com/SergeiSkv/AiBsCleaner)
[![Go Report Card](https://goreportcard.com/badge/github.com/SergeiSkv/AiBsCleaner)](https://goreportcard.com/report/github.com/SergeiSkv/AiBsCleaner)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Release](https://img.shields.io/github/release/SergeiSkv/AiBsCleaner.svg)](https://github.com/SergeiSkv/AiBsCleaner/releases)
[![Docker](https://img.shields.io/docker/pulls/sergeiskv/aibscleaner.svg)](https://hub.docker.com/r/sergeiskv/aibscleaner)
[![codecov](https://codecov.io/gh/SergeiSkv/AiBsCleaner/branch/main/graph/badge.svg)](https://codecov.io/gh/SergeiSkv/AiBsCleaner)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

> **Performance analyzer that finds issues standard linters miss**

**aiBsCleaner** - A high-performance static analyzer for Go that focuses on detecting performance bottlenecks, algorithmic complexity issues, and AI-generated anti-patterns.

## 🎯 What It Does

aiBsCleaner identifies performance issues that standard linters don't catch:

- **Algorithmic Complexity**: O(n²/n³) loops, inefficient algorithms
- **Memory Issues**: Memory leaks, goroutine leaks, GC pressure
- **Database Problems**: N+1 queries, missing prepared statements
- **Concurrency Issues**: Race conditions, deadlocks, channel misuse
- **AI-Generated Anti-patterns**: Over-engineered solutions, unnecessary complexity
- **Resource Management**: Unclosed resources, defer overhead

## 🚀 Quick Start

### Installation

**Using Go:**
```bash
go install github.com/SergeiSkv/AiBsCleaner@latest
```

**Using Homebrew (macOS/Linux):**
```bash
brew install SergeiSkv/tap/aibscleaner
```

**Using Docker:**
```bash
docker pull sergeiskv/aibscleaner:latest
docker run -v $(pwd):/workspace sergeiskv/aibscleaner .
```

**From Release (Linux/macOS/Windows):**
```bash
# Download from GitHub Releases
wget https://github.com/SergeiSkv/AiBsCleaner/releases/latest/download/aibscleaner-linux-amd64
chmod +x aibscleaner-linux-amd64
sudo mv aibscleaner-linux-amd64 /usr/local/bin/aibscleaner
```

### Basic Usage

```bash
# Analyze current directory
./aiBsCleaner .

# Analyze specific path
./aiBsCleaner ./src

# With configuration
./aiBsCleaner --config .aiBsCleaner.yaml .

# Enable cache for faster subsequent runs
./aiBsCleaner --enable-cache .

# JSON output for CI/CD
./aiBsCleaner --json .
```

## 📊 Current Status

- **33 Specialized Analyzers** covering performance, security, and code quality
- **Test Coverage**: 66.8% (analyzer: 82.9%, cache: 56.9%)
- **Zero External Dependencies** for core functionality
- **Fast**: Processes large codebases efficiently with caching

## 🔍 Analyzers

### Performance Analyzers

- **Loop Analyzer**: Detects nested loops, allocations in loops, O(n²) complexity
- **Memory Leak Analyzer**: Finds goroutine leaks, unclosed resources
- **GC Pressure Analyzer**: Identifies excessive allocations
- **Defer Optimization**: Analyzes defer overhead in hot paths
- **CPU Optimization**: Cache-friendly struct layout suggestions
- **String Analyzer**: String concatenation in loops, builder usage

### Concurrency Analyzers

- **Race Condition Analyzer**: Unsafe concurrent access patterns
- **Goroutine Analyzer**: Goroutine leaks, unbounded goroutines
- **Channel Analyzer**: Deadlocks, unbuffered channels in hot paths
- **Sync Pool Analyzer**: sync.Pool usage optimization

### Database & Network

- **Database Analyzer**: N+1 queries, SQL in loops, connection leaks
- **HTTP Client Analyzer**: Missing timeouts, connection reuse
- **HTTP Reuse Analyzer**: Connection pooling optimization
- **Network Patterns**: Efficient networking patterns

### Code Quality

- **AI Bullshit Analyzer**: Over-engineered solutions, unnecessary complexity
- **Interface Analyzer**: Interface pollution, empty interfaces
- **Reflection Analyzer**: Reflection misuse, performance impact
- **API Misuse Analyzer**: Standard library misuse

### Security & Best Practices

- **Crypto Analyzer**: Weak crypto, insecure random
- **Privacy Analyzer**: PII exposure, sensitive data logging
- **Context Analyzer**: Context misuse, cancellation handling
- **Error Handling**: Error checking, panic recovery

## 📝 Configuration

Create `.aiBsCleaner.yaml`:

```yaml
analyzers:
  loop:
    enabled: true
  defer_optimization:
    enabled: true
  memory_leak:
    enabled: true
  ai_bullshit:
    enabled: true

thresholds:
  max_loop_depth: 3
  max_complexity: 10
  max_function_length: 50

paths:
  exclude:
    - vendor/
    - testdata/
    - "*.pb.go"
    - "*_test.go"

output:
  format: text  # or: json, compact
  show_context: false
```

## 🎯 Example Output

```
Analyzing project...
Found 42 issues

HIGH SEVERITY (5)
────────────────────────────────────────
loop_analyzer.go:124  NESTED_LOOP_O_N3
  Triple nested loop detected: O(n³) complexity
  Suggestion: Consider using hash map for O(n) lookup
  PVE-001

database.go:67  N_PLUS_ONE_QUERY
  Query in loop: SELECT * FROM users WHERE id = ?
  Suggestion: Use JOIN or batch loading
  PVE-130

MEDIUM SEVERITY (18)
────────────────────────────────────────
handler.go:45  ALLOC_IN_LOOP
  Allocation inside loop may cause GC pressure
  Suggestion: Preallocate slice with make()
  PVE-001

AI BULLSHIT DETECTED (12)
────────────────────────────────────────
utils.go:23  OVERENGINEERED_SIMPLE
  20 lines to check if number is even
  Suggestion: Just use: n%2 == 0
  PVE-201
```

## 🛠️ Development

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Specific package
go test ./analyzer -v
```

### Building

```bash
# Build binary
go build -o aiBsCleaner .

# Run linter
golangci-lint run ./...

# Run on self
./aiBsCleaner .
```

## 📈 Performance

- **Fast Analysis**: Processes ~1000 files/second
- **Smart Caching**: 10x faster on subsequent runs
- **Low Memory**: <100MB for most projects
- **Parallel Processing**: Utilizes multiple cores

## 🗺️ Roadmap

### v1.0.0 (Production Ready) ✅

- ✅ 33 production-ready analyzers
- ✅ Test coverage improvements (66.8%)
- ✅ Dead code removal
- ✅ Cache system with deadlock fixes
- ✅ CI/CD pipeline with GitHub Actions
- ✅ GoReleaser for automated releases
- ✅ Docker support
- ✅ Multi-platform builds (Linux/macOS/Windows)
- ✅ Security scanning with Gosec
- ✅ Comprehensive documentation

### Future Plans

- [ ] IDE integrations (VS Code, GoLand)
- [ ] Auto-fix suggestions
- [ ] HTML/SARIF report formats
- [ ] More analyzers (additional crypto patterns)
- [ ] Performance benchmarking suite
- [ ] GitHub App integration

## 🤝 Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

**Areas of focus:**

1. **New Analyzers**: Add performance detectors
2. **Test Coverage**: Improve from 66.8% to 85%+
3. **Documentation**: Usage examples, best practices
4. **Bug Reports**: Real-world testing feedback

**Quick start:**
```bash
git clone https://github.com/SergeiSkv/AiBsCleaner.git
cd AiBsCleaner
make setup
make test
```

## 📜 License

MIT License - see [LICENSE](LICENSE) file

## 🙏 Acknowledgments

- Go team for excellent static analysis tools
- Community for feedback and contributions
- All developers fighting performance issues

## 📚 Documentation

- [Performance Error Catalog](performance_error_catalog.md) - All PVE codes
- [PVE Codes Reference](PVE_CODES.md) - Detailed error descriptions
- [Contributing Guide](CONTRIBUTING.md) - How to contribute
- [Changelog](CHANGELOG.md) - Release history
- [Testing Documentation](TESTING.md) - Test guides

---

**Find performance issues before they find production** 🔍
