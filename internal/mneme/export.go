package mneme

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gaia/internal/core/ports"
)

// ExportKnowledgeGraph exports knowledge graph facts from a KnowledgeGraphStore
// to Obsidian-compatible markdown files in exportDir (default: ./gaia-memory-export).
func ExportKnowledgeGraph(ctx context.Context, kgStore ports.KnowledgeGraphStore, exportPath ...string) (int, string, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return 0, "", err
		}
	}

	if kgStore == nil {
		return 0, "", fmt.Errorf("no knowledge graph store configured")
	}

	topics, err := kgStore.GetAllTopics(ctx)
	if err != nil {
		return 0, "", fmt.Errorf("memory export: %w", err)
	}

	exportDir := "./gaia-memory-export"
	if len(exportPath) > 0 && exportPath[0] != "" {
		exportDir = filepath.Clean(exportPath[0])
	}
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return 0, "", fmt.Errorf("memory export create dir: %w", err)
	}

	count := 0
	for _, topic := range topics {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return count, exportDir, err
			}
		}

		facts, err := kgStore.GetFactsByTopic(ctx, topic)
		if err != nil {
			continue
		}

		safeTopic := filepath.Base(filepath.Clean(strings.ReplaceAll(topic, " ", "-")))
		if safeTopic == "." || safeTopic == "/" || safeTopic == ".." {
			continue
		}
		fileName := safeTopic + ".md"
		filePath := filepath.Join(exportDir, fileName)

		var sb strings.Builder
		sb.WriteString("---\n")
		sb.WriteString(fmt.Sprintf("created: %s\n", time.Now().Format("2006-01-02")))
		sb.WriteString("source: gaia-memory-export\n")
		sb.WriteString("---\n\n")
		sb.WriteString(fmt.Sprintf("# %s\n\n", topic))

		for _, f := range facts {
			preview := f.Fact
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			sb.WriteString(fmt.Sprintf("- %s _(by %s)_\n", preview, f.SourceAgent))
		}

		if err := os.WriteFile(filePath, []byte(sb.String()), 0644); err != nil {
			continue
		}
		count++
	}

	return count, exportDir, nil
}
