# Specification: Deferred Delivery Queue System

## 1. Domain Model: DeliveryItem and Status

### Requirement: Delivery Item Data Integrity
Every delivery slice queued for remote release MUST contain immutable references to the local git commit, branch, and CAS review receipt.

#### Scenario: Enqueue a valid delivery item
- **GIVEN** a completed and reviewed SDD change on local branch `feature/auth-models`
- **WHEN** the change is archived in `deferred` mode
- **THEN** a `DeliveryItem` MUST be created containing `id`, `change_name`, `branch`, `base_branch`, `commit_sha`, `receipt_lineage`, `pr_title`, `pr_body`, and `status = "standby"`
- **AND** `created_at` MUST be set to current UTC time.

#### Scenario: Status transitions
- **GIVEN** a `DeliveryItem` in `standby` status
- **WHEN** `Release()` is executed successfully
- **THEN** status MUST transition to `released` with `released_at` recorded
- **WHEN** `Discard()` is executed
- **THEN** status MUST transition to `discarded`.

---

## 2. Queue Persistence & Isolation

### Requirement: Persistent Local Outbox Store
The delivery queue MUST persist state locally under `.gaia/delivery/queue.json` and survive agent restarts and compactions.

#### Scenario: Thread-safe queue persistence
- **GIVEN** an active `DeliveryQueue`
- **WHEN** multiple tasks enqueue items concurrently
- **THEN** the queue MUST serialize disk writes safely using read-write locks
- **AND** disk errors MUST return structured, non-fatal errors.

#### Scenario: Filtering by status
- **GIVEN** a queue containing 2 `standby` items, 1 `released` item, and 1 `discarded` item
- **WHEN** `List(DeliveryStatusStandby)` is called
- **THEN** it MUST return only the 2 `standby` items sorted by creation time (FIFO).

---

## 3. Delivery Engine & Gate Verification

### Requirement: CAS Receipt Pre-Release Verification
The `DeliveryEngine` MUST verify that the local git tree matches the approved review receipt hash before initiating remote transport.

#### Scenario: Releasing an approved, un-drifted slice
- **GIVEN** a queued delivery item whose branch tree matches its CAS receipt hash
- **WHEN** `Release(id)` is called
- **THEN** the engine MUST verify `GatePrePR` and `GatePrePush` pass cleanly
- **AND** the engine MUST push the branch to remote and create the pull request
- **AND** the item status MUST be marked as `released`.

#### Scenario: Releasing a drifted or unreviewed slice
- **GIVEN** a queued delivery item where local files were altered after review receipt issuance
- **WHEN** `Release(id)` is called
- **THEN** the CAS gate validation MUST fail with a content drift error
- **AND** remote push MUST NOT be executed
- **AND** the item status MUST be marked as `failed` with error details.

---

## 4. Stacked Branch Ordering

### Requirement: Topological Order Delivery
When releasing multiple queued slices that depend on each other, the engine MUST push branches in topological order (base branches before child branches).

#### Scenario: Batch releasing a stacked chain
- **GIVEN** PR1 targeting `main`, PR2 targeting `feature/pr1`, and PR3 targeting `feature/pr2`
- **WHEN** `ReleaseAll()` is executed
- **THEN** PR1 MUST be pushed and opened first
- **AND** PR2 MUST be pushed and opened second with `base = feature/pr1`
- **AND** PR3 MUST be pushed and opened third with `base = feature/pr2`.

---

## 5. CLI Interface

### Requirement: Human Inspection and Control
The CLI MUST provide commands for humans to list, inspect, release, and discard queued delivery items.

#### Scenario: Listing pending deliveries
- **WHEN** running `gaia delivery list`
- **THEN** it MUST display a table of all `standby` PRs with ID, Change Name, Branch, Base, and Created At.

#### Scenario: Releasing all pending deliveries
- **WHEN** running `gaia delivery release --all`
- **THEN** all `standby` items MUST be processed sequentially and reported with generated PR URLs.
