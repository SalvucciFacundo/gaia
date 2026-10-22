---
description: "GAIA Architecture & Go Engineering Rules"
globs: ["**/*.go"]
alwaysApply: true
---

# Architecture & Go Engineering Rules

Before editing or creating code in this repository:
1. Review `AGENTS.md` in the project root for package boundaries and the Go skills matrix.
2. Check the matching skill in `~/.gemini/antigravity-cli/skills/golang-*` for the technical area being modified (concurrency, context, error handling, security, database, etc.).
3. Verify that `internal/core/` does NOT import outer layers (`adapters`, `agent`, `commands`, `modules`, `tools`, etc.).
4. Run `go vet ./...` and `go test -race ./...` before considering any task complete.
