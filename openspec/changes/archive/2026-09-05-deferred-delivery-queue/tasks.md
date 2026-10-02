# Tasks: Deferred Delivery Queue System

## Review Workload Forecast

- **Estimated lines**: ~1050 LOC total
- **Decision needed before apply**: No (pre-approved SDD change)
- **Chained PRs recommended**: Yes (3 chained PRs)
- **400-line budget risk**: Low (each PR under 360 lines)
- **Chain strategy**: `stacked-to-main`
- **Allowed edit roots**:
  - `internal/delivery/`
  - `internal/agent/sdd/`
  - `cmd/gaia/`
  - `docs/`

---

## Phase 1: Core Domain & Queue Persistence (PR1: ~320 LOC)

### Work Unit: PR1.1 — Domain Manifest & Status Types
- [x] Create `internal/delivery/manifest.go` with `DeliveryStatus`, `DeliveryItem`, and helper validators.
- [x] Define error constants (`ErrItemNotFound`, `ErrInvalidStatus`, `ErrContentDrift`).
- [x] Unit tests in `internal/delivery/manifest_test.go`.

### Work Unit: PR1.2 — Thread-Safe FileQueue Store
- [x] Create `internal/delivery/queue.go` implementing `Queue` interface (`FileQueue`).
- [x] Implement `Enqueue()`, `List()`, `Get()`, `MarkReleased()`, `MarkDiscarded()`, `MarkFailed()`, `Clear()`.
- [x] Unit tests in `internal/delivery/queue_test.go` verifying concurrent access, JSON persistence, and FIFO filtering.

---

## Phase 2: Delivery Engine, CAS Gates & Transport (PR2: ~340 LOC)

### Work Unit: PR2.1 — Transport Interface & Mock Adapter
- [x] Create `internal/delivery/transport.go` with `Transport` interface (`PushBranch`, `CreatePullRequest`).
- [x] Implement `ExecTransport` (invoking `git push` and `gh pr create`) and `MockTransport` for tests.
- [x] Unit tests in `internal/delivery/transport_test.go`.

### Work Unit: PR2.2 — Delivery Engine & CAS Receipt Verification
- [x] Create `internal/delivery/engine.go` with `Engine` struct.
- [x] Implement `Release(id)`: validates CAS receipt via `gates.ReceiptStore`, calls transport, updates queue status.
- [x] Implement `ReleaseAll()`: resolves stacked branch dependency order and releases batch atomically.
- [x] Unit tests in `internal/delivery/engine_test.go` verifying receipt validation, content drift protection, and stacked release.

---

## Phase 3: SDD Archiver Hook, CLI Commands & Documentation (PR3: ~350 LOC)

### Work Unit: PR3.1 — SDD Archiver Deferred Mode Integration
- [x] Modify `internal/agent/sdd/archiver.go` to inspect `delivery_mode`.
- [x] In `deferred` mode, automatically generate PR title/body and enqueue `DeliveryItem` to `delivery.Queue`.
- [x] Unit tests in `internal/agent/sdd/sdd_test.go`.

### Work Unit: PR3.2 — CLI Commands in `cmd/gaia/`
- [x] Create `cmd/gaia/delivery.go` implementing `gaia delivery [list|status|release|discard|diff]`.
- [x] Wire `delivery` command in `cmd/gaia/main.go`.
- [x] Test CLI parsing and formatted output.

### Work Unit: PR3.3 — Technical Documentation
- [x] Create `docs/delivery.md` detailing the Outbox Pattern, deferred mode, and batch release commands.
- [x] Update `docs/cli.md` and `docs/sdd.md`.
