package skills

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// AuditFinding represents a security or quality finding for a skill.
type AuditFinding struct {
	SkillName string
	Severity  string // "error", "warn", "info"
	Message   string
	File      string
}

// AuditResult holds the complete audit results for a skill.
type AuditResult struct {
	SkillName string
	Source    string
	Hash      string
	Passed    bool
	Findings  []AuditFinding
}

// dangerousPatterns are regex patterns that indicate potentially dangerous skill content.
var dangerousPatterns = []struct {
	Name    string
	Pattern *regexp.Regexp
	Severity string
}{
	{Pattern: regexp.MustCompile(`(?i)(rm\s+(-rf|--recursive)\s+[/~])`), Name: "destructive_file_operation", Severity: "error"},
	{Pattern: regexp.MustCompile(`(?i)(curl|wget)\s+.*\|.*\b(bash|sh|zsh)\b`), Name: "pipe_to_shell", Severity: "error"},
	{Pattern: regexp.MustCompile(`(?i)(eval|exec|system)\s*\(`), Name: "dynamic_code_execution", Severity: "warn"},
	{Pattern: regexp.MustCompile(`(?i)base64.*-d.*\|`), Name: "obfuscated_command", Severity: "warn"},
	{Pattern: regexp.MustCompile(`(?i)(s3|gs|az)://[a-zA-Z0-9._-]+`), Name: "cloud_storage_reference", Severity: "info"},
	{Pattern: regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password)\s*[:=]\s*['\"][a-zA-Z0-9_\-]{16,}`), Name: "hardcoded_credential", Severity: "error"},
	{Pattern: regexp.MustCompile(`(?i)chmod\s+777`), Name: "permissive_permissions", Severity: "warn"},
	{Pattern: regexp.MustCompile(`(?i)>(?:\s*>)?\s*(?:/etc/|/var/|/root/)`), Name: "system_file_write", Severity: "warn"},
	{Pattern: regexp.MustCompile(`(?i)(dd\s+if=|mkfs\.|fdisk)`), Name: "disk_operation", Severity: "error"},
	{Pattern: regexp.MustCompile(`(?i)~\/\.ssh\/`), Name: "ssh_key_reference", Severity: "info"},
}

// AuditSkill scans a single skill directory for security issues and computes its hash.
func AuditSkill(dir string) (*AuditResult, error) {
	skill, err := ParseSkillFile(dir)
	if err != nil {
		return nil, fmt.Errorf("parse skill %s: %w", dir, err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return nil, fmt.Errorf("read SKILL.md: %w", err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(content))

	result := &AuditResult{
		SkillName: skill.Meta.Name,
		Source:    skill.Meta.Source,
		Hash:      hash,
		Passed:    true,
	}

	// Scan for dangerous patterns in content
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		for _, dp := range dangerousPatterns {
			if dp.Pattern.MatchString(line) {
				finding := AuditFinding{
					SkillName: skill.Meta.Name,
					Severity:  dp.Severity,
					Message:   fmt.Sprintf("%s: %s", dp.Name, strings.TrimSpace(line)),
					File:      fmt.Sprintf("SKILL.md:%d", i+1),
				}
				result.Findings = append(result.Findings, finding)
				if dp.Severity == "error" {
					result.Passed = false
				}
			}
		}
	}

	// Check for required frontmatter fields
	if skill.Meta.Name == "" {
		result.Findings = append(result.Findings, AuditFinding{
			SkillName: skill.Meta.Name, Severity: "error",
			Message: "missing required field 'name' in frontmatter", File: "SKILL.md:1",
		})
		result.Passed = false
	}

	return result, nil
}

// AuditAll scans all skills in the hub index and returns audit results.
func (h *Hub) AuditAll() []AuditResult {
	h.ensureIndex()
	h.mu.RLock()
	entries := make([]SkillMeta, len(h.index))
	copy(entries, h.index)
	h.mu.RUnlock()

	var results []AuditResult
	for _, meta := range entries {
		dir := meta.DirPath
		if dir == "" {
			dir = h.findSkillDir(meta.Name)
		}
		if dir == "" {
			continue
		}

		result, err := AuditSkill(dir)
		if err != nil {
			results = append(results, AuditResult{
				SkillName: meta.Name,
				Source:    meta.Source,
				Passed:    false,
				Findings:  []AuditFinding{{Severity: "error", Message: err.Error()}},
			})
			continue
		}

		// Fill provenance if not set
		if skill, err := ParseSkillFile(dir); err == nil {
			skill.Meta.ContentHash = result.Hash
			skill.Meta.Provenance = meta.Source
		}

		results = append(results, *result)
	}
	return results
}

// FormatAuditResults formats audit results as a human-readable string.
func FormatAuditResults(results []AuditResult) string {
	var sb strings.Builder
	passed := 0
	failed := 0

	for _, r := range results {
		if r.Passed {
			passed++
		} else {
			failed++
		}

		status := "✅ PASS"
		if !r.Passed {
			status = "❌ FAIL"
		}

		sb.WriteString(fmt.Sprintf("%s %s [%s]\n", status, r.SkillName, r.Source))
		sb.WriteString(fmt.Sprintf("   Hash: %s\n", r.Hash))

		if len(r.Findings) > 0 {
			for _, f := range r.Findings {
				icon := "⚠️"
				if f.Severity == "error" {
					icon = "🚫"
				}
				sb.WriteString(fmt.Sprintf("   %s [%s] %s\n", icon, f.Severity, f.Message))
				if f.File != "" {
					sb.WriteString(fmt.Sprintf("       at %s\n", f.File))
				}
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Audit complete: %d passed, %d failed\n", passed, failed))
	return sb.String()
}
