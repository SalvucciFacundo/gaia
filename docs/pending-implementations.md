# Pending Implementations

Features with partial/stub implementations that need to be completed.

---

## Info & System

### `/image <path>` — Attach image for vision processing

**Current state**: Implemented.
**Location**: `internal/core/kernel.go` — `Brain.AttachImage()`
- [x] Integrate with LLM providers supporting vision (GPT-4o, Claude Sonnet, Gemini)
- [x] Read image bytes, encode base64 and pass in Chat request
- [x] Support common formats: PNG, JPEG, WebP
- [x] Size limit handling (≤20MB)

### `/paste` — Attach clipboard image

**Current state**: Implemented.
**Location**: `internal/core/kernel.go` — `Brain.PasteImage()`
- [x] Read image from system clipboard (PowerShell on Windows, `pngpaste` on macOS, `xclip` on Linux)
- [x] Save clipboard to temp file and attach via `AttachImage`

### `/credits` — Show credit/usage balance

**Current state**: Shows links to provider dashboards.
**Location**: `internal/core/kernel.go` — `Brain.CreditsInfo()`
**Brownie points**:
- [ ] Query OpenRouter API for credit balance if provider is OpenRouter
- [ ] Show cached balance from last API response headers (many providers return `x-ratelimit-remaining`)

### `/billing` — Billing management

**Current state**: Shows links to provider billing dashboards.
**Location**: `internal/core/kernel.go` — `Brain.BillingInfo()`
**Brownie points**:
- [ ] Same as `/credits` — provider-specific API queries

---

## Configuration

### `/fast <mode>` — Toggle fast mode

**Current state**: Implemented.
**Location**: `internal/core/kernel.go` — `Brain.FastMode()`
- [x] Store "fast model" override in Brain (`gpt-4o-mini`)
- [x] `/fast on` saves current model, switches to fast model
- [x] `/fast off` restores original model
- [x] `/fast <model>` allows custom fast model switch

### `/busy <mode>` — Control Enter behavior while agent works

**Current state**: Implemented.
**Location**: `internal/core/kernel.go` — `Brain.BusyMode()`
- [x] Modes: `queue` (default — queues messages), `steer` (injects as steer), `ignore` (discards input while busy)

### `/voice <on|off>` — Toggle voice mode

**Current state**: Implemented.
**Location**: `internal/adapters/audio/` & `internal/gateway/multiplexer.go`
- [x] Inbound audio transcription with OpenAI Whisper STT
- [x] Outbound audio voice note synthesis with ElevenLabs and OpenAI TTS
- [x] Bi-directional voice support over Telegram and WhatsApp

---

## Tools & Skills

### `/kanban` — Project board from chat

**Current state**: Planned.
**Note**: Will integrate with GitHub Projects / Issues and local state.

---

## Completed Core Architectural Upgrades & Refactors

The following major diagnostic items and architectural debts were completed, verified, and merged:

- [x] **Tamper Detection in Backup/Restore (BUG-1)**: Full gzip stream checksum verification (`io.Copy(io.Discard, gr)`), path traversal protection, manifest checks, and 0600 file modes (`internal/backup/restore.go`).
- [x] **Reasoning Effort & Persona Stubs Wired (BUG-2)**: Functional options `WithReasoningEffort` and dynamic persona injection in `ports.go`, `openai.go`, and `copilot_client.go`.
- [x] **Judgment Day Adversarial Dual-Review (INCOMPLETE-1)**: End-to-end CLI execution with `--judgment-day`, blind dual judges, compare findings, surgical fix cycles, and CAS receipt storage (`cmd/gaia/review.go`, `internal/review/judgment/`).
- [x] **Auto-Create Skill in Learning Loop (INCOMPLETE-2)**: Subagents trigger skill creation upon reaching execution threshold via `Spawner.SetOnLearningThreshold` callback (`internal/agent/spawner.go`).
- [x] **Native Mnemosyne Shutdown Persistence (INCOMPLETE-3)**: Replaced external cosmetic Engram stubs with native SQLite session summaries.
- [x] **`kernel.go` God Object Modularization (DEBT-1)**: Reduced `kernel.go` from ~4,028 to 1,821 lines by extracting 7 domain command files into `internal/core/`.
- [x] **Context Cancellation & Signal Propagation (DEBT-2)**: Wired `rootCtx` from OS signals (SIGINT/SIGTERM) through TUI, wizard, gateways, and cron routines.
- [x] **Production Error Visibility (DEBT-3)**: Surfaced asynchronous execution errors into the TUI chat history (`❌ Error: ...`) and added structured error handling in web server and reviewer ops.
- [x] **Safe CodeAct `Find()` Execution (DEBT-4)**: Captured `filepath.Walk` errors in `GaiaSDK.Find` and added support for relative path glob matching.
- [x] **Canonical Browser Configuration (DEBT-5)**: Consolidated browser automation under `gateway.browser_tools`.
- [x] **$O(1)$ Dynamic Loader Tool Validation (DEBT-6)**: Replaced $O(n \times m)$ nested loops with map lookups in `cmd/gaia/main.go`.

