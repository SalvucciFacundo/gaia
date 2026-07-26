package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	globalPolicyDir  = ".config/gaia"
	globalPolicyFile = "policy.yaml"
	projectPolicyDir  = ".gaia"
	projectPolicyFile = "policy.yaml"
)

// PolicyStore defines the persistence contract for policy configuration.
// Implementations load, save, and merge global and project-level policies.
type PolicyStore interface {
	// LoadGlobal loads the global policy from ~/.config/gaia/policy.yaml.
	// Returns nil, nil if the file does not exist.
	LoadGlobal() (*PolicyConfig, error)

	// LoadProject loads the project policy from <root>/.gaia/policy.yaml.
	// Returns nil, nil if the file does not exist.
	LoadProject(root string) (*PolicyConfig, error)

	// Save persists a policy config at the given scope ("global" or "project").
	Save(cfg *PolicyConfig, scope string) error

	// Merge combines global and project policies with the defined precedence:
	// hardline > global deny > project deny > global overrides > project overrides > tier defaults.
	Merge(global, project *PolicyConfig) *PolicyConfig
}

// YAMLPolicyStore loads and saves PolicyConfig from YAML files.
// Global path: ~/.config/gaia/policy.yaml
// Project path: .gaia/policy.yaml (relative to project root)
type YAMLPolicyStore struct {
	projectRoot string // default project root for Save
}

// NewYAMLPolicyStore creates a YAML-backed policy store.
// projectRoot is used as the default when persisting project-scoped policies.
func NewYAMLPolicyStore(projectRoot string) *YAMLPolicyStore {
	return &YAMLPolicyStore{projectRoot: projectRoot}
}

// GlobalPath returns the global policy file path.
func (s *YAMLPolicyStore) GlobalPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get home directory: %w", err)
	}
	return filepath.Join(home, globalPolicyDir, globalPolicyFile), nil
}

// ProjectPath returns the project policy file path for the given root.
func (s *YAMLPolicyStore) ProjectPath(root string) string {
	if root == "" {
		root = s.projectRoot
	}
	return filepath.Join(root, projectPolicyDir, projectPolicyFile)
}

// LoadGlobal loads the global policy from YAML.
// Returns nil, nil if the file does not exist (no policy configured globally).
func (s *YAMLPolicyStore) LoadGlobal() (*PolicyConfig, error) {
	path, err := s.GlobalPath()
	if err != nil {
		return nil, err
	}
	return loadYAMLPolicy(path)
}

// LoadProject loads the project policy from YAML.
// Returns nil, nil if the file does not exist (no project-specific policy).
func (s *YAMLPolicyStore) LoadProject(root string) (*PolicyConfig, error) {
	if root == "" {
		root = s.projectRoot
	}
	path := s.ProjectPath(root)
	return loadYAMLPolicy(path)
}

// Save persists a policy config to the given scope.
// scope must be "global" or "project".
func (s *YAMLPolicyStore) Save(cfg *PolicyConfig, scope string) error {
	if cfg == nil {
		return fmt.Errorf("cannot save nil policy config")
	}

	var path string
	switch scope {
	case "global":
		var err error
		path, err = s.GlobalPath()
		if err != nil {
			return err
		}
	case "project":
		path = s.ProjectPath(s.projectRoot)
	default:
		return fmt.Errorf("unknown scope %q: must be \"global\" or \"project\"", scope)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create policy directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write policy file: %w", err)
	}
	return nil
}

// Merge combines global and project policies with the defined precedence:
//  1. Hardline blocklist (absolute — never overridable, handled separately)
//  2. Global deny rules
//  3. Project deny rules
//  4. Global per-tool overrides
//  5. Project per-tool overrides
//  6. Tier defaults
//  7. Path rules: global first, project appends/overrides
//
// The project tier takes precedence over the global tier.
func (s *YAMLPolicyStore) Merge(global, project *PolicyConfig) *PolicyConfig {
	merged := DefaultPolicyConfig()

	// Determine effective tier: project wins over global
	if global != nil && global.Tier != "" {
		merged.Tier = global.Tier
	}
	if project != nil && project.Tier != "" {
		merged.Tier = project.Tier
	}

	// Build overrides: global first, then project overwrites
	if global != nil {
		for k, v := range global.Overrides {
			merged.Overrides[k] = v
		}
	}
	if project != nil {
		for k, v := range project.Overrides {
			merged.Overrides[k] = v
		}
	}

	// Build deny rules: global deny rules come first, then project
	if global != nil {
		merged.DenyRules = append(merged.DenyRules, global.DenyRules...)
	}
	if project != nil {
		merged.DenyRules = append(merged.DenyRules, project.DenyRules...)
	}

	// Build path rules: global first, then project appends/overrides
	if global != nil {
		merged.PathRules = append(merged.PathRules, global.PathRules...)
	}
	if project != nil {
		// Project path rules override global for same pattern
		for _, pr := range project.PathRules {
			replaced := false
			for i, existing := range merged.PathRules {
				if existing.Pattern == pr.Pattern {
					merged.PathRules[i] = pr
					replaced = true
					break
				}
			}
			if !replaced {
				merged.PathRules = append(merged.PathRules, pr)
			}
		}
	}

	return merged
}

// DefaultPolicyConfig returns a sensible default policy configuration.
func DefaultPolicyConfig() *PolicyConfig {
	return &PolicyConfig{
		Tier:      TierFull,
		Overrides: make(map[string]OverridePolicy),
		DenyRules: nil,
	}
}

// --- Deny Rule Matching ---

// IsDeniedByRules checks whether a command string matches any deny rule glob.
// Deny rules use simple glob matching: lowercase contains, with * wildcard support.
// Returns true and the matching rule if denied.
func IsDeniedByRules(cmd string, rules []string) (bool, string) {
	if cmd == "" || len(rules) == 0 {
		return false, ""
	}
	normalized := strings.TrimSpace(strings.ToLower(cmd))
	for _, rule := range rules {
		if matchDenyRule(normalized, strings.ToLower(strings.TrimSpace(rule))) {
			return true, rule
		}
	}
	return false, ""
}

// matchDenyRule checks a single deny rule against a normalized command.
// Rules support simple * wildcards: "rm*" matches "rm -rf /".
func matchDenyRule(cmd, rule string) bool {
	if rule == "" {
		return false
	}
	if !strings.Contains(rule, "*") {
		return strings.Contains(cmd, rule)
	}
	// Simple glob: split on *, check each segment appears in order
	parts := strings.Split(rule, "*")
	pos := 0
	for _, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(cmd[pos:], part)
		if idx < 0 {
			return false
		}
		pos += idx + len(part)
	}
	return true
}

// loadYAMLPolicy reads and unmarshals a PolicyConfig from a YAML file.
// Returns nil, nil if the file does not exist.
func loadYAMLPolicy(path string) (*PolicyConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read policy file %s: %w", path, err)
	}

	var cfg PolicyConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal policy from %s: %w", path, err)
	}
	return &cfg, nil
}
