# Proposal: tui-interactive-diff

## Intent
Implement an interactive visual diff viewer within the GAIA TUI to enable developers to review, navigate, and selectively stage or discard changes directly before committing.

## Scope
**In Scope:**
- Interactive diff viewer rendering colored git diffs using Lipgloss.
- Hunk navigation using `n` (next) and `p` (previous).
- Selective hunk-level staging and discarding.
- Registration of the `/diff` slash command in the TUI.
- Line-level human steering integration before commits.

**Out of Scope:**
- Merge conflict resolution.
- Commit history visualization (`git log` / graphs).
- Non-Git version control systems.

## Capabilities
**New:**
- `/diff` command handler.
- Bubbletea-based diff viewer model and update loops.
- Hunk parsing and state management (staged/unstaged mapping).
**Modified:**
- TUI command router to support `/diff`.
- Pre-commit flow to optionally inject the visual diff step.

## Approach
1. **State Machine:** Build a standalone Bubbletea model (`DiffModel`) handling hunk state, viewport scrolling, and keybindings.
2. **Parsing:** Wrap `git diff` and parse hunks into a structured format.
3. **Rendering:** Use Lipgloss to colorize additions (green), deletions (red), and headers (cyan/magenta), optimizing rendering for terminal dimensions.
4. **Operations:** Execute `git apply --cached` (or equivalent plumbing commands) for staging specific hunks.

## Affected Areas
- `tui/commands`: Command parsing and routing.
- `tui/views`: Addition of the new diff viewer view.
- `git/operations`: New wrappers for staging/discarding specific hunks.

## Risks
- **Performance:** Rendering massive diffs (e.g., auto-generated files or lockfiles) may block the Bubbletea main thread.
- **State Desync:** External file modifications while the diff view is open could invalidate hunk offsets and apply operations.

## Rollback Plan
Remove or disable the `/diff` slash command from the router and revert the pre-commit flow to its existing standard behavior.

## Success Criteria
- The `/diff` command opens a responsive, colored diff viewer.
- Users can navigate hunks via `n` and `p`.
- Staging or discarding hunks reflects accurately in subsequent `git status` calls without leaving the TUI.

---

## Proposal question round
*The following assumptions require validation to finalize this proposal:*
1. **Target users and situations:** Does the `/diff` view block all other TUI operations while open, or can the user background it?
2. **Edge cases:** What is the preferred fallback behavior if a diff is extremely large (e.g., >10,000 lines)? Should we truncate, warn the user, or omit large files?
3. **Scope boundaries:** The requirements mention "line-level human steering." Does this strictly require individual line-by-line staging for the first slice, or is hunk-level staging with line-level visibility sufficient?