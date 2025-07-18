# Contributing to Voyager

We welcome contributions from the community! Before you start, please read these guidelines.

## Getting Started
1. Fork the repository
2. Clone your fork: `git clone https://github.com/your-username/voyager.git`
3. Create a new branch: `git checkout -b feature/your-feature`

## Development
- Run `make install-tools` to install required tools
- Use `make generate` to regenerate protobuf files
- Write tests for new functionality
- Ensure all tests pass: `make test`

## Code Style
- Follow Go standard formatting
- Use descriptive variable names
- Comment public functions and types
- Keep functions small and focused

## Submitting a Pull Request
1. Ensure your code passes all tests
2. Update documentation if needed
3. Describe your changes in CHANGELOG.md
4. Push your branch: `git push origin feature/your-feature`
5. Open a pull request against the `main` branch

## Reporting Issues
Please include:
- Voyager version
- Operating system
- Steps to reproduce
- Expected behavior
- Actual behavior