# Capability Specification: Skill Path Injection Protocol

## Capability: `skill-path-injection`

The Skill Loader resolves relevant skills from the registry and injects only file paths (`## Skills to load before work`) into subagent prompts, preserving agent context windows.

---

### Requirement: SKILL-001 — Path-Based Skill Ingestion

The orchestrator and spawner MUST pass exact skill file paths rather than inline summaries when spawning subagents.

#### Scenario: Subagent prompt receives skill path references
- **Given** an Implementer task touching Go files
- **When** the Spawner constructs the system prompt
- **Then** it SHALL include `## Skills to load before work` followed by exact paths such as `skills/golang-testing/SKILL.md`
- **And** the subagent SHALL read the full skill file on demand using `file_read`
