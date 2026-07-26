# Policy Guard Specification

## Purpose

Policy-based execution guard for autonomous agent operations. Enforces tier-based permissions across ALL execution paths (Brain loop and Spawner RunLoop) with a hardline blocklist, user-defined deny rules, and smart escalation.

## Requirements

### Requirement: Policy Tiers

The system MUST define 3 policy tiers: `read` (no mutations allowed), `sandbox` (mutations allowed within sandboxed scope), `full` (all operations permitted). Each tool MUST be classified into exactly one tier category. The system SHALL default to `read` tier on fresh install.

#### Scenario: Read tier blocks mutation

- GIVEN policy tier is `read`
- WHEN a shell_exec tool call attempts `git commit`
- THEN the system denies execution and escalates per the escalation chain

#### Scenario: Full tier permits operation

- GIVEN policy tier is `full`
- WHEN any tool call is issued
- THEN the system executes without policy denial

#### Scenario: Sandbox tier scopes mutations

- GIVEN policy tier is `sandbox`
- WHEN a file_write targets a path inside the project workspace
- THEN the system permits execution
- AND when the same call targets a path outside the workspace, the system denies

### Requirement: Hardline Blocklist

The system MUST compile an immutable blocklist into the binary. Blocklisted commands SHALL be denied regardless of tier or user overrides. The blocklist MUST include at minimum: `rm -rf /`, fork bombs, `dd` to block devices, `mkfs` on mounted root, piping untrusted URLs to `sh`.

#### Scenario: Blocklist overrides full tier

- GIVEN policy tier is `full` and no user overrides exist
- WHEN a tool call matches a blocklisted pattern
- THEN the system denies execution unconditionally

#### Scenario: Blocklist cannot be overridden

- GIVEN a user-defined allow rule matching a blocklisted pattern
- WHEN the tool call is evaluated
- THEN the blocklist takes precedence and execution is denied

### Requirement: User-Defined Deny Rules

The system MUST support user-defined deny rules via YAML glob patterns. Rules MUST be loaded from project-level (`.gaia/permissions.yaml`) and global-level (`~/.config/gaia/permissions.yaml`) files. Global deny rules SHALL take precedence over project-level allow rules.

#### Scenario: Project deny rule blocks tool

- GIVEN `.gaia/permissions.yaml` contains a deny rule for glob `shell_exec:*rm*`
- WHEN a shell_exec call matches the pattern
- THEN the system denies execution regardless of tier

#### Scenario: Global deny overrides project allow

- GIVEN global config denies `shell_exec:*` and project config allows `shell_exec:ls`
- WHEN a `shell_exec:ls` call is issued
- THEN the system denies execution (global wins on deny)

### Requirement: Policy Evaluation Point

The system MUST evaluate policy BEFORE every tool execution in both the Brain loop (`kernel.go:handleToolCalls`) and the Spawner RunLoop (`spawner.go:RunLoop`). No tool call SHALL bypass policy evaluation.

#### Scenario: Brain loop evaluation

- GIVEN the Brain loop receives a tool call
- WHEN the tool call is about to execute
- THEN PolicyGuard evaluates the call before execution proceeds

#### Scenario: Spawner RunLoop evaluation

- GIVEN a sub-agent issues a tool call via Spawner.RunLoop
- WHEN the tool call is about to execute
- THEN PolicyGuard evaluates the call before execution proceeds

### Requirement: Smart Escalation Chain

When a policy evaluation denies a tool call, the system MUST follow an escalation chain: (1) skip silently if the tool is marked skippable, (2) attempt a registered alternative tool, (3) block and notify the user with context. The system SHALL NOT prompt the user unless steps 1 and 2 fail.

#### Scenario: Skip on skippable denial

- GIVEN a denied tool call is marked as skippable
- WHEN policy evaluation denies the call
- THEN the system skips the call silently and continues execution

#### Scenario: Alternative tool fallback

- GIVEN a denied tool call is not skippable and has a registered alternative
- WHEN policy evaluation denies the call
- THEN the system attempts the alternative tool
- AND if the alternative is permitted, executes it

#### Scenario: User notification on final block

- GIVEN a denied tool call has no skippable flag and no alternatives
- WHEN policy evaluation denies the call
- THEN the system blocks execution and notifies the user with the denial reason and tool context

### Requirement: Tier Migration Path

On upgrade from the legacy confirmation system, the system MUST default to basic mode (current behavior plus hardline blocklist). The legacy `ConfirmGuard` SHALL be preserved as an adapter over `PolicyGuard`. The `--yes` flag MUST map to `full` tier; `--dry-run` MUST map to `read` tier.

#### Scenario: Upgrade preserves behavior

- GIVEN a user upgrades from a version with ConfirmGuard
- WHEN no explicit policy configuration exists
- THEN the system operates in basic mode (blocklist only, no tier restrictions beyond legacy defaults)

#### Scenario: Flag-to-tier mapping

- GIVEN the user runs gaia with `--yes`
- WHEN policy evaluation occurs
- THEN the effective tier is `full`
