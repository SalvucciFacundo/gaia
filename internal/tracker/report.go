package tracker

import (
	"fmt"
	"strings"
)

// GenerateReport produces a markdown table of all features grouped by port status.
func GenerateReport(manifest *PortManifest) string {
	var b strings.Builder

	b.WriteString("# GAIA Upstream Port Status\n\n")

	// Group features by status.
	grouped := make(map[ManifestStatus][]ManifestEntry)
	for _, f := range manifest.Features {
		grouped[f.Status] = append(grouped[f.Status], f)
	}

	// Define display order.
	order := []ManifestStatus{
		StatusPorted,
		StatusPartial,
		StatusNotPorted,
		StatusNotApplicable,
	}

	total := len(manifest.Features)

	for _, status := range order {
		entries := grouped[status]
		if len(entries) == 0 {
			continue
		}

		b.WriteString(fmt.Sprintf("## %s (%d)\n\n", statusLabel(status), len(entries)))
		b.WriteString("| Feature | Location | Version | Notes |\n")
		b.WriteString("|---------|----------|---------|-------|\n")

		for _, e := range entries {
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				e.UpstreamFeature,
				orDash(e.GaiaLocation),
				orDash(e.UpstreamVersion),
				orDash(e.Notes),
			))
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("---\n*%d total features tracked. Last checked: %s*\n",
		total,
		manifest.LastChecked.Format("2006-01-02 15:04"),
	))

	return b.String()
}

// GenerateDelta produces a markdown report showing what changed since the last check.
func GenerateDelta(lastCheck, newReleases []Change, manifest *PortManifest) string {
	var b strings.Builder

	b.WriteString("# Upstream Release Delta\n\n")

	if len(newReleases) == 0 {
		b.WriteString("No new upstream changes detected.\n")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("## New Changes (%d)\n\n", len(newReleases)))
	b.WriteString("| Type | Description | Release |\n")
	b.WriteString("|------|-------------|----------|\n")

	for _, c := range newReleases {
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
			c.Type,
			c.Description,
			c.Release,
		))
	}
	b.WriteString("\n")

	// Show impact on manifest — which features are affected.
	affected := findAffectedFeatures(newReleases, manifest)
	if len(affected) > 0 {
		b.WriteString("## Affected Features\n\n")
		b.WriteString("| Feature | Current Status |\n")
		b.WriteString("|---------|---------------|\n")
		for _, a := range affected {
			b.WriteString(fmt.Sprintf("| %s | %s |\n",
				a.UpstreamFeature,
				a.Status,
			))
		}
		b.WriteString("\n")
	}

	_ = lastCheck // reserved for future delta-from-timestamp logic.
	return b.String()
}

func statusLabel(s ManifestStatus) string {
	switch s {
	case StatusPorted:
		return "✅ Ported"
	case StatusPartial:
		return "🔶 Partially Ported"
	case StatusNotPorted:
		return "❌ Not Ported"
	case StatusNotApplicable:
		return "⬜ Not Applicable"
	default:
		return string(s)
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func findAffectedFeatures(changes []Change, manifest *PortManifest) []ManifestEntry {
	// Match change descriptions against manifest feature names.
	seen := make(map[string]bool)
	var affected []ManifestEntry

	for _, c := range changes {
		for _, f := range manifest.Features {
			if seen[f.UpstreamFeature] {
				continue
			}
			// Simple heuristic: feature name appears in the change description.
			if strings.Contains(strings.ToLower(c.Description), strings.ToLower(f.UpstreamFeature)) ||
				strings.Contains(strings.ToLower(c.Raw), strings.ToLower(f.UpstreamFeature)) {
				affected = append(affected, f)
				seen[f.UpstreamFeature] = true
			}
		}
	}
	return affected
}
