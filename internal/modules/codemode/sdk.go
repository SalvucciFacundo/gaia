package codemode

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gaia/internal/core"
)

// GaiaSDK provides ergonomic, "Must"-style helper methods for dynamic Go code execution.
// Errors raise panics that are caught and safely formatted by the Engine sandbox.
type GaiaSDK struct {
	projectRoot string
	registry    *core.ToolRegistry
	out         *bytes.Buffer
	ctx         context.Context
}

// NewGaiaSDK initializes an SDK instance bound to a project root, registry, and output buffer.
func NewGaiaSDK(projectRoot string, registry *core.ToolRegistry, out *bytes.Buffer, ctx context.Context) *GaiaSDK {
	if out == nil {
		out = new(bytes.Buffer)
	}
	return &GaiaSDK{
		projectRoot: projectRoot,
		registry:    registry,
		out:         out,
		ctx:         ctx,
	}
}

// resolvePath returns the normalized path relative to the project root.
func (s *GaiaSDK) resolvePath(rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(s.projectRoot, rel)
}

// ReadFile reads the full contents of a file or panics if not found/unreadable.
func (s *GaiaSDK) ReadFile(path string) string {
	resolved := s.resolvePath(path)
	data, err := os.ReadFile(resolved)
	if err != nil {
		panic(fmt.Sprintf("ReadFile failed for %q: %v", path, err))
	}
	return string(data)
}

// WriteFile writes string content to a file, automatically creating parent directories.
func (s *GaiaSDK) WriteFile(path, content string) {
	resolved := s.resolvePath(path)
	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0755); err != nil {
		panic(fmt.Sprintf("WriteFile failed creating dir for %q: %v", path, err))
	}
	if err := os.WriteFile(resolved, []byte(content), 0644); err != nil {
		panic(fmt.Sprintf("WriteFile failed for %q: %v", path, err))
	}
}

// Find recursively searches for files matching a glob or substring pattern.
func (s *GaiaSDK) Find(pattern string) []string {
	var matches []string
	cleanPattern := filepath.Clean(pattern)

	_ = filepath.Walk(s.projectRoot, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		rel, err := filepath.Rel(s.projectRoot, p)
		if err != nil {
			return nil
		}
		// Skip hidden directories and large caches
		if info.IsDir() && (strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" || info.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}

		matched, _ := filepath.Match(cleanPattern, info.Name())
		if matched || strings.Contains(rel, cleanPattern) {
			matches = append(matches, rel)
		}
		return nil
	})
	return matches
}

// Grep scans a list of files for lines matching a regex pattern.
func (s *GaiaSDK) Grep(pattern string, files ...string) []string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		panic(fmt.Sprintf("invalid regex pattern %q: %v", pattern, err))
	}

	var results []string
	for _, f := range files {
		data := s.ReadFile(f)
		lines := strings.Split(data, "\n")
		for i, line := range lines {
			if re.MatchString(line) {
				results = append(results, fmt.Sprintf("%s:%d: %s", f, i+1, strings.TrimSpace(line)))
			}
		}
	}
	return results
}

// Exec runs a shell command within the project directory and returns stdout/stderr combined.
func (s *GaiaSDK) Exec(cmdStr string) string {
	cmd := exec.CommandContext(s.ctx, "sh", "-c", cmdStr)
	cmd.Dir = s.projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("command %q failed: %v (output: %s)", cmdStr, err, string(out)))
	}
	return string(out)
}

// Print writes arguments followed by a newline to the output buffer.
func (s *GaiaSDK) Print(args ...any) {
	fmt.Fprintln(s.out, args...)
}

// Printf writes formatted output using a format string to the output buffer.
func (s *GaiaSDK) Printf(format string, args ...any) {
	fmt.Fprintf(s.out, format, args...)
}
