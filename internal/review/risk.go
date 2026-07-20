package review

import (
	"path/filepath"
	"strings"
)

// RiskCode identifies a specific risk signal detected in a change.
type RiskCode string

// The eight risk codes in the GAIA review taxonomy.
const (
	RiskConfigurationChange RiskCode = "configuration_change"
	RiskExecutableChange    RiskCode = "executable_change"
	RiskExecutableMode      RiskCode = "executable_mode"
	RiskHotPath             RiskCode = "hot_path"
	RiskLargeChange         RiskCode = "large_change"
	RiskNonExecutableOnly   RiskCode = "non_executable_only"
	RiskServiceToken        RiskCode = "service_token"
	RiskShellSource         RiskCode = "shell_source"
)

// highRiskCodes are codes that, when present, force a High risk level.
var highRiskCodes = map[RiskCode]bool{
	RiskHotPath:      true,
	RiskLargeChange:  true,
	RiskServiceToken: true,
	RiskShellSource:  true,
}

// fileTypeLens maps file type categories to their dominant lens for
// medium-risk changes.
var fileTypeLens = map[string]string{
	"config":  "review-risk",
	"test":    "review-reliability",
	"doc":     "review-readability",
	"service": "review-resilience",
}

// ClassifyRisk examines a diff string and list of file paths to detect
// risk codes. Each code is returned at most once.
func ClassifyRisk(diff string, files []string) []RiskCode {
	seen := make(map[RiskCode]bool)
	var codes []RiskCode

	// Analyze each file.
	hasNonDoc := false
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		base := strings.ToLower(filepath.Base(f))
		dir := strings.ToLower(filepath.Dir(f))

		// configuration_change
		if isConfigFile(base, ext) {
			seen[RiskConfigurationChange] = true
		}

		// executable_change
		if isExecutable(f) {
			seen[RiskExecutableChange] = true
		}

		// executable_mode
		if hasExecutableModeChange(diff, f) {
			seen[RiskExecutableMode] = true
		}

		// hot_path
		if isHotPath(f, dir) {
			seen[RiskHotPath] = true
		}

		// shell_source
		if isShellSource(ext) {
			seen[RiskShellSource] = true
		}

		// Track if there's any non-documentation change.
		if !isDocOnly(ext) {
			hasNonDoc = true
		}
	}

	// large_change: more than 400 changed lines in the diff.
	if diffLineCount(diff) > 400 {
		seen[RiskLargeChange] = true
	}

	// service_token: detect secrets/tokens in the diff.
	if hasServiceToken(diff) {
		seen[RiskServiceToken] = true
	}

	// non_executable_only: only docs, comments, formatting, typo fixes.
	if !hasNonDoc && len(files) > 0 {
		seen[RiskNonExecutableOnly] = true
	}

	for code := range seen {
		codes = append(codes, code)
	}
	if len(codes) == 0 {
		// If we couldn't classify, default to configuration_change as a catch-all.
		codes = append(codes, RiskConfigurationChange)
	}
	return codes
}

// DetermineRiskLevel returns "low", "medium", or "high" based on the
// set of risk codes detected.
//
// Rules:
//   - Only non_executable_only → Low
//   - Any high-risk code (hot_path, large_change, service_token, shell_source) → High
//   - Any other risk code → Medium
func DetermineRiskLevel(codes []RiskCode) string {
	hasHigh := false
	hasNonExec := false
	hasOther := false

	for _, c := range codes {
		if highRiskCodes[c] {
			hasHigh = true
		}
		if c == RiskNonExecutableOnly {
			hasNonExec = true
		}
		if c != RiskNonExecutableOnly && !highRiskCodes[c] {
			hasOther = true
		}
	}

	if hasHigh {
		return "high"
	}
	if hasOther {
		return "medium"
	}
	if hasNonExec {
		return "low"
	}
	return "medium" // default
}

// SelectLenses determines which lenses to run based on risk level and
// file types.
//
// Rules:
//   - High risk → all 4 lenses (risk, resilience, readability, reliability)
//   - Medium risk → one dominant lens based on file type
//   - Low risk → no lenses (auto-approve)
func SelectLenses(riskLevel string, files []string) []string {
	switch riskLevel {
	case "high":
		return []string{"review-risk", "review-resilience", "review-readability", "review-reliability"}
	case "low":
		return nil
	default: // medium
		return []string{dominantLensFromFiles(files)}
	}
}

