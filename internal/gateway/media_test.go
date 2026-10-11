package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsAllowedMime(t *testing.T) {
	if !IsAllowedImageMime("image/png") || !IsAllowedImageMime("image/jpeg; charset=utf-8") {
		t.Error("expected valid image MIME to be allowed")
	}
	if IsAllowedImageMime("application/pdf") {
		t.Error("expected pdf to be disallowed as image")
	}

	if !IsAllowedAudioMime("audio/ogg") || !IsAllowedAudioMime("audio/mp3") {
		t.Error("expected valid audio MIME to be allowed")
	}
	if IsAllowedAudioMime("video/mp4") {
		t.Error("expected video to be disallowed as audio")
	}
}

func TestMediaDownloader_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("fake-png-data"))
	}))
	defer server.Close()

	d := NewMediaDownloader(nil, 1024*1024, 1024*1024)
	data, mime, err := d.DownloadMedia(context.Background(), server.URL, "", false)
	if err != nil {
		t.Fatalf("DownloadMedia error: %v", err)
	}
	if string(data) != "fake-png-data" || mime != "image/png" {
		t.Errorf("unexpected result: %q, %s", string(data), mime)
	}
}

func TestMediaDownloader_OversizeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("12345678901234567890"))
	}))
	defer server.Close()

	d := NewMediaDownloader(nil, 10, 10) // 10 bytes cap
	_, _, err := d.DownloadMedia(context.Background(), server.URL, "", false)
	if err != ErrMediaTooLarge {
		t.Errorf("expected ErrMediaTooLarge, got %v", err)
	}
}
