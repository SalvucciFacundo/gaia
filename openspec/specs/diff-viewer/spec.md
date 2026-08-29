# Diff Viewer Specification

## Purpose

Provide structured parsing and visual terminal rendering of Git diffs within the GAIA TUI using Bubbletea and Lipgloss, including slash-command access and colored hunk visualization.

## Requirements

### Requirement: Unified Diff Parsing into Structured Files and Hunks

The system MUST parse raw unified Git diff output (`git diff`, `git diff --staged`, or arbitrary diff streams) into strongly-typed structures representing files, file metadata (old/new paths, binary indicators, change modes), and individual hunks with header offsets and categorized lines (addition, deletion, context).

#### Scenario: Parse multi-file unified diff with multiple hunks

- GIVEN a unified diff string containing changes across multiple files
- WHEN the diff parser processes the string
- THEN the system MUST produce a list of file diff objects
- AND each file diff MUST contain its relative file path, modification status, and list of parsed hunks
- AND each hunk MUST preserve header line numbers, hunk range metadata, and line types (addition, deletion, context).

#### Scenario: Handle binary or empty diffs

- GIVEN a Git diff containing binary file changes or no modifications
- WHEN the diff parser processes the output
- THEN the system MUST mark binary files without crashing or mangling content
- AND the system MUST return an empty file list when no changes are present.

### Requirement: Lipgloss Visual Diff Rendering

The system MUST render parsed diffs in the terminal using Lipgloss styles, color-coding additions (green), deletions (red), hunk headers (cyan/magenta or distinct accent), and context lines with neutral foreground formatting according to active terminal dimensions.

#### Scenario: Render addition and deletion lines with distinct colors

- GIVEN parsed diff hunks containing additions (`+`) and deletions (`-`)
- WHEN the diff view component renders the hunk lines
- THEN addition lines MUST be styled with green foreground styling
- AND deletion lines MUST be styled with red foreground styling
- AND hunk headers (e.g. `@@ -1,5 +1,6 @@`) MUST be styled with a distinct header highlight style.

#### Scenario: Line wrapping and terminal width truncation

- GIVEN terminal viewport dimensions of width `W` and height `H`
- WHEN rendering long diff lines exceeding width `W`
- THEN the system MUST clip or wrap lines cleanly using Lipgloss without breaking ANSI escape sequences or viewport bounds.

### Requirement: TUI Slash Command Integration (`/diff`)

The system MUST register a `/diff` slash command within the GAIA TUI command router, allowing users to invoke the interactive visual diff viewer on demand.

#### Scenario: User enters /diff command in TUI

- GIVEN the GAIA TUI is in interactive input mode
- WHEN the user submits `/diff` or `/diff --staged`
- THEN the TUI command router MUST instantiate the diff viewer Bubbletea model with the requested diff target
- AND the TUI MUST switch the active view to the interactive diff viewer.

#### Scenario: User enters /diff with no working directory changes

- GIVEN the Git working tree has no modified, added, or staged files
- WHEN the user submits `/diff`
- THEN the system MUST display an informational message indicating that the working tree is clean
- AND the TUI MUST remain in or return cleanly to the main conversation view.

### Requirement: Viewport Scrolling and Line Numbering

The system MUST support viewport scrolling and display relative or absolute line numbers for both original and modified file states within rendered hunks.

#### Scenario: Scroll viewport with j/k or arrow keys

- GIVEN the interactive diff viewer is active with content taller than the terminal height
- WHEN the user presses `j` (down) or `k` (up)
- THEN the viewport scroll offset MUST adjust accordingly
- AND the rendered lines MUST remain aligned with their corresponding line numbers and styles.
