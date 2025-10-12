# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- GoReleaser configuration for automated releases
- Comprehensive CI/CD pipeline with GitHub Actions
- Multi-platform support (Linux, macOS, Windows)
- Docker and Docker Compose setup
- Security scanning with Gosec
- Code coverage reporting with Codecov
- Homebrew tap support for easy installation

### Fixed
- golangci-lint issues (emptyStringTest, prealloc, goconst)
- Self-analysis issues identified by aiBsCleaner

### Changed
- Improved build process with proper version injection
- Enhanced documentation structure

## [1.0.0] - TBD

### Added
- 33 specialized analyzers for Go code
- Performance issue detection (loops, memory, GC pressure)
- Concurrency pattern analysis (goroutines, channels, race conditions)
- AI-generated code detection
- Database optimization (N+1 queries, connection leaks)
- Security analysis (crypto, privacy, PII exposure)
- Smart caching system for faster analysis
- Configuration via YAML
- Multiple output formats (text, JSON, compact)
- CLI with commands: init, list-analyzers, stats, version
- Comprehensive test coverage (66.8%+)
- Detailed PVE (Performance & Validation Error) code catalog

### Performance
- Processes ~1000 files/second
- 10x faster with caching enabled
- Low memory footprint (<100MB)
- Parallel processing support

### Documentation
- Performance Error Catalog
- PVE Codes Reference
- Testing guides
- Agent documentation
- False positive analysis

## [0.1.0] - Initial Release

### Added
- Initial analyzer implementation
- Basic CLI functionality
- Core performance detection algorithms

---

## Version History

- **v1.0.0** - First production-ready release
- **v0.1.0** - Initial beta release

## Migration Guides

### Upgrading to v1.0.0

No breaking changes from v0.1.0. All configuration files remain compatible.

## Contributors

Thanks to all contributors who helped make aiBsCleaner better!

See the full list of contributors on [GitHub](https://github.com/SergeiSkv/AiBsCleaner/graphs/contributors).
