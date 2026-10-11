package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	// DefaultMaxImageSize is the default maximum allowed size for images (10MB).
	DefaultMaxImageSize int64 = 10 * 1024 * 1024

	// DefaultMaxAudioSize is the default maximum allowed size for audio files (25MB).
	DefaultMaxAudioSize int64 = 25 * 1024 * 1024
)

var (
	// ErrMediaTooLarge is returned when media payload exceeds configured size limit.
	ErrMediaTooLarge = errors.New("media exceeds maximum allowed size")

	// ErrUnsupportedMediaType is returned when media MIME type is not allowed.
	ErrUnsupportedMediaType = errors.New("unsupported or disallowed media type")

	// ErrEmptyMedia is returned when media response body is empty.
	ErrEmptyMedia = errors.New("empty media payload")
)

// Allowed image MIME types.
var allowedImageMimes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
	"image/gif":  true,
}

// Allowed audio MIME types.
var allowedAudioMimes = map[string]bool{
	"audio/ogg":   true,
	"audio/wav":   true,
	"audio/wave":  true,
	"audio/x-wav": true,
	"audio/mpeg":  true,
	"audio/mp3":   true,
	"audio/mp4":   true,
	"audio/x-m4a": true,
	"audio/aac":   true,
}

// IsAllowedImageMime reports whether the given MIME type is a supported image format.
func IsAllowedImageMime(mime string) bool {
	clean := strings.ToLower(strings.TrimSpace(mime))
	if idx := strings.Index(clean, ";"); idx != -1 {
		clean = clean[:idx]
	}
	return allowedImageMimes[clean]
}

// IsAllowedAudioMime reports whether the given MIME type is a supported audio format.
func IsAllowedAudioMime(mime string) bool {
	clean := strings.ToLower(strings.TrimSpace(mime))
	if idx := strings.Index(clean, ";"); idx != -1 {
		clean = clean[:idx]
	}
	return allowedAudioMimes[clean]
}

// MediaDownloader downloads and validates media from URLs with size caps and MIME validation.
type MediaDownloader struct {
	client       *http.Client
	maxImageSize int64
	maxAudioSize int64
}

// NewMediaDownloader creates a new MediaDownloader with configured or default size limits.
func NewMediaDownloader(client *http.Client, maxImageSize, maxAudioSize int64) *MediaDownloader {
	if client == nil {
		client = http.DefaultClient
	}
	if maxImageSize <= 0 {
		maxImageSize = DefaultMaxImageSize
	}
	if maxAudioSize <= 0 {
		maxAudioSize = DefaultMaxAudioSize
	}
	return &MediaDownloader{
		client:       client,
		maxImageSize: maxImageSize,
		maxAudioSize: maxAudioSize,
	}
}

// DownloadMedia downloads and validates media from URL with optional auth headers.
func (d *MediaDownloader) DownloadMedia(ctx context.Context, url string, authHeader string, isAudio bool) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating media request: %w", err)
	}

	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetching media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("media fetch returned status %d", resp.StatusCode)
	}

	maxSize := d.maxImageSize
	if isAudio {
		maxSize = d.maxAudioSize
	}

	// Read limited bytes + 1 to detect oversize
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSize+1))
	if err != nil {
		return nil, "", fmt.Errorf("reading media body: %w", err)
	}

	if int64(len(data)) > maxSize {
		return nil, "", ErrMediaTooLarge
	}

	if len(data) == 0 {
		return nil, "", ErrEmptyMedia
	}

	// Validate content type
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	if isAudio {
		if !IsAllowedAudioMime(contentType) {
			return nil, "", fmt.Errorf("%w: %s", ErrUnsupportedMediaType, contentType)
		}
	} else {
		if !IsAllowedImageMime(contentType) {
			return nil, "", fmt.Errorf("%w: %s", ErrUnsupportedMediaType, contentType)
		}
	}

	return data, contentType, nil
}
