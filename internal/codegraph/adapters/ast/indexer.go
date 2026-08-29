package ast

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"gaia/internal/codegraph/adapters/sqlite"
	"gaia/internal/codegraph/domain"
	"gaia/internal/codegraph/ports"
)

var _ ports.IndexerPort = (*Indexer)(nil)

// Indexer coordinates workspace file discovery, AST parsing, and SQLite persistence.
type Indexer struct {
	store  *sqlite.Store
	parser *Parser
}

// NewIndexer creates an Indexer bound to the SQLite store.
func NewIndexer(store *sqlite.Store) *Indexer {
	return &Indexer{
		store:  store,
		parser: NewParser(),
	}
}

// IndexWorkspace traverses workspacePath, computes file hashes, purges stale files, parses new/modified files, and persists in batch.
func (idx *Indexer) IndexWorkspace(ctx context.Context, workspacePath string) error {
	storedHashes, err := idx.store.GetFileHashes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stored hashes: %w", err)
	}

	discoveredFiles := make(map[string]string) // filePath -> hash

	err = filepath.WalkDir(workspacePath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".go") {
			hash, hErr := idx.parser.HashFile(path)
			if hErr != nil {
				return nil
			}
			discoveredFiles[path] = hash
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk workspace: %w", err)
	}

	// 1. Identify deleted files
	for storedPath := range storedHashes {
		if _, exists := discoveredFiles[storedPath]; !exists {
			if delErr := idx.store.DeleteStaleFile(ctx, storedPath); delErr != nil {
				return fmt.Errorf("failed to delete stale file %s: %w", storedPath, delErr)
			}
		}
	}

	// 2. Identify modified or new files
	filesToParse := make(map[string]string)
	for path, currentHash := range discoveredFiles {
		if oldHash, exists := storedHashes[path]; !exists || oldHash != currentHash {
			filesToParse[path] = currentHash
		}
	}

	if len(filesToParse) == 0 {
		return nil // nothing changed
	}

	// 3. For modified files, purge old records first
	for path := range filesToParse {
		if _, exists := storedHashes[path]; exists {
			if delErr := idx.store.DeleteStaleFile(ctx, path); delErr != nil {
				return fmt.Errorf("failed to clear modified file %s: %w", path, delErr)
			}
		}
	}

	// 4. Parse files and collect nodes and edges
	var allNodes []domain.SymbolNode
	var allEdges []domain.Edge
	validFiles := make(map[string]string)

	for path, hash := range filesToParse {
		// Calculate package path relative to workspace if possible
		relDir, rErr := filepath.Rel(workspacePath, filepath.Dir(path))
		var pkgOverride string
		if rErr == nil && relDir != "." && relDir != "" {
			pkgOverride = filepath.ToSlash(relDir)
		}

		res, pErr := idx.parser.ParseFile(path, pkgOverride)
		if pErr != nil {
			// Resilient to syntax/parse errors: log or ignore broken file and continue
			continue
		}

		validFiles[path] = hash
		allNodes = append(allNodes, res.Nodes...)
		allEdges = append(allEdges, res.Edges...)
	}

	// 5. Batch persist in SQLite transaction
	if len(validFiles) > 0 {
		if err := idx.store.SaveBatch(ctx, validFiles, allNodes, allEdges); err != nil {
			return fmt.Errorf("failed to save batch index: %w", err)
		}
	}

	return nil
}
