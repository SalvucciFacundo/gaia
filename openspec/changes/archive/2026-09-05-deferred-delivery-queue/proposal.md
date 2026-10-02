# Proposal: Deferred Delivery Queue (Local-First Outbox Pattern)

## Intent

GAIA's autonomous and overnight workflows are currently halted when reaching the delivery stage of an SDD change or task because remote operations (`git push`, `gh pr create`) require synchronous human authorization. The Deferred Delivery Queue decouples local autonomous execution, verification, and commit from remote network transport by introducing an Outbox Pattern. Changes are committed to local branches, CAS receipts are validated, and delivery manifests are queued in standby, allowing unattended pipelines to proceed continuously to subsequent tasks while aggregating a single consolidated release batch for human review.

## Scope

### In Scope
- **delivery-manifest**: Domain model representing a ready-to-deliver PR slice (`ID`, `ChangeName`, `Branch`, `BaseBranch`, `CommitSHA`, `ReceiptLineage`, `PRTitle`, `PRBody`, `Status`, `CreatedAt`).
- **delivery-queue**: Persistent local Outbox store in `.gaia/delivery/queue.json` supporting `Enqueue`, `List`, `Get`, `MarkReleased`, `MarkDiscarded`, and `Clear`.
- **delivery-modes**: Configurable execution modes: `deferred` (unattended/overnight — enqueues and advances without blocking), `interactive` (attended — prompts after each PR), and `auto` (unattended CI/CD with pre-authorized tokens).
- **delivery-engine**: Engine validating Git CAS review receipts before releasing, handling stacked branch pushes, and creating GitHub PRs via transport adapters.
- **delivery-cli**: Command-line interface `gaia delivery [list|status|release|discard|diff]` for human inspection and one-click batch release.
- **sdd-integration**: Hooking `sdd-archive` to automatically enqueue manifests when running in `deferred` mode.

### Out of Scope
- Direct GitHub webhook event receivers (GAIA uses outbound transport CLI/API).
- Multi-repository delivery in a single atomic transaction (single repository scope per queue).
- Automatic PR merging (PR merge follows repository review policy).

## Capabilities

### New Capabilities
- `delivery-domain`: Package `internal/delivery/` containing `DeliveryItem`, `DeliveryStatus` (`standby`, `released`, `discarded`, `failed`), and JSON persistence.
- `delivery-queue`: Thread-safe `Queue` interface and JSON file-backed store (`internal/delivery/queue.go`).
- `delivery-engine`: `Engine` struct orchestrating batch releases, receipt verification (`GatePrePR`/`GatePrePush`), and transport execution (`internal/delivery/engine.go`).
- `delivery-transport`: `Transport` interface abstracting `git push` and `gh pr create` with production and test stub implementations (`internal/delivery/transport.go`).
- `gaia-delivery-cli`: CLI subcommand family `gaia delivery` in `cmd/gaia/delivery.go`.

### Modified Capabilities
- `sdd-archiver`: `internal/agent/sdd/archiver.go` checks `delivery_mode`; in `deferred` mode, archives change locally and enqueues to `delivery.Queue` instead of requiring interactive push.
- `attempt-ledger`: `internal/agent/sdd/attempt_ledger.go` records delivery queue state in work unit completion history.

## Approach & Chained PR Slicing

Three atomic, chained PR slices (stacked-to-main), each strictly under 400 lines:

1. **PR1: delivery-core (~320 LOC)**: `internal/delivery/manifest.go`, `internal/delivery/queue.go`, `internal/delivery/queue_test.go`. Implements `DeliveryItem`, status transitions, file-backed queue storage, and 100% unit test coverage.
2. **PR2: delivery-engine-transport (~340 LOC)**: `internal/delivery/engine.go`, `internal/delivery/transport.go`, `internal/delivery/engine_test.go`. Implements `DeliveryEngine`, `DeliveryMode`, CAS receipt verification before push, stacked branch release logic, and test mocks.
3. **PR3: sdd-cli-integration (~350 LOC)**: `cmd/gaia/delivery.go`, `internal/agent/sdd/archiver.go` integration, `docs/delivery.md`, and integration tests for the full deferred queue lifecycle.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/delivery/` | Create | Core domain, queue, engine, and transport implementations |
| `internal/agent/sdd/archiver.go` | Modify | Hook deferred queue on archive completion |
| `cmd/gaia/delivery.go` | Create | Subcommands for `gaia delivery` |
| `cmd/gaia/main.go` | Modify | Wire delivery CLI subcommands |
| `docs/delivery.md` | Create | Technical guide for deferred delivery queue |

## Success Criteria
- [ ] Autonomous pipelines in `deferred` mode never block on remote GitHub operations.
- [ ] All completed work units generate valid `DeliveryItem` records in `.gaia/delivery/queue.json`.
- [ ] `gaia delivery release --all` releases all queued slices in correct stacked order.
- [ ] CAS receipt validation blocks release if local files drifted after review approval.
- [ ] 100% test coverage across all new packages.
