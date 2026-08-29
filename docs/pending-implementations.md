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

**Current state**: Not implemented.
**Note**: Requires TTS (text-to-speech) integration. Low priority for a programming-first agent.

---

## Tools & Skills

### `/kanban` — Project board from chat

**Current state**: Not implemented.
**Note**: Complex feature. Would need GitHub Issues integration or a local kanban data store.

---

## Messaging (Gateway)

Commands from Hermes that would apply to gateway mode (Telegram, Discord, etc.):

- `/sethome` — Set current chat as delivery home
- `/approve` — Approve pending dangerous command
- `/deny` — Deny pending dangerous command
- `/commands` — Browse all commands (paginated)
- `/restart` — Gracefully restart gateway
- `/update` — Update GAIA to latest version
- `/topic` — Multi-session DM mode (Telegram)

These require gateway integration and are lower priority than core features.
