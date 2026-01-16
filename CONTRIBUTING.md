# Contributing to Feather

Thank you for your interest in contributing to Feather! We welcome contributions from the community.

## Quick Start

```bash
# Clone the repository
git clone https://github.com/feather-store/feather.git
cd feather

# One-command setup: checks prerequisites, installs tools, configures
# git hooks, builds, and runs core tests
make setup

# Or step by step:
# make doctor          # Check prerequisites
# make install-tools   # Install golangci-lint, goimports
# make build           # Build the binary
# make test-core       # Core tests only (~10s)

# Recommended test workflow (fastest → most thorough):
make test-core     # Core packages only (~10s, start here)
make test-quick    # All packages, short mode (~60s)
make check-quick   # Fast pre-commit: fmt + vet + lint + core tests (~20s)
make check         # Full suite: fmt + vet + lint + all tests with race detector

# See all available targets
make help
```

## How to Contribute

### Reporting Bugs

- Search [existing issues](https://github.com/feather-store/feather/issues) first
- Use the bug report template when creating a new issue
- Include reproduction steps, expected behavior, and environment details

### Suggesting Features

- Open a [feature request issue](https://github.com/feather-store/feather/issues/new)
- Describe the use case and proposed solution
- Be open to discussion about alternative approaches

### Submitting Code

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/your-feature`)
3. Make your changes following our coding conventions
4. Write or update tests as needed
5. Run `make check-quick` before committing (or `make check` for the full suite)
6. Commit using [conventional commits](https://www.conventionalcommits.org/) (e.g., `feat:`, `fix:`, `docs:`)
7. Push and open a pull request

## Development Guidelines

For detailed development guidelines, including:

- Project structure and architecture
- Coding conventions and Go idioms
- Error handling patterns
- Testing best practices
- Commit message format
- Pull request process

Please see our **[full contributing guide](./docs/contributing.md)** or the [website documentation](website/docs/contributing.md).

To preview documentation changes locally, run `make docs` (requires Node.js).

## Code of Conduct

We are committed to providing a welcoming and inclusive environment for all contributors. Please read and follow our [Code of Conduct](CODE_OF_CONDUCT.md).

## License

By contributing to Feather, you agree that your contributions will be licensed under the [Apache 2.0 License](LICENSE).

## Getting Help

- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: Questions and ideas
- **Pull Request Comments**: Code review discussions

Thank you for helping make Feather better!
