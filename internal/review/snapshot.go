package review

import (
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"strings"

	"gaia/internal/modules/security"
)

// FileSnapshot holds the normalized content and hash of a single file.
type FileSnapshot struct {
	Path    string // relative path within the project
	Content string // LF-normalized content
	Hash    string // SHA256 hex of the normalized content
}

// SnapshotFiles reads and normalizes the given files relative to
// projectRoot. It validates each path with security.ValidatePath
// and normalizes all line endings to LF before hashing.
func SnapshotFiles(projectRoot string, files []string) ([]FileSnapshot, error) {
	if projectRoot == "" {
		return nil, fmt.Errorf("project root is required")
	}
	if len(files) == 0 {
		return nil, nil
	}

	snapshots := make([]FileSnapshot, 0, len(files))
	for _, f := range files {
		// Validate path is within project root.
		absPath, err := security.ValidatePath(projectRoot, f)
		if err != nil {
			return nil, fmt.Errorf("snapshot %s: %w", f, err)
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("snapshot %s: %w", f, err)
		}

		// Normalize CRLF → LF.
		content := strings.ReplaceAll(string(data), "\r\n", "\n")
		// Normalize lone CR → LF.
		content = strings.ReplaceAll(content, "\r", "\n")

		hash := sha256Hex(content)
		snapshots = append(snapshots, FileSnapshot{
			Path:    f,
			Content: content,
			Hash:    hash,
		})
	}

	return snapshots, nil
}

// ComputeSnapshotHash produces a deterministic SHA256 hash over a set
// of file snapshots. Snapshots are sorted by path, then each file's
// "path\ncontent\n" is written to the hash stream. The result is
// prefixed with "sha256:".
func ComputeSnapshotHash(snapshots []FileSnapshot) string {
	if len(snapshots) == 0 {
		return "sha256:" + sha256Hex("")
	}

	// Sort by path for deterministic ordering.
	sorted := make([]FileSnapshot, len(snapshots))
	copy(sorted, snapshots)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})

	h := sha256.New()
	for _, s := range sorted {
		fmt.Fprintf(h, "%s\n%s\n", s.Path, s.Content)
	}
	return "sha256:" + fmt.Sprintf("%x", h.Sum(nil))
}

// sha256Hex returns the SHA256 hex digest of content.
func sha256Hex(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:])
}
