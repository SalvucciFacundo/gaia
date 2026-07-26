# Pending Implementations

Features with partial/stub implementations that need to be completed.

---

## Info & System

### `/image <path>` — Attach image for vision processing

**Current state**: Stub — validates file exists, shows "not yet implemented" message.
**Location**: `internal/core/kernel.go` — `Brain.AttachImage()`
**What's needed**:
- [ ] Integrate with an LLM provider that supports vision (GPT-4o, Claude Sonnet 4, Gemini)
- [ ] Read image bytes and pass as a vision content block in the Chat request
- [ ] Support common formats: PNG, JPEG, WebP
- [ ] Size limit handling (>20MB images should be rejected or resized)

### `/paste` — Attach clipboard image

**Current state**: Stub — shows "not yet implemented" message.
**Location**: `internal/core/kernel.go` — `Brain.PasteImage()`
**What's needed**:
- [ ] Read image from system clipboard (platform-specific: `powershell -command Get-Clipboard -Format Image` on Windows, `pngpaste` on macOS, `xclip` on Linux)
- [ ] Same vision integration as `/image` once implemented
- [ ] Fallback: save clipboard to temp file and call `/image` handler

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

**Current state**: Not implemented.
**Brownie points**:
- [ ] Store a "fast model" override in Brain (e.g., `claude-haiku` or `gpt-4o-mini`)
- [ ] `/fast on` saves current model, switches to fast model
- [ ] `/fast off` restores original model
- [ ] Configurable in `config.yaml` as `llm.fast_model`

### `/busy <mode>` — Control Enter behavior while agent works

**Current state**: Not implemented.
**Brownie points**:
- [ ] Modes: `queue` (default — queues messages), `steer` (injects as steer), `ignore` (discards input while busy)
- [ ] TUI checks this setting before dispatching Enter key

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
