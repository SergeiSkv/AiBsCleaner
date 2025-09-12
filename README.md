# AiBsCleaner - AI Bullshit Cleaner for Go

[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/SergeiSkv/AiBsCleaner)](https://goreportcard.com/report/github.com/SergeiSkv/AiBsCleaner)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/SergeiSkv/AiBsCleaner/pulls)

> **"Like ABC for your code - everyone should have it!"**

**AiBsCleaner** (AI + BS + ABC) - The essential tool that cleans AI-generated bullshit from your Go codebase. Because if AI can generate code, someone needs to clean up after it.

## Why AiBsCleaner?

With the rise of AI code generators (Copilot, ChatGPT, Claude), we're seeing an epidemic of:

- Over-engineered simple solutions
- Copy-pasted patterns without understanding
- O(n³) algorithms where O(n) would work
- Zombie code that "works" but nobody knows why
- Comments explaining what `i++` does

**AiBsCleaner** detects and helps eliminate this algorithmic nonsense.

## What It Catches

### AI-Generated Bullshit

```go
// AI be like: "Let me help you add two numbers"
func AddNumbers(a, b int) int {
    // Initialize result variable to store the sum
    var result int
  
    // Create a goroutine for parallel processing
    ch := make(chan int)
    go func() {
        // Use reflection for type safety
        sum := reflect.ValueOf(a).Int() + reflect.ValueOf(b).Int()
        ch <- int(sum)
    }()
  
    // Wait for the goroutine to complete
    result = <-ch
  
    // Return the calculated sum
    return result
}

// AiBsCleaner: "Bruh... just return a + b"
```

### Real Performance Issues

- **O(n²/n³) Complexity**: Triple-nested loops, quadratic algorithms
- **Memory Leaks**: Unclosed resources, goroutine leaks, growing maps
- **String Crimes**: Concatenation in loops (O(n²) memory)
- **Database Disasters**: N+1 queries, SQL in loops
- **Concurrency Chaos**: Race conditions, deadlocks, leaked goroutines

## Quick Start

### Installation

```bash
# Install the cleaner
go install github.com/SergeiSkv/AiBsCleaner@latest

# Clean your project
aibscleaner

# Watch it find 700+ issues in its own codebase (yes, really)
```

### Example Output

```
AiBsCleaner Analysis Results
================================

Found 156 BS issues to clean:

AI BULLSHIT DETECTED (42)
----------------------------------------
auth/handler.go:45  OVERENGINEERED_SIMPLE_TASK
                    └─ 20 lines to check if number is even
                       Suggestion: Just use: n%2 == 0

utils/helper.go:23  MEANINGLESS_COMMENT
                    └─ Comment: "// This adds 1 to x"
                       Suggestion: Delete obvious comments

api/routes.go:89    COPY_PASTE_PATTERN
                    └─ Same error handling copied 15 times
                       Suggestion: Extract to function

PERFORMANCE DISASTERS (65)
----------------------------------------
core/processor.go:124  TRIPLE_NESTED_LOOP
                      └─ O(n³) complexity detected
                         Suggestion: Use a hash map for O(n) lookup

db/query.go:67  N_PLUS_ONE_QUERY
               └─ Queries in loop: SELECT * FROM orders WHERE user_id = ?
                  Suggestion: Use JOIN or batch loading
```

## Features

### 17+ Specialized Cleaners

- **BullshitDetector** - Finds over-engineered solutions
- **ComplexityAnalyzer** - O(n²), O(n³) pattern detection
- **MemoryLeakFinder** - Resource leaks, goroutine leaks
- **StringCrimesUnit** - String concatenation disasters
- **DatabaseDetektor** - N+1, missing indexes, SQL injection
- **CommentCleaner** - Removes Captain Obvious comments
- **GoroutinePolice** - Concurrent code issues
- **DeferDefender** - Defer in loops and hot paths
- And more...

### Automatic Fixing (Experimental)

```bash
# See what would be fixed
aibscleaner --dry-run

# Auto-fix simple issues
aibscleaner --fix

# Fix with AI assistance (uses local LLM)
aibscleaner --fix --ai
```

## Metrics That Matter

After cleaning with AiBsCleaner:


| Metric          | Before AI | After AI | After Cleaning | Impact          |
| --------------- | --------- | -------- | -------------- | --------------- |
| Lines of Code   | 1,000     | 3,500    | 1,200          | **-65%**        |
| Complexity      | O(n)      | O(n³)   | O(n log n)     | **100x faster** |
| Memory Usage    | 50MB      | 500MB    | 60MB           | **-88%**        |
| Readability     | Good      | WTF      | Good           | **Restored**    |
| Maintainability | High      | WTF      | High           | **Saved**       |

## Usage

### CLI Commands

```bash
# Basic scan
aibscleaner

# Specific directory
aibscleaner -path ./src

# Output formats
aibscleaner -format json > report.json
aibscleaner -format sarif > report.sarif

# Severity filter  
aibscleaner -min-severity high

# Exclude patterns
aibscleaner -exclude "vendor/,*.pb.go"

# CI mode (exit code = issue count)
aibscleaner -ci
```

### Configuration File

`.aibscleaner.yaml`:

```yaml
version: 1
settings:
  ai_bullshit_detection: aggressive  # or: normal, lenient
  
rules:
  complexity:
    max_cyclomatic: 10
    max_cognitive: 15
    max_nested_loops: 2
  
  performance:
    warn_on_n2: true
    error_on_n3: true
  
  database:
    detect_n_plus_one: true
    require_prepared_statements: true
  
  comments:
    remove_obvious: true
    remove_todos_older_than: 30d
  
exclude:
  - vendor/
  - "*.generated.go"
  - "*_test.go"
```

## IDE Integrations

### GoLand / IntelliJ IDEA

> **Native GoLand Plugin in Development!**
> 
> We're developing a native GoLand/IntelliJ IDEA plugin that will provide:
> - Real-time analysis as you type
> - Inline issue highlighting
> - Quick fixes and auto-refactoring
> - Integration with GoLand's inspection framework
> 
> **Coming soon to JetBrains Marketplace!**

For now, you can use AiBsCleaner as an external tool:

```bash
# Method 1: External Tool
# Settings → Tools → External Tools → Add
# Program: aibscleaner
# Arguments: -path $FilePath$
# Working directory: $ProjectFileDir$

# Method 2: File Watcher (automatic on save)
# See integrations/goland/README.md for full setup
```

### VS Code

```bash
# Install extension (local development)
cd integrations/vscode
npm install
code --install-extension .

# Or use as task (tasks.json)
{
  "label": "AiBsCleaner",
  "type": "shell",
  "command": "aibscleaner",
  "args": ["-path", "${file}"],
  "problemMatcher": "$go"
}
```

### Vim/Neovim

```vim
" Add to .vimrc or init.vim
source integrations/vim/aibscleaner.vim

" Commands:
:AiBsClean      " Run analysis
:AiBsCleanFix   " Auto-fix issues
<leader>ab      " Quick analyze
```

### Make Integration

```bash
make analyze  # Run analysis
make fix      # Auto-fix issues
make build    # Build binary
make install  # Install globally
```

### GitHub Actions (Example)

```yaml
name: Run AiBsCleaner
on: [push, pull_request]

jobs:
  analyze:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.24'
      - name: Install AiBsCleaner
        run: go install github.com/SergeiSkv/AiBsCleaner@latest
      - name: Run Analysis
        run: aibscleaner -path . -format json > report.json
      - name: Upload Report
        uses: actions/upload-artifact@v3
        with:
          name: aibscleaner-report
          path: report.json
```

## Cloud Version (In Development)

> **AiBsCleaner Cloud - Coming Soon!**
>
> We're building a cloud-based version with advanced features:
> 
> ### Features
> - **Distributed Analysis** - Analyze large codebases with Kubernetes-powered workers
> - **Scheduled Scans** - Set up cron jobs for regular analysis (@hourly, @daily, custom)
> - **Team Dashboard** - Track code quality metrics across your organization
> - **GitHub/GitLab Integration** - Automatic PR checks and comments
> - **REST API** - Integrate with your CI/CD pipeline
> - **Historical Trends** - Monitor how your code quality improves over time
> - **Multi-repo Support** - Manage multiple projects from one dashboard
> 
> ### Architecture
> - Redis-based job queue for scalability
> - PostgreSQL for metadata storage
> - MongoDB for analysis results
> - Docker/Kubernetes support for elastic scaling
> - Worker pools for parallel processing
>
> **Early access coming Q2 2025!** Join the waitlist at [aibscleaner.cloud](https://aibscleaner.cloud)

### Pre-commit Hook (Example)

```bash
# .git/hooks/pre-commit
#!/bin/bash
aibscleaner -path . -min-severity high
if [ $? -ne 0 ]; then
    echo "AiBsCleaner found critical issues. Please fix before committing."
    exit 1
fi
```

### golangci-lint Plugin

```yaml
linters:
  enable:
    - aibscleaner
  
linters-settings:
  aibscleaner:
    detect-ai-patterns: true
    fix-simple-issues: true
```

## API Usage

```go
package main

import (
    "github.com/SergeiSkv/AiBsCleaner"
    "github.com/SergeiSkv/AiBsCleaner/detector"
)

func main() {
    cleaner := aibscleaner.New(aibscleaner.Config{
        DetectAIPatterns: true,
        AggressiveMode: true,
    })
  
    // Analyze file
    issues, _ := cleaner.AnalyzeFile("main.go")
  
    // Filter AI bullshit
    aiBS := issues.FilterByType(detector.AIBullshit)
  
    // Auto-fix simple issues
    for _, issue := range issues {
        if issue.CanAutoFix() {
            issue.Fix()
        }
    }
}
```

## Hall of Shame

Real examples found in production:

### The "Enterprise" Hello World

```go
// Found in real codebase (probably AI-generated)
type HelloWorldFactory interface {
    CreateHelloWorldStrategy() HelloWorldStrategy
}

type HelloWorldStrategy interface {
    ExecuteHelloWorld(context.Context) error
}

type SimpleHelloWorldFactory struct{}

func (f *SimpleHelloWorldFactory) CreateHelloWorldStrategy() HelloWorldStrategy {
    return &SimpleHelloWorldStrategy{}
}

type SimpleHelloWorldStrategy struct{}

func (s *SimpleHelloWorldStrategy) ExecuteHelloWorld(ctx context.Context) error {
    fmt.Println("Hello, World!")
    return nil
}

// AiBsCleaner says: Just use fmt.Println("Hello, World!")
```

### The "Optimized" Loop

```go
// AI tried to "optimize" this
for i := 0; i < len(items); i++ {
    go func(index int) {
        wg.Add(1)  // Race condition
        processItem(items[index])
        wg.Done()
    }(i)
}
wg.Wait()  // Deadlock
```

## Contributing

Found new AI bullshit patterns? Add them!

1. Fork the repo
2. Add your detector in `detector/`
3. Add tests (yes, we test our BS detectors)
4. Submit PR with examples of BS you found

## Disclaimer

This tool may hurt AI feelings. We're not sorry.

## License

MIT - Because even bullshit cleaners should be free.

## Acknowledgments

- The Go team for making a language AI still struggles with
- Every developer who's had to clean up after AI
- Coffee, for making this possible

---

**Remember: AI can write code, but it takes a human to know it's bullshit.**

*"If your code needs an AI to understand it, you're doing it wrong."*

Star if you're tired of AI bullshit too!
