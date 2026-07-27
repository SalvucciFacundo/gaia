# Vision Support Specification

## Purpose

Enable multi-modal image analysis by allowing users to attach images from disk or clipboard and send them to LLM providers for vision processing.

## Requirements

### Requirement: ImageContent Type

The system MUST define an `ImageContent` struct in `domain/models.go` with:
- `Data []byte` — base64-encoded image data
- `MimeType string` — MIME type (image/png, image/jpeg, image/webp)

The system MUST add an `Images []ImageContent` field to `domain.Message`.

The system MUST reject images exceeding 20MB with error: "Image exceeds 20MB limit".

The system MUST support only PNG, JPEG, and WebP formats. Unsupported formats MUST return error: "Unsupported image format. Supported: PNG, JPEG, WebP".

#### Scenario: Valid image attachment

- GIVEN a user provides a valid PNG image path
- WHEN `Brain.AttachImage()` is called
- THEN the system reads the file, validates size <20MB, detects MIME type as "image/png", base64-encodes the data, and creates a `Message` with `Images` field populated

#### Scenario: Image exceeds size limit

- GIVEN a user provides an image file larger than 20MB
- WHEN `Brain.AttachImage()` is called
- THEN the system returns error "Image exceeds 20MB limit" without processing

#### Scenario: Unsupported image format

- GIVEN a user provides a GIF or BMP file
- WHEN `Brain.AttachImage()` is called
- THEN the system returns error "Unsupported image format. Supported: PNG, JPEG, WebP"

### Requirement: Provider Image Handling

Each LLM provider adapter MUST handle `Message.Images` when present:

**OpenAI**: Convert `Images` to content blocks with `type: "image_url"` and `url: "data:image/{mime};base64,{data}"`.

**Anthropic**: Convert `Images` to content blocks with `type: "image"` and `source.type: "base64"`, `source.media_type`, and `source.data`.

**Gemini**: Convert `Images` to `inlineData` parts with `mimeType` and `data` fields.

**Router**: Pass `Images` through to the active provider without modification.

**Fallback**: If a provider does not support vision, it MUST return error: "Current provider does not support image analysis".

#### Scenario: OpenAI vision request

- GIVEN a `Message` with one PNG image in `Images` field
- WHEN OpenAI adapter processes the message
- THEN the adapter formats the image as a content block with `type: "image_url"` and `url: "data:image/png;base64,{base64data}"`

#### Scenario: Anthropic vision request

- GIVEN a `Message` with one JPEG image in `Images` field
- WHEN Anthropic adapter processes the message
- THEN the adapter formats the image as a content block with `type: "image"`, `source.type: "base64"`, `source.media_type: "image/jpeg"`, and `source.data` containing base64 data

#### Scenario: Provider without vision support

- GIVEN a provider that does not support image analysis
- WHEN a `Message` with `Images` is sent
- THEN the provider returns error "Current provider does not support image analysis"

### Requirement: Brain.AttachImage

The system MUST implement `Brain.AttachImage(ctx, path)` that:
1. Stats the file to verify existence and size
2. Rejects files >20MB
3. Reads the file contents
4. Detects MIME type from file extension (`.png`, `.jpg`, `.jpeg`, `.webp`)
5. Base64-encodes the data
6. Creates a `Message` with `Role: user`, `Content: ""`, and `Images: []ImageContent{{Data, MimeType}}`
7. Processes the message through the normal LLM loop

#### Scenario: Attach valid image

- GIVEN a valid PNG file at `/path/to/image.png` (5MB)
- WHEN user executes `/image /path/to/image.png`
- THEN the system loads the image, creates a user message with the image attached, and sends it to the LLM for analysis

#### Scenario: Attach non-existent file

- GIVEN a path that does not exist
- WHEN user executes `/image /nonexistent.png`
- THEN the system returns error "Image not found: /nonexistent.png"

### Requirement: Brain.PasteImage

The system MUST implement `Brain.PasteImage(ctx)` that reads an image from the system clipboard:

**Windows**: Execute `powershell -Command "Get-Clipboard -Format Image"` and save output to a temporary PNG file.

**macOS**: Execute `pngpaste` command and save output to a temporary PNG file.

**Linux**: Execute `xclip -selection clipboard -t image/png -o` and save output to a temporary PNG file.

**Fallback**: If the platform is unsupported or clipboard read fails, return error: "No image found in clipboard" or "Clipboard image not supported on this platform".

After saving to temp file, the system MUST call `AttachImage()` with the temp file path.

#### Scenario: Paste from clipboard (Windows)

- GIVEN a user has copied an image to clipboard on Windows
- WHEN user executes `/paste`
- THEN the system reads the clipboard image via PowerShell, saves to temp PNG, calls `AttachImage()`, and sends to LLM

#### Scenario: Clipboard empty

- GIVEN the clipboard contains no image data
- WHEN user executes `/paste`
- THEN the system returns error "No image found in clipboard"

#### Scenario: Unsupported platform

- GIVEN the system is running on an unsupported OS
- WHEN user executes `/paste`
- THEN the system returns error "Clipboard image not supported on this platform"

### Requirement: Error Handling

The system MUST provide clear, actionable error messages for all failure modes:

- File not found: "Image not found: {path}"
- File too large: "Image exceeds 20MB limit"
- Unsupported format: "Unsupported image format. Supported: PNG, JPEG, WebP"
- Provider no vision: "Current provider does not support image analysis"
- Clipboard empty: "No image found in clipboard"
- Clipboard unsupported: "Clipboard image not supported on this platform"

#### Scenario: All error paths covered

- GIVEN any of the failure conditions above
- WHEN the operation is attempted
- THEN the system returns the corresponding error message and does not crash or corrupt state
