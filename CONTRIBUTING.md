# Contributing to GAIA

Thanks for your interest in contributing to GAIA! This project follows a structured workflow to keep changes safe, reviewable, and well-documented.

## How to contribute

1. **Fork the repository** and create a branch from `main`.
2. **Open an issue first** for substantial changes — describe what you want to do and why. Small fixes (typos, docs, bug fixes) can go straight to a PR.
3. **Follow the SDD workflow** for substantial changes: the project uses Spec-Driven Development. See [docs/sdd.md](docs/sdd.md) for the full pipeline (`explore → propose → spec → design → tasks → apply → verify → archive`).
4. **Run the tests** before submitting:
   ```bash
   go build ./cmd/gaia
   go test ./...
   ```
5. **Submit a pull request** with a clear description of the change and the problem it solves.

## Code style

- Go 1.22+, standard formatting (`gofmt`), linting via `.golangci.yml`.
- Keep the single-binary, zero-external-dependency philosophy: no runtime dependencies outside the Go toolchain.
- New features should follow the existing hexagonal architecture (ports & adapters). See [docs/architecture.md](docs/architecture.md).

## Security

- Report security vulnerabilities privately — do **not** open a public issue for them. Open a [security advisory](https://github.com/SalvucciFacundo/gaia/security/advisories/new) or contact the maintainer directly.
- If you add or modify skills, they are scanned by GAIA's AST Security Audit before installation. Make sure your code doesn't execute obfuscated commands or leak credentials.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
