# Proposal: Vision Support

## Intent

Enable `/image <path>` and `/paste` commands to send images to the LLM for analysis. Currently these are stubs that validate file existence but never transmit image data to the model.

## Scope

### In Scope
- Image loading from disk (base64 encoding)
- Image pasting from clipboard (platform-specific: Windows/macOS/Linux)
- Multi-modal LLM calls (text + image to provider)
- Reimplement `Brain.AttachImage()` and `Brain.PasteImage()` stubs
- Update provider adapters (OpenAI, Anthropic) to handle image content blocks

### Out of Scope
- Image generation (separate feature)
- Video analysis
- TUI image rendering/preview
- Image URL support (base64 only for v1)

## Capabilities

### New Capabilities
- `vision-support`: Image loading, clipboard paste, and multi-modal LLM integration

### Modified Capabilities
None

## Approach

**Option A: Add `Images []ImageContent` field to `domain.Message`**

Extend the Message struct with an optional `Images` field. Provider adapters check `len(m.Images) > 0` and build multi-modal content blocks accordingly.

**Rationale**:
- No breaking changes to existing `Content string` field
- Clean separation: text content + image attachments
- Provider adapters can ignore images if unsupported (graceful degradation)
- Brain populates images before passing to LLM

**Implementation**:
1. Add `ImageContent` struct to `domain/models.go`:
   ```go
   type ImageContent struct {
       Data     []byte `json:"data"`      // base64-encoded image
       MimeType string `json:"mime_type"` // e.g., "image/png"
   }
   ```
2. Add `Images []ImageContent` field to `domain.Message`
3. Update `LLMProvider` interface — no signature change, but adapters must handle `Message.Images`
4. OpenAI adapter: convert `Images` to `[]openai.ChatMessagePart` with `Type: "image_url"`
5. Anthropic adapter: convert `Images` to `[]anthropic.ContentBlockParamUnion` with `NewImageBlock`
6. Reimplement `Brain.AttachImage()`:
   - Read file from disk
   - Detect MIME type via `http.DetectContentType`
   - Validate size (< 20MB)
   - Encode base64, store in `ImageContent`
   - Attach to next user message
7. Reimplement `Brain.PasteImage()`:
   - Platform-specific clipboard read (Windows: `GetClipboardData`, macOS: `NSPasteboard`, Linux: `xclip`)
   - Save to temp file, then call `AttachImage()`
8. Update `ProcessMessage` to attach pending images to the user message before LLM call

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/core/domain/models.go` | Modified | Add `ImageContent` struct and `Images` field to `Message` |
| `internal/core/ports/ports.go` | Modified | Document that adapters must handle `Message.Images` |
| `internal/adapters/llm/openai.go` | Modified | Convert `Images` to OpenAI multi-modal format |
| `internal/adapters/llm/anthropic.go` | Modified | Convert `Images` to Anthropic multi-modal format |
| `internal/core/kernel.go` | Modified | Reimplement `AttachImage()` and `PasteImage()` |
| `internal/adapters/llm/router.go` | None | Pass-through — no changes needed |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Provider support varies (some models don't support vision) | Medium | Graceful degradation: skip images if provider doesn't support, show warning |
| Clipboard reading is platform-specific | High | Use existing cross-platform libraries (`golang.design/x/clipboard` or `atotto/clipboard`) |
| Large images (>20MB) exceed provider limits | Medium | Validate size before encoding, reject with clear error message |
| Base64 encoding increases payload size ~33% | Low | Acceptable for v1; document limits in `/image` help text |

## Rollback Plan

Revert commit. The `Images` field is additive — removing it restores stub behavior. No database schema changes.

## Dependencies

- `golang.design/x/clipboard` or `atotto/clipboard` for cross-platform clipboard access
- Provider SDKs already support multi-modal (go-openai, anthropic-sdk-go)

## Success Criteria

- [ ] `/image <path>` loads image, sends to LLM, receives analysis
- [ ] `/paste` reads clipboard image, sends to LLM, receives analysis
- [ ] Works with OpenAI (gpt-4o) and Anthropic (claude-3.5-sonnet)
- [ ] Rejects images > 20MB with clear error
- [ ] Gracefully handles providers without vision support (warning, no crash)
- [ ] Existing text-only conversations unaffected
