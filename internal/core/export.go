package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gaia/internal/core/domain"
)

// MemoryExport exports knowledge graph facts to Obsidian-compatible markdown files.
func (b *Brain) MemoryExport(ctx context.Context) error {
	if b.kgStore == nil {
		msg := domain.Message{Role: domain.RoleSystem, Content: "No knowledge graph store configured."}
		b.repo.SaveMessage(ctx, msg)
		return b.ui.Display(msg)
	}

	topics, err := b.kgStore.GetAllTopics(ctx)
	if err != nil {
		return fmt.Errorf("memory export: %w", err)
	}

	exportDir := filepath.Join(".", "gaia-memory-export")
	os.MkdirAll(exportDir, 0755)

	count := 0
	for _, topic := range topics {
		facts, err := b.kgStore.GetFactsByTopic(ctx, topic)
		if err != nil {
			continue
		}

		fileName := strings.ReplaceAll(topic, " ", "-") + ".md"
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

	msg := domain.Message{
		Role: domain.RoleSystem,
		Content: fmt.Sprintf("Memory export complete: %d topics exported to %s/\nOpen the folder in Obsidian to browse.", count, exportDir),
	}
	b.repo.SaveMessage(ctx, msg)
	return b.ui.Display(msg)
}
