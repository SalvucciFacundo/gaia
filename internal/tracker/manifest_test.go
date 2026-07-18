package tracker

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

)

func TestLoadManifest_FileNotFound(t *testing.T) {
	m, err := LoadManifest(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if m.Version != 1 {
		t.Errorf("expected version 1, got %d", m.Version)
	}
	if len(m.Features) != 0 {
		t.Errorf("expected empty features, got %d", len(m.Features))
	}
}

func TestLoadManifest_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	os.WriteFile(path, []byte(`version: 1
features:
  - upstream_feature: "streaming-tools"
    status: "ported"
    notes: "Done"
`), 0644)

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(m.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(m.Features))
	}
	if m.Features[0].UpstreamFeature != "streaming-tools" {
		t.Errorf("expected 'streaming-tools', got %q", m.Features[0].UpstreamFeature)
	}
	if m.Features[0].Status != StatusPorted {
		t.Errorf("expected 'ported', got %q", m.Features[0].Status)
	}
}

func TestLoadManifest_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	os.WriteFile(path, []byte("{{invalid"), 0644)

	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestSaveManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")

	m := &PortManifest{
		Version: 1,
		Features: []ManifestEntry{
			{UpstreamFeature: "test-feature", Status: StatusNotPorted, Notes: "pending"},
		},
	}

	if err := m.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify the file exists and is valid YAML
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	reloaded, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest after save failed: %v", err)
	}
	if len(reloaded.Features) != 1 {
		t.Errorf("expected 1 feature, got %d", len(reloaded.Features))
	}
	if reloaded.Features[0].UpstreamFeature != "test-feature" {
		t.Errorf("expected 'test-feature', got %q", reloaded.Features[0].UpstreamFeature)
	}
	if reloaded.LastChecked.IsZero() {
		t.Error("expected LastChecked to be set after save")
	}

	_ = data // data available for inspection
}

func TestSaveManifest_Atomicity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")

	// Write an initial manifest
	initial := &PortManifest{
		Version: 1,
		Features: []ManifestEntry{
			{UpstreamFeature: "original", Status: StatusPorted},
		},
	}
	if err := initial.Save(path); err != nil {
		t.Fatalf("initial Save failed: %v", err)
	}

	// Concurrent saves to test atomic write safety
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m := &PortManifest{
				Version: 1,
				Features: []ManifestEntry{
					{UpstreamFeature: "concurrent-feature", Notes: "writer"},
				},
			}
			// Ignore errors — we're testing that the file is never corrupted
			_ = m.Save(path)
		}(i)
	}
	wg.Wait()

	// Verify the file is always valid YAML
	reloaded, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest after concurrent saves: %v", err)
	}
	if reloaded.Version != 1 {
		t.Errorf("expected version 1, got %d", reloaded.Version)
	}
}

func TestSaveManifest_ParentDirNotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent", "manifest.yaml")

	m := &PortManifest{Version: 1}
	err := m.Save(path)
	if err == nil {
		t.Fatal("expected error when parent directory does not exist")
	}
}

func TestFindEntry(t *testing.T) {
	m := &PortManifest{
		Features: []ManifestEntry{
			{UpstreamFeature: "feature-a", Status: StatusPorted},
			{UpstreamFeature: "feature-b", Status: StatusNotPorted},
		},
	}

	entry := m.FindEntry("feature-a")
	if entry == nil {
		t.Fatal("expected to find 'feature-a'")
	}
	if entry.Status != StatusPorted {
		t.Errorf("expected 'ported', got %q", entry.Status)
	}

	entry = m.FindEntry("nonexistent")
	if entry != nil {
		t.Errorf("expected nil for nonexistent feature, got %+v", entry)
	}
}

func TestUpdateEntry_Existing(t *testing.T) {
	m := &PortManifest{
		Features: []ManifestEntry{
			{UpstreamFeature: "feature-a", Status: StatusPorted, Notes: "initial"},
		},
	}

	m.UpdateEntry("feature-a", StatusPartial, "updated")

	if len(m.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(m.Features))
	}
	if m.Features[0].Status != StatusPartial {
		t.Errorf("expected 'partial', got %q", m.Features[0].Status)
	}
	if m.Features[0].Notes != "updated" {
		t.Errorf("expected 'updated', got %q", m.Features[0].Notes)
	}
}

func TestUpdateEntry_New(t *testing.T) {
	m := &PortManifest{
		Features: []ManifestEntry{
			{UpstreamFeature: "existing", Status: StatusPorted},
		},
	}

	m.UpdateEntry("new-feature", StatusNotPorted, "pending review")

	if len(m.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(m.Features))
	}

	entry := m.FindEntry("new-feature")
	if entry == nil {
		t.Fatal("expected to find 'new-feature'")
	}
	if entry.Status != StatusNotPorted {
		t.Errorf("expected 'not-ported', got %q", entry.Status)
	}
}

func TestUpdateEntry_EmptyNotes(t *testing.T) {
	m := &PortManifest{
		Features: []ManifestEntry{
			{UpstreamFeature: "feature", Status: StatusPorted, Notes: "original"},
		},
	}

	// Update with empty notes — should not overwrite
	m.UpdateEntry("feature", StatusPartial, "")

	if m.Features[0].Notes != "original" {
		t.Errorf("expected notes to remain 'original', got %q", m.Features[0].Notes)
	}
}

func TestFindEntry_EmptyManifest(t *testing.T) {
	m := &PortManifest{Features: []ManifestEntry{}}
	entry := m.FindEntry("anything")
	if entry != nil {
		t.Errorf("expected nil for empty manifest, got %+v", entry)
	}
}


