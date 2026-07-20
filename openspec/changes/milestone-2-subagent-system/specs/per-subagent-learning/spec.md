# Per-Subagent Learning Specification

## Purpose

Independent learning loop per subagent: post-task nudge, session summary generation, and domain skill creation. Includes cross-pollination via shared knowledge graph.

## Requirements

### Requirement: Post-Task Nudge

After every N subagent executions (configurable, default N=5), the system MUST prompt the subagent to reflect on patterns observed. The nudge SHALL be non-blocking — the subagent MAY decline.

#### Scenario: Nudge trigger

- GIVEN the Implementer has completed 5 tasks
- WHEN the 6th task finishes
- THEN the system sends a nudge asking "What patterns did you observe?"

#### Scenario: Nudge decline

- GIVEN a nudge is sent
- WHEN the subagent declines
- THEN no skill or summary is created and the counter resets

### Requirement: Session Summary

When nudged, the subagent MUST generate a session summary capturing: Goal, Discoveries, Accomplished, and Relevant Files. The summary SHALL be saved to the subagent's Engram namespace.

#### Scenario: Summary generation

- GIVEN the Explorer is nudged after investigation tasks
- WHEN it generates a summary
- THEN the summary is saved under `gaia/explorer/{project}` with type "learning"

### Requirement: Skill Creation

When the subagent identifies a repeatable pattern across summaries, it MAY create or improve a skill file. Skill creation MUST follow the skill-creator format. The system SHALL check for existing skills before creating duplicates.

#### Scenario: New skill creation

- GIVEN the Implementer identifies a repeated pattern
- WHEN it creates a skill
- THEN the skill file follows the LLM-first SKILL.md format

#### Scenario: Existing skill check

- GIVEN a skill with similar trigger already exists
- WHEN the subagent attempts to create a new skill
- THEN the system warns about the overlap and suggests improvement instead

### Requirement: Knowledge Cross-Pollination

Subagents SHALL share high-value observations through a shared knowledge graph namespace (`gaia/shared/{project}`). Cross-pollination MUST be read-only — subagents read shared knowledge but write only to their own namespace.

#### Scenario: Shared knowledge read

- GIVEN the Verifier starts a verification task
- WHEN it checks shared knowledge
- THEN it receives observations from other subagents' summaries

#### Scenario: Write isolation

- GIVEN the Verifier attempts to write to shared namespace
- WHEN the write is attempted
- THEN the system rejects it — writes go to `gaia/verifier/{project}` only
