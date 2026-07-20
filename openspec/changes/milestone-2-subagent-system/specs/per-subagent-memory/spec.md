# Per-Subagent Memory Specification

## Purpose

Isolated Engram memory namespaces per subagent, enabling domain-specific knowledge accumulation without cross-contamination.

## Requirements

### Requirement: Topic Key Namespace

Each subagent MUST use Engram topic keys in the format `gaia/{subagent}/{project}`. The namespace MUST be enforced by the memory wrapper, not self-reported by the subagent.

#### Scenario: Namespace isolation

- GIVEN the Explorer subagent saves a memory
- WHEN the topic key is generated
- THEN it follows the pattern `gaia/explorer/{project-name}`

#### Scenario: Cross-subagent isolation

- GIVEN the Explorer and Implementer both save memories
- WHEN searching the Explorer's namespace
- THEN Implementer memories are NOT returned

### Requirement: Memory Retrieval

Each subagent MUST have access to `search` and `get_observation` operations scoped to its namespace. The wrapper SHALL prepend the namespace prefix to all search queries automatically.

#### Scenario: Scoped search

- GIVEN the Proposer searches for "proposal patterns"
- WHEN the search executes
- THEN only memories under `gaia/proposer/{project}` are queried

#### Scenario: Full observation retrieval

- GIVEN a search result with an observation ID
- WHEN get_observation is called
- THEN the full untruncated content is returned

### Requirement: Memory Lifecycle

Subagent memories MUST follow Engram lifecycle rules: `active` memories are usable, `needs_review` memories MUST be surfaced as stale context before use. The wrapper SHALL check lifecycle state on retrieval.

#### Scenario: Active memory use

- GIVEN an observation with state "active"
- WHEN the subagent retrieves it
- THEN the content is returned without warning

#### Scenario: Stale memory warning

- GIVEN an observation with state "needs_review"
- WHEN the subagent retrieves it
- THEN the wrapper includes a staleness warning in the response
