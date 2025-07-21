# Contributing to Voyager

We welcome contributions from the community! Before you start, please read these guidelines.

## Getting Started
1. Fork the repository
2. Clone your fork: `git clone https://github.com/your-username/voyager.git`
3. Create a new feature branch: `git checkout -b feature/your-feature develop`

## Development Environment Setup
1. Run the setup script: `./dev-setup.sh`
2. Install required tools: `make install-tools`
3. Generate code: `make generate`
4. Build binaries: `make build`

## Code Style
- Follow Go standard formatting (`gofmt`)
- Use descriptive variable and function names
- Comment public functions and types
- Keep functions focused and concise
- Write comprehensive tests for new functionality

## Development Process
1. Work on your feature branch
2. Commit changes regularly with meaningful messages:
   ```bash
   git add .
   git commit -m "feat: add new functionality"
   ```
3. Keep your branch updated with the latest changes from `develop`:
   ```bash
   git pull origin develop
   ```

## Testing Requirements
- Write unit tests for all new code
- Include integration tests for complex features
- Ensure all tests pass: `make test`
- Maintain test coverage above 85%
- Run tests locally before submitting PR:
  ```bash
  make test-unit       # Unit tests
  make test-integration # Integration tests (non-Windows)
  ```

## Submitting a Pull Request
1. Ensure your code passes all checks:
   ```bash
   make lint
   make test
   ```
2. Update documentation:
  - Add relevant sections to README.md
  - Update examples if needed
  - Add new configuration options to documentation
3. Describe your changes in CHANGELOG.md under `## [Unreleased]` section
4. Push your branch:
   ```bash
   git push origin feature/your-feature
   ```
5. Open a pull request against the `develop` branch
6. In your PR description:
  - Explain the purpose of the changes
  - Document any breaking changes
  - Reference related issues

## Beta Phase Contributions
During our beta phase, we especially welcome:
- Bug reports with reproduction steps
- Performance improvements
- Additional test coverage
- Documentation enhancements
- Compatibility fixes for different environments

## Reporting Issues
Please include:
- Voyager version (`voyagerd --version`)
- Operating system and architecture
- Steps to reproduce
- Expected behavior
- Actual behavior
- Relevant log output
- Environment details (ETCD version, etc.)

## Code Review Process
- All PRs require at least one approval from core maintainers
- PRs must pass CI checks (linting, tests, build)
- Maintainers may request changes before merging
- Discussion is encouraged - feel free to ask questions!

## Release Process
- Follow our [Release Guide](RELEASE_GUIDE.md) for detailed instructions
- All releases go through release branches
- Versioning follows Semantic Versioning (SemVer)

## Makefile Reference
```bash
make install-tools    # Install development tools
make generate        # Generate protobuf code
make build           # Build all binaries
make test            # Run unit tests
make test-integration# Run integration tests
make lint            # Run linters
make docker          # Build Docker images
make run             # Run services locally
make release-test    # Run release validation checks
```

## Getting Help
- Join our [Discord community](https://discord.gg/voyager-sd)
- File GitHub issues for bugs or feature requests
- Check documentation in `/docs` folder

## License
By contributing to Voyager, you agree that your contributions will be licensed under the [Apache 2.0 License](LICENSE).
