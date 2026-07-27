# Tasks: Vision Support

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~315 |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | force-chained |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Domain types + provider adapters | PR 1 | `go test ./internal/adapters/llm/...` | N/A — pure data + formatting | Revert ImageContent + Images field |
| 2 | Brain impl + integration | PR 1 | `go test ./internal/core/... -run TestAttachImage` | Create temp PNG, call /image | Revert pendingImages + AttachImage |

## Phase 1: Domain Types

- [x] 1.1 Add `ImageContent` struct (`MIMEType string`, `Data string` base64) and `Images []ImageContent` field with `json:"images,omitempty"` to `domain.Message` in `internal/core/domain/models.go`

## Phase 2: Provider Adapters

- [x] 2.1 Update `OpenAIAdapter.buildRequest` — when `len(m.Images) > 0`, build `MultiContent` parts with `openai.ChatMessagePartTypeImageURL` using `data:{mime};base64,{data}` URIs; existing plain-text messages keep `Content` string unchanged
- [x] 2.2 Update `AnthropicAdapter.buildParams` — when `len(m.Images) > 0`, append `anthropic.NewImageBlockBase64` to the user message content blocks; existing text-only messages use `NewTextBlock` as before

## Phase 3: Brain Implementation

- [x] 3.1 Add `pendingImages []domain.ImageContent` field to `Brain` struct in `internal/core/kernel.go`
- [x] 3.2 Reimplement `Brain.AttachImage` — stat file, validate size ≤ 20MB, detect MIME from extension (`.png`/`.jpg`/`.jpeg`/`.webp`), read + base64 encode, store in `pendingImages`, display confirmation
- [x] 3.3 Reimplement `Brain.PasteImage` — read clipboard via platform command (Windows: `powershell Get-Clipboard`, macOS: `pngpaste`, Linux: `xclip`), save to temp file, call `AttachImage`; return clear error on unsupported platform or empty clipboard
- [x] 3.4 Wire pending images in `ProcessMessage` — before creating the user `Message`, prepend `pendingImages` to `Images` field, then clear buffer; images travel transparently to provider via the existing `getHistory` → provider call

## Phase 4: Tests

- [x] 4.1 Write unit tests in `domain/models_test.go` for `ImageContent` validation — valid MIME types, invalid formats (GIF/BMP rejected), size boundary at 20MB
- [x] 4.2 Write unit tests in `llm/` for provider image formatting — OpenAI `buildRequest` emits `MultiContent` with `image_url` parts, Anthropic `buildParams` emits `NewImageBlockBase64` with base64 source; assert both Go-level struct fields and JSON shape
- [x] 4.3 Write integration test in `kernel_test.go` for `/image` flow — create temp valid/invalid image files, call `AttachImage`, verify `pendingImages` populated; verify `ProcessMessage` attaches images to user message and clears buffer
