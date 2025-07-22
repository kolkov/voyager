# Changelog

## [v1.0.0-beta.6] - 2025-07-23 (Upcoming Release)
### Added
- Automatic retracted dependency detection in workflows
- Enhanced security scanning with govulncheck
- Commit history squash process for cleaner releases
- Improved Windows compatibility in CI pipelines
- Dependency verification step in CI workflows

### Changed
- Updated release guide with best practices for commit history
- Optimized Docker build process for smaller image sizes
- Upgraded GitHub Actions to latest versions
- Improved dependency management workflow
- Retracted v1.0.0-beta.5 due to checksum mismatch issues

### Fixed
- Resolved checksum verification issues for Go modules
- Fixed artifact path configuration in CI workflows
- Addressed minor logging inconsistencies
- Improved error handling in client registration
- Resolved Windows Bash script execution in CI/CD pipelines
- Fixed ETCD container startup sequence for integration tests

### Security
- Pinned gRPC to stable v1.73.0 (CVE-2025-XXXXX mitigation)
- Added govulncheck security scanning to CI pipeline
- Implemented automatic dependency vulnerability checks
- Enhanced token rotation recommendations in documentation

## [v1.0.0-beta.5] - 2025-07-22 [RETRACTED]
### Fixed
- Resolved Windows Bash script execution in CI/CD pipelines
- Fixed ETCD container startup sequence for integration tests
- Corrected coverage reporting for multi-OS environments
- Addressed GitHub artifact download path issues
- Fixed retracted dependency detection logic

### Security
- Pinned gRPC to stable v1.73.0 (CVE-2025-XXXXX mitigation)
- Added govulncheck security scanning to CI pipeline
- Implemented automatic dependency vulnerability checks

## [v1.0.0-beta.4] - 2025-07-22
### Added
- Cross-platform bash scripts replacing PowerShell in CI
- Automatic retracted dependency detection in workflows
- ETCD health checks in integration test setup
- Compression for coverage artifacts
- Quality gate for coverage thresholds

### Changed
- Upgraded to GitHub Actions artifact v4
- Migrated from services container to manual ETCD control
- Improved Windows compatibility in test runner
- Optimized Makefile security targets
- Enhanced dependency verification output

### Fixed
- Critical "dirty git state" error in release pipeline
- Artifact download path configuration
- YAML syntax errors in workflow definitions
- PowerShell/Bash shell compatibility issues
- macOS integration test container limitations

## [v1.0.0-beta.3] - 2025-07-22
### Added
- Cross-platform bash scripts replacing PowerShell
- Improved token handling in client-server communication
- Added security best practices section to documentation

## [v1.0.0-beta.2] - 2025-07-17
### Added
- Initial implementation of service health monitoring
- gRPC interceptors for automatic service discovery
- Client-side load balancing strategies
- Connection pooling with reuse and timeout
- ETCD lease management for service registration
- Basic CLI for service discovery (voyagerctl)
- Windows service installation support
- ARMv7 build support for Raspberry Pi

### Changed
- Refactored service registration protocol
- Improved error handling in client connections
- Optimized cache synchronization
- Enhanced logging with structured fields
- Updated gRPC to 1.74.0
- Improved test coverage (75% → 89%)
- Simplified configuration management

### Fixed
- Race condition in service cache
- Memory leak in gRPC connection pool
- ETCD watch channel blocking issue
- Windows service stop command
- Authentication token expiration handling
- Metrics label inconsistencies

## [v1.0.0-beta] - 2025-07-01
### Added
- Initial release of Voyager Service Discovery
- Basic Kubernetes deployment examples
- RoundRobin, Random and LeastConnections load balancing strategies
- Health check system with TTL support
