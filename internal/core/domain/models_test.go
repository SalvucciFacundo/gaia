package domain

import (
	"testing"
)

func TestImageContent_Validation(t *testing.T) {
	tests := []struct {
		name    string
		mime    string
		data    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid PNG",
			mime:    "image/png",
			data:    "aGVsbG8=",
			wantErr: false,
		},
		{
			name:    "valid JPEG",
			mime:    "image/jpeg",
			data:    "aGVsbG8=",
			wantErr: false,
		},
		{
			name:    "valid WEBP",
			mime:    "image/webp",
			data:    "aGVsbG8=",
			wantErr: false,
		},
		{
			name:    "invalid MIME GIF",
			mime:    "image/gif",
			data:    "aGVsbG8=",
			wantErr: true,
			errMsg:  "unsupported MIME type",
		},
		{
			name:    "invalid MIME BMP",
			mime:    "image/bmp",
			data:    "aGVsbG8=",
			wantErr: true,
			errMsg:  "unsupported MIME type",
		},
		{
			name:    "empty MIME",
			mime:    "",
			data:    "aGVsbG8=",
			wantErr: true,
			errMsg:  "MIME type is required",
		},
		{
			name:    "empty data",
			mime:    "image/png",
			data:    "",
			wantErr: true,
			errMsg:  "image data is required",
		},
		{
			name:    "invalid base64",
			mime:    "image/png",
			data:    "!!!not-base64!!!",
			wantErr: true,
			errMsg:  "invalid base64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateImageContent(ImageContent{MIMEType: tt.mime, Data: tt.data})
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestImageContent_SizeBoundary(t *testing.T) {
	// Generate base64 data at exactly 20MB boundary for testing.
	// 20MB = 20971520 bytes. Base64 encoding adds ~33% overhead,
	// so pre-encoded data of 20MB means the raw bytes were ~15MB.
	// For validation we check the decoded size.

	// 1-byte image (valid base64 "aA==" → "h")
	small := ImageContent{MIMEType: "image/png", Data: "aA=="}
	if err := ValidateImageContent(small); err != nil {
		t.Errorf("small image should pass: %v", err)
	}

	// oversized: create a dummy data string that would exceed 20MB decoded.
	// 15MB raw → ~20MB base64 encoded. We create a string just over that.
	// This tests the size check fires correctly.
	oversized := ImageContent{
		MIMEType: "image/png",
		// Valid base64 with raw size ~21MB.
		Data: makeOversizedBase64(21 * 1024 * 1024),
	}
	if err := ValidateImageContent(oversized); err == nil {
		t.Error("expected error for oversized image, got nil")
	}
}

// makeOversizedBase64 creates a valid base64 string that decodes
// to approximately targetSize bytes.
func makeOversizedBase64(targetSize int) string {
	// base64: every 4 chars → 3 bytes.
	// To get ~targetSize decoded bytes, we need targetSize/3*4 chars.
	numTriplets := targetSize / 3
	buf := make([]byte, numTriplets*4)
	for i := range buf {
		buf[i] = 'A' // 'A' is valid base64
	}
	return string(buf)
}

func TestMessage_WithImages(t *testing.T) {
	img := ImageContent{MIMEType: "image/png", Data: "aGVsbG8="}
	msg := Message{
		ID:      "1",
		Role:    RoleUser,
		Content: "Look at this",
		Images:  []ImageContent{img},
	}

	if len(msg.Images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(msg.Images))
	}
	if msg.Images[0].MIMEType != "image/png" {
		t.Errorf("expected image/png, got %s", msg.Images[0].MIMEType)
	}
	if msg.Images[0].Data != "aGVsbG8=" {
		t.Errorf("expected aGVsbG8=, got %s", msg.Images[0].Data)
	}
}

func TestMessage_ImagesOmitempty(t *testing.T) {
	msg := Message{
		ID:      "1",
		Role:    RoleUser,
		Content: "No images",
	}
	if msg.Images != nil {
		t.Errorf("expected nil Images, got %v", msg.Images)
	}
}
