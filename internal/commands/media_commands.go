package commands

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

// AttachImage attaches an image from the local filesystem to the next agent turn.
func (d *Dispatcher) AttachImage(ctx context.Context, path string) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Image not found: %s", path)}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}
	const maxSize int64 = 20 * 1024 * 1024 // 20MB
	if info.Size() > maxSize {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("Image too large: %s (%.1f MB). Maximum is 20 MB.", path, float64(info.Size())/(1024*1024)),
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

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
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: fmt.Sprintf("Failed to read image: %v", err)}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	img := domain.ImageContent{
		MIMEType: mimeType,
		Data:     encoded,
	}
	d.brain.AddPendingImage(img)

	msg := domain.Message{
		Role:    domain.RoleSystem,
		Content: fmt.Sprintf("Image attached: %s (%s, %.1f KB). Send a message to include it in the conversation.", filepath.Base(path), mimeType, float64(info.Size())/1024),
	}
	_ = repo.SaveMessage(ctx, msg)
	return ui.Display(msg)
}

// PasteImage attaches an image from the system clipboard.
func (d *Dispatcher) PasteImage(ctx context.Context) error {
	repo := d.brain.Repo()
	ui := d.brain.UI()
	if repo == nil || ui == nil {
		return nil
	}

	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("gaia-paste-%d.png", time.Now().UnixNano()))

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
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
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	if err := cmd.Run(); err != nil {
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: fmt.Sprintf("No image found in clipboard, or clipboard tool is not installed. Error: %v", err),
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	info, err := os.Stat(tmpFile)
	if err != nil || info.Size() == 0 {
		_ = os.Remove(tmpFile)
		msg := domain.Message{
			Role:    domain.RoleSystem,
			Content: "No image found in clipboard.",
		}
		_ = repo.SaveMessage(ctx, msg)
		return ui.Display(msg)
	}

	err = d.AttachImage(ctx, tmpFile)
	_ = os.Remove(tmpFile)
	return err
}
