# Changelog

## [v1.0.0-beta.2] - 2025-07-17
### Added
- Cross-platform bash scripts replacing PowerShell
- Full release automation workflow with GoReleaser
- Developer guides in English and Russian (RELEASE_GUIDE.md, RELEASE_GUIDE.ru.md)
- New Makefile commands for release management:
    - `release-prepare`: Create release branch
    - `release-publish`: Publish release to main
    - `release-test`: Run release validation checks
    - `release-guide`: Open release documentation
- Multi-architecture Docker builds (linux/amd64, linux/arm64)
- GitHub Actions workflow enhancements:
    - Release validation step
    - Automatic snapshot builds on PRs
    - Improved caching strategy

### Changed
- Complete Makefile overhaul:
    - Unified build process for all platforms
    - Improved Windows/Linux/macOS compatibility
    - Enhanced version information injection
    - Better error handling and diagnostics
- Linting configuration updated to latest standards
- CI/CD pipeline optimizations (40% faster builds)
- Documentation restructuring and improvements
- Kubernetes deployment examples updated for beta images

### Fixed
- All linter warnings and errors across codebase
- Windows path handling issues in build scripts
- ETCD connection stability in container environments
- Service discovery cache refresh logic
- Metrics collection for long-running instances
- Connection pooling resource leaks

### Security
- Updated all dependencies to address known vulnerabilities
- Improved token handling in client-server communication
- Added security best practices section to documentation

## [v1.0.0-beta] - 2025-07-01
### Added
- Initial release of Voyager Service Discovery
- ETCD backend support
- In-memory mode for development
- Prometheus metrics integration
- Production-ready CLI (voyagerd)
- Basic Kubernetes deployment examples
- RoundRobin, Random and LeastConnections load balancing strategies
- Health check system with TTL support