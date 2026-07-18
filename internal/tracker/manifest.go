package tracker

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// ManifestStatus is the port status of a feature.
type ManifestStatus string

const (
	StatusPorted        ManifestStatus = "ported"
	StatusPartial       ManifestStatus = "partial"
	StatusNotPorted     ManifestStatus = "not-ported"
	StatusNotApplicable ManifestStatus = "not-applicable"
)

// ManifestEntry represents one upstream feature and its port status.
type ManifestEntry struct {
	UpstreamFeature string         `yaml:"upstream_feature"`
	Status          ManifestStatus `yaml:"status"`
	GaiaLocation    string         `yaml:"gaia_location"`
	UpstreamVersion string         `yaml:"upstream_version"`
	Notes           string         `yaml:"notes"`
}

// PortManifest is the YAML-backed tracker of feature port status.
type PortManifest struct {
	Version     int             `yaml:"version"`
	LastChecked time.Time       `yaml:"last_checked"`
	Features    []ManifestEntry `yaml:"features"`
}

// LoadManifest reads a port-manifest.yaml file from the given path.
// Returns an empty manifest if the file does not exist.
func LoadManifest(path string) (*PortManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &PortManifest{
				Version:  1,
				Features: []ManifestEntry{},
			}, nil
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var m PortManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	if m.Version == 0 {
		m.Version = 1
	}
	if m.Features == nil {
		m.Features = []ManifestEntry{}
	}
	return &m, nil
}

// Save writes the manifest atomically: write to a temp file, then rename.
func (m *PortManifest) Save(path string) error {
	m.LastChecked = time.Now()

	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	// Write to a temporary file in the same directory.
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".port-manifest-*.yaml")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	cleanup := func() {
		tmpFile.Close()
		os.Remove(tmpPath)
	}

	if _, err := tmpFile.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	// Atomic rename.
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// FindEntry locates a manifest entry by feature name. Returns nil if not found.
func (m *PortManifest) FindEntry(featureName string) *ManifestEntry {
	for i := range m.Features {
		if m.Features[i].UpstreamFeature == featureName {
			return &m.Features[i]
		}
	}
	return nil
}

// UpdateEntry sets the status and notes for a feature. Creates a new entry if
// the feature does not exist in the manifest.
func (m *PortManifest) UpdateEntry(featureName string, status ManifestStatus, notes string) {
	entry := m.FindEntry(featureName)
	if entry == nil {
		m.Features = append(m.Features, ManifestEntry{
			UpstreamFeature: featureName,
			Status:          status,
			Notes:           notes,
		})
		return
	}
	entry.Status = status
	if notes != "" {
		entry.Notes = notes
	}
}
