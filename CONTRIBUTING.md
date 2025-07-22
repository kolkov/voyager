# Contributing to Voyager

We welcome contributions from the community! Before you start, please read these guidelines carefully.

## 🚀 Getting Started
1. **Fork** the repository
2. **Clone** your fork:
   ```bash
   git clone https://github.com/your-username/voyager.git
   ```
3. **Create a new branch** from `develop`:
   ```bash
   git checkout develop
   git pull
   git checkout -b feature/your-feature
   ```

## 🛠️ Development Setup
Run the development setup script to ensure a consistent environment:
```bash
./dev-setup.sh
```

## 🔧 Development Workflow
- Install required tools: `make install-tools`
- Generate protobuf code: `make generate`
- Write tests for new functionality
- Ensure all tests pass: `make test`
- Maintain at least 85% test coverage
- Check for security vulnerabilities: `make security`
- Verify code quality: `make lint`

### Code Style Guidelines
- Follow Go standard formatting (`gofmt`)
- Use descriptive variable and function names
- Comment public functions and complex logic
- Keep functions focused and under 50 lines
- Use consistent error handling patterns
- Write comprehensive unit tests
- Maintain clean import grouping

## 📝 Submitting a Pull Request
1. Ensure your code passes all checks:
   ```bash
   make release-test
   ```
2. Update relevant documentation (README, CHANGELOG, etc.)
3. Describe your changes in CHANGELOG.md under "Unreleased"
4. Push your branch:
   ```bash
   git push origin feature/your-feature
   ```
5. Open a **Pull Request against the `develop` branch**
6. Include a clear description of changes and motivation

### Pull Request Requirements
- Minimum 1 approval from core maintainers
- All CI checks must pass (tests, lint, security)
- Code coverage must not decrease
- Must include relevant tests
- Must be rebased on latest `develop` branch
- Must follow semantic commit message conventions

## 🐛 Reporting Issues
Please include all relevant details:
- Voyager version (e.g., v1.0.0-beta.6)
- Operating system and architecture
- Exact steps to reproduce
- Expected behavior
- Actual behavior
- Relevant logs or error messages
- Environment details (ETCD version, etc.)

## 🔒 Security Vulnerabilities
If you discover a security issue, please disclose it responsibly:
1. Do NOT create a public issue
2. Email security@kolkov.com with details
3. Use our PGP key for sensitive reports
4. We will acknowledge within 48 hours

## 🌟 First-Time Contributors
Look for issues labeled `good first issue` to start with. We're happy to help you through the process!

## 📜 Commit Message Convention
Use semantic commit messages:
```
feat: add new discovery endpoint
fix(server): resolve cache race condition
docs: update authentication examples
chore: update dependencies
refactor(client): simplify connection pooling
test(server): add healthcheck tests
```

## ✅ Code Review Process
1. Maintainers will review within 3 business days
2. You may be asked to make changes
3. Once approved, your PR will be squashed and merged
4. Your contribution will appear in the next release

## 🧪 Testing Guidelines
- Unit tests for all new functionality
- Integration tests for ETCD interactions
- Edge case and error condition coverage
- Cross-platform verification (Linux, Windows, macOS)
- Benchmark tests for performance-critical code

## 📚 Documentation Standards
- Update relevant documentation with changes
- Keep examples up-to-date
- Use clear, concise language
- Add diagrams for complex workflows
- Maintain both English and Russian versions

## 🤝 Community Standards
- Be respectful and inclusive
- Assume positive intent
- Give constructive feedback
- Help others when possible
- Follow the Code of Conduct

We appreciate your contribution to making Voyager better! 🎉
