package core

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gaia/internal/core/domain"
)

func (b *Brain) AttachImage(ctx context.Context, path string) error {
	// 1. Stat file, check exists and size < 20MB.
	info, err := os.Stat(path)
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Image not found: %s", path)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}
	const maxSize int64 = 20 * 1024 * 1024 // 20MB
	if info.Size() > maxSize {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Image too large: %s (%.1f MB). Maximum is 20 MB.", path, float64(info.Size())/(1024*1024)),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	// 2. Detect MIME from extension.
	ext := strings.ToLower(filepath.Ext(path))
	var mimeType string
	switch ext {
	case ".png":
		mimeType = "image/png"
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".webp":
		mimeType = "image/webp"
	default:
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Unsupported image format: %s. Supported: png, jpg, jpeg, webp.", ext),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	// 3. Read file bytes.
	data, err := os.ReadFile(path)
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Failed to read image: %v", err)}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	// 4. Base64 encode.
	encoded := base64.StdEncoding.EncodeToString(data)

	// 5. Create ImageContent and store.
	img := domain.ImageContent{
		MIMEType: mimeType,
		Data:     encoded,
	}
	b.pendingImages = append(b.pendingImages, img)

	// 6. Display confirmation.
	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Image attached: %s (%s, %.1f KB). Send a message to include it in the conversation.", filepath.Base(path), mimeType, float64(info.Size())/1024),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}

// PasteImage attaches an image from the system clipboard.
// It runs a platform-specific command to extract the clipboard image,
// saves it to a temp file, calls AttachImage, and cleans up.
func (b *Brain) PasteImage(ctx context.Context) error {
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("gaia-paste-%d.png", time.Now().UnixNano()))

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// PowerShell: copy clipboard image to file.
		cmd = exec.Command("powershell", "-Command",
			fmt.Sprintf(`Add-Type -AssemblyName System.Drawing; $img = [System.Windows.Forms.Clipboard]::GetImage(); if ($img -eq $null) { exit 1 }; $img.Save('%s', [System.Drawing.Imaging.ImageFormat]::Png)`, tmpFile))
	case "darwin":
		cmd = exec.Command("pngpaste", tmpFile)
	case "linux":
		cmd = exec.Command("sh", "-c", fmt.Sprintf("xclip -selection clipboard -t image/png -o > %s", tmpFile))
	default:
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Clipboard image paste is not supported on %s.", runtime.GOOS),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	if err := cmd.Run(); err != nil {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("No image found in clipboard, or clipboard tool is not installed. Error: %v", err),
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	// Check that the temp file was actually created and is non-empty.
	info, err := os.Stat(tmpFile)
	if err != nil || info.Size() == 0 {
		os.Remove(tmpFile)
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "No image found in clipboard.",
		}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	// Call AttachImage with the temp file path, then clean up.
	err = b.AttachImage(ctx, tmpFile)
	os.Remove(tmpFile)
	return err
}
