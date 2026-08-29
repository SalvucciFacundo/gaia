# Diff Steering Specification

## Purpose

Define interactive navigation, selective hunk staging/discarding, and human steering injection within the GAIA TUI diff viewer, enabling developers to guide the agent and curate change sets before commits.

## Requirements

### Requirement: Interactive Hunk and File Navigation

The system MUST provide keyboard controls to navigate between hunks and files within the diff viewer, supporting `n` (next hunk), `p` (previous hunk), `j`/`k` (line scroll), and `q`/`esc` (exit viewer).

#### Scenario: Navigate to next and previous hunks

- GIVEN a diff view containing multiple files and hunks with hunk #1 currently focused
- WHEN the user presses `n`
- THEN the focus MUST move to hunk #2 and auto-scroll the viewport to ensure hunk #2 is visible
- AND WHEN the user presses `p`
- THEN the focus MUST move back to hunk #1.

#### Scenario: Exit diff viewer cleanly

- GIVEN the interactive diff viewer is open
- WHEN the user presses `q` or `esc`
- THEN the diff viewer MUST exit and return the user to the active TUI conversation view without loss of state.

### Requirement: Selective Hunk Staging and Discarding

The system MUST allow users to selectively stage or discard the currently focused hunk directly from the TUI using Git plumbing operations (`git apply --cached` or index manipulation).

#### Scenario: Stage currently focused hunk

- GIVEN an unstaged hunk is focused in the diff viewer
- WHEN the user presses the stage shortcut (`s`)
- THEN the system MUST construct a patch for only the selected hunk and apply it to the Git index (`git apply --cached`)
- AND the system MUST update the visual status of the hunk to reflect its staged state
- AND if all hunks in a file are staged, the file status MUST update to staged.

#### Scenario: Discard currently focused unstaged hunk

- GIVEN an unstaged hunk is focused in the diff viewer
- WHEN the user presses the discard shortcut (`d`) and confirms the action
- THEN the system MUST apply a reverse patch for that hunk to the working tree
- AND the diff viewer MUST refresh the diff view to remove the discarded hunk.

### Requirement: Line-Level Steering Prompt for Agent Feedback

The system MUST provide a steering input mode (activated via key shortcuts such as `e` for edit guidance or `r` for feedback/refine) allowing the user to attach natural language instructions to a specific hunk or line range, which are injected back into the active agent context.

#### Scenario: Open steering prompt on focused hunk

- GIVEN a specific hunk or line is focused in the diff viewer
- WHEN the user presses `e` or `r`
- THEN the system MUST open a text input overlay anchored to the focused hunk
- AND the prompt MUST prompt the user for steering feedback.

#### Scenario: Submit steering feedback to agent loop

- GIVEN the steering prompt overlay is open with user input "Simplify error handling here"
- WHEN the user presses `Enter` to submit
- THEN the system MUST package the file path, hunk diff snippet, line range, and user comment into a structured steering message
- AND the system MUST inject the steering message into the agent's conversation context
- AND the TUI MUST close the diff viewer and trigger agent replanning or remediation based on the guidance.

### Requirement: Pre-Commit Visual Review Flow Integration

The system MUST support optional invocation of the interactive diff viewer during the agent pre-commit workflow, allowing human review and steering before changes are committed.

#### Scenario: Review diff before commit approval

- GIVEN the agent loop is preparing to execute a commit
- WHEN visual review is enabled or requested
- THEN the TUI MUST present the interactive diff viewer to the user
- AND the user MAY stage/discard hunks or provide steering guidance before approving the final commit action.

### Requirement: State Synchronization and Apply Failure Handling

The system MUST handle concurrent file changes or Git apply failures gracefully, notifying the user if a hunk cannot be staged or discarded due to index divergence.

#### Scenario: Staging fails due to external file change

- GIVEN a hunk displayed in the diff viewer
- WHEN the underlying file is modified externally and the user attempts to stage the hunk
- THEN the system MUST detect the patch application failure
- AND the system MUST display an error message explaining the desynchronization
- AND the system MUST offer to refresh the diff view with current working tree state.
