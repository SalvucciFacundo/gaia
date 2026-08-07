# Contributing to GAIA

Thanks for your interest in contributing to GAIA! We welcome contributions from the community.

## How to contribute

1. **Fork the repository** and create your feature branch from `main`.
2. **Open an issue first** for substantial features or architectural changes — describe what you plan to build so we can align on design early.
3. **Run tests & verification** before submitting your pull request:
   ```bash
   go build ./cmd/gaia
   go test ./...
   ```
4. **Submit a pull request** with a clear description of the problem solved and changes made.

## Code style & guidelines

- **Go 1.23+**: Ensure your code is compatible with Go 1.23+ and formatted using `gofmt`.
- **Zero external runtime dependencies**: Preserve the single-binary philosophy (no CGO requirements or external non-Go library dependencies).
- **Architecture**: Follow the existing hexagonal architecture (ports & adapters). See [docs/unified-architecture.md](docs/unified-architecture.md).

## Security

- Report security vulnerabilities privately. Do **not** open a public issue for zero-days or sensitive security bugs. Contact the maintainer directly or use GitHub Security Advisories.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
