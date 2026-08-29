# Capability Specification: Artifact Store Policy & Memory Routing

## Capability: `store-policy`

The Store Policy governs how SDD phase artifacts and state are persisted across `engram`, `openspec`, `hybrid`, and `none` modes, preventing blind file or memory queries.

---

### Requirement: STORE-001 — Multi-Backend Store Selection

The orchestrator MUST resolve the active artifact store mode per session and route artifact queries to the corresponding backend.

#### Scenario: Engram-only store resolution
- **Given** the artifact store mode is configured as `engram`
- **When** SDD status or artifacts are retrieved
- **Then** the engine SHALL query Engram / SQLite topic keys directly without requiring `openspec/` markdown files on disk

#### Scenario: Hybrid store resolution
- **Given** the artifact store mode is configured as `hybrid`
- **When** an SDD phase saves its output
- **Then** the engine SHALL persist the artifact to both the filesystem (`openspec/changes/<change>/...`) and Engram memory