// dominantLensFromFiles selects the best lens for a set of files based
// on their types. Priority: config → test → doc → service → default(risk).
func dominantLensFromFiles(files []string) string {
	hasConfig := false
	hasTest := false
	hasDoc := false
	hasService := false

	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		base := strings.ToLower(filepath.Base(f))

		if isConfigFile(base, ext) {
			hasConfig = true
		} else if isTestFile(base, ext) {
			hasTest = true
		} else if isDocOnly(ext) {
			hasDoc = true
		} else {
			hasService = true
		}
	}

	switch {
	case hasConfig:
		return fileTypeLens["config"]
	case hasTest:
		return fileTypeLens["test"]
	case hasDoc:
		return fileTypeLens["doc"]
	case hasService:
		return fileTypeLens["service"]
	default:
		return fileTypeLens["config"] // "review-risk" as fallback
	}
}

// Helper functions for file classification.

func isConfigFile(base, ext string) bool {
	configBases := map[string]bool{
		".env": true, "config.yaml": true, "config.yml": true, "config.json": true,
		"config.toml": true, "makefile": true, "dockerfile": true,
		"docker-compose.yaml": true, "docker-compose.yml": true,
	}
	configExts := map[string]bool{
		".yaml": true, ".yml": true, ".toml": true, ".json": true, ".ini": true,
		".cfg": true, ".conf": true, ".env": true,
	}
	if configBases[base] {
		return true
	}
	if configExts[ext] {
		return true
	}
	// Also check for files in config/ directories.
	return strings.Contains(base, "config") || strings.Contains(base, "env")
}

func isExecutable(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".exe" || ext == ".bin" || ext == ".sh" || ext == ""
}

func hasExecutableModeChange(diff, file string) bool {
	// Detect chmod +x or file mode changes in git diffs.
	return strings.Contains(diff, "old mode") &&
		strings.Contains(diff, "new mode") &&
		strings.Contains(diff, "100755")
}

func isHotPath(path, dir string) bool {
	hotPathDirs := []string{
		"auth", "authorization", "payment", "payments",
		"security", "crypto", "cert", "token", "secret",
		"login", "session", "permission", "access",
	}
	lower := strings.ToLower(path)
	for _, h := range hotPathDirs {
		if strings.Contains(lower, h) {
			return true
		}
	}
	// Also check the directory components.
	dirLower := strings.ToLower(dir)
	for _, h := range hotPathDirs {
		if strings.Contains(dirLower, h) {
			return true
		}
	}
	return false
}

func isShellSource(ext string) bool {
	shellExts := map[string]bool{
		".sh": true, ".bash": true, ".zsh": true, ".fish": true,
		".ps1": true, ".bat": true, ".cmd": true,
	}
	return shellExts[ext]
}

func isDocOnly(ext string) bool {
	docExts := map[string]bool{
		".md": true, ".mdx": true, ".rst": true, ".txt": true,
		".adoc": true, ".org": true,
	}
	return docExts[ext]
}

func isTestFile(base, ext string) bool {
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, "_test.py") ||
		strings.HasSuffix(base, ".test.ts") ||
		strings.HasSuffix(base, ".spec.ts") ||
		strings.Contains(base, "test_") ||
		strings.Contains(base, "_test.")
}

// diffLineCount estimates the number of changed lines in a diff.
func diffLineCount(diff string) int {
	count := 0
	for _, line := range strings.Split(diff, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "-") {
			// Skip +++ and --- file headers.
			if !strings.HasPrefix(trimmed, "+++") && !strings.HasPrefix(trimmed, "---") {
				count++
			}
		}
	}
	return count
}

// hasServiceToken checks for API keys, tokens, or secrets in the diff.
func hasServiceToken(diff string) bool {
	tokenIndicators := []string{
		"sk-", "ghp_", "github_pat_", "AKIA",
		"api_key", "apiKey", "apikey",
		"secret_key", "secretKey",
		"access_token", "accessToken",
		"private_key", "privateKey",
		"-----BEGIN", "PRIVATE KEY-----",
	}
	lower := strings.ToLower(diff)
	for _, ind := range tokenIndicators {
		if strings.Contains(lower, strings.ToLower(ind)) {
			return true
		}
	}
	return false
}
