# Design: Vision Support

## Technical Approach

Add an `Images []ImageContent` field to `domain.Message` (additive, non-breaking). Provider adapters detect images and build multi-modal content blocks per provider format. `AttachImage` and `PasteImage` are reimplemented to load image data, attach it to a user message, save to repo, and enter the standard LLM loop. No interface changes — `LLMProvider.Chat/Stream` signatures stay identical since `Message` carries the images.

## Architecture Decisions

### Decision: Image data model

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `Images []ImageContent` on Message | Additive; no breakage to existing `Content string` consumers | **Chosen** |
| Change `Content` to `[]ContentBlock` | Cleaner long-term but breaks every message consumer, serializer, and test | Rejected |

`ImageContent` stores `MIMEType string` and `Data string` (base64-encoded). Base64 string (not `[]byte`) because Message is JSON-serialized to the repository — base64 string round-trips cleanly through JSON without custom marshalers.

Supported MIME types: `image/png`, `image/jpeg`, `image/webp`, `image/gif`. Max size: 20MB raw (before base64 expansion).

### Decision: Provider adapter strategy

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Each adapter checks `len(m.Images) > 0` | Adapter owns provider-specific formatting | **Chosen** |
| Central image-to-content converter in domain | Leaks provider format into core | Rejected |

- **OpenAI**: When `len(m.Images) > 0`, build `MultiContent` with `ChatMessagePart` slices — text part + `image_url` parts with `data:` URIs.
- **Anthropic**: When `len(m.Images) > 0`, build `[]ContentBlockParamUnion` with text block + `NewImageBlock` (base64 source).
- **Router**: No changes — delegates to active provider which handles images.

### Decision: Brain integration — pending images vs inline

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `pendingImages` buffer on Brain | `/image` loads and buffers; next `ProcessMessage` attaches to user msg | **Chosen** |
| `/image` immediately sends to LLM | Breaks conversational flow; user can't add a text prompt with the image | Rejected |

Flow: `/image <path>` → load + validate → store in `b.pendingImages` → show confirmation. Next `ProcessMessage` call attaches `pendingImages` to the user message, clears the buffer, and enters the standard LLM loop. `/paste` follows the same pattern.

## Data Flow

```
/image <path>                      /paste
     │                                │
     ▼                                ▼
AttachImage()                   PasteImage()
  │ read file                     │ clipboard read (platform)
  │ detect MIME                   │ save to temp file
  │ validate size (<20MB)         │ call AttachImage()
  │ base64 encode                 │
  │ store in pendingImages        │
  └──────────┬────────────────────┘
             ▼
     pendingImages buffer
             │
             ▼ (next user message)
     ProcessMessage(content)
       │ create userMsg with Images
       │ save to repo
       │ getHistory → []Message (with Images)
       │ provider.Stream(history)
       ▼
     Provider Adapter
       │ OpenAI: MultiContent parts
       │ Anthropic: ContentBlockParamUnion
       ▼
     LLM API → response
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/core/domain/models.go` | Modify | Add `ImageContent` struct + `Images []ImageContent` field to `Message` |
| `internal/core/kernel.go` | Modify | Add `pendingImages` field to Brain; reimplement `AttachImage`, `PasteImage`; attach pending images in `ProcessMessage` |
| `internal/adapters/llm/openai.go` | Modify | Update `buildRequest` to emit `MultiContent` when message has Images |
| `internal/adapters/llm/anthropic.go` | Modify | Update `buildParams` to emit image blocks when message has Images |
| `internal/core/ports/ports.go` | No change | Interface stays — Message carries images |
| `internal/adapters/llm/router.go` | No change | Delegates to provider |

## Interfaces / Contracts

```go
// domain/models.go
type ImageContent struct {
    MIMEType string `json:"mime_type"` // "image/png", "image/jpeg", "image/webp", "image/gif"
    Data     string `json:"data"`      // base64-encoded image data
}

type Message struct {
    // ... existing fields ...
    Images []ImageContent `json:"images,omitempty"`
}
```

Validation rules:
- MIME type must be one of: `image/png`, `image/jpeg`, `image/webp`, `image/gif`
- Raw image size must be ≤ 20MB (checked before base64 encoding)
- Invalid files return a user-facing error message via `b.ui.Display`

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | `ImageContent` validation (size, MIME) | Table-driven tests with valid/invalid inputs |
| Unit | OpenAI `buildRequest` with images | Mock messages with Images, assert `MultiContent` JSON shape |
| Unit | Anthropic `buildParams` with images | Mock messages with Images, assert `ContentBlockParamUnion` shape |
| Integration | `/image` full flow | Create temp PNG, call `AttachImage`, verify `pendingImages` populated |
| Integration | `ProcessMessage` with pending images | Verify images attached to user message and forwarded to provider |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary. This change is confined to domain model extension and LLM API payload formatting.

## Migration / Rollout

No migration required. The `Images` field is `omitempty` — existing messages deserialize without it. No database schema changes (messages stored as JSON). No feature flag needed — `/image` and `/paste` simply start working instead of showing stub messages.

## Open Questions

- [ ] Clipboard library choice: `golang.design/x/clipboard` (CGo) vs `atotto/clipboard` (pure Go, limited image support) vs platform-specific shell commands — needs spike to confirm image support on all three platforms.
