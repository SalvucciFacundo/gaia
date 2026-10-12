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

## Messaging (Gateway)

Implemented gateway capabilities across Telegram, WhatsApp (Meta Cloud API), Discord, and Slack:

- [x] `/approve` & `/deny` — Interactive approval gates with inline keyboard buttons
- [x] `gaia update` — Self-updater CLI with SemVer, GitHub release download, and atomic binary upgrade
- [x] Multi-platform audio transcription and voice responses
- [x] Magic-byte media ingest validation
- [x] Multi-channel session manager

