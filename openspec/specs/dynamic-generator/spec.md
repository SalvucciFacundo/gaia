# Dynamic Subagent Generator Specification

## Purpose

Runtime creation and management of user-defined subagents without code changes or recompilation. Dynamic subagents are configured via structured definitions persisted in SQLite and satisfy the existing Subagent interface.

## Requirements

### Requirement: Subagent Definition

The system MUST provide a `SubagentDef` struct with fields: Name (unique identifier), Description (non-empty), AllowedTools ([]string, validated against registered tools), Skills ([]string), SystemPrompt (string), Personality (string), and CreatedAt (timestamp). Definitions MUST be persisted in a SQLite `subagent_defs` table with JSON columns for slice fields. The system MUST support CRUD operations: CreateDef, GetDef, ListDefs, UpdateDef, DeleteDef.

#### Scenario: Create a definition

- GIVEN no subagent named "sql-optimizer" exists
- WHEN CreateDef is called with a valid SubagentDef
- THEN the definition is persisted to SQLite
- AND GetDef returns the created definition

#### Scenario: Name uniqueness

- GIVEN a subagent named "sql-optimizer" already exists
- WHEN CreateDef is called with Name: "sql-optimizer"
- THEN the operation fails with a duplicate name error

#### Scenario: Validate tool references

- GIVEN AllowedTools contains "nonexistent_tool"
- WHEN CreateDef is called
- THEN the operation fails with a validation error listing unknown tools

#### Scenario: Delete a definition

- GIVEN a dynamic subagent "sql-optimizer" exists
- WHEN DeleteDef is called with "sql-optimizer"
- THEN the definition is removed from SQLite and unregistered from the Registry

### Requirement: Dynamic Subagent Runtime

The system MUST provide a `DynamicSubagent` struct that implements the `Subagent` interface. On Execute(), it MUST set the task's AllowedTools from its SubagentDef, build a system prompt from the def's SystemPrompt and Personality fields, and delegate to Spawner.RunLoop(). A `DynamicSubagentFactory` MUST construct a DynamicSubagent from a SubagentDef. The Registry MUST support `RegisterDynamic(name, SubagentDef)` which creates and registers a factory. On startup, the system MUST load all SubagentDefs from SQLite and register each dynamically.

#### Scenario: Execute with dynamic subagent

- GIVEN a dynamic subagent "k8s-debugger" with AllowedTools: [shell_exec, file_read]
- WHEN Execute is called with a task
- THEN the task runs with only shell_exec and file_read available
- AND the system prompt includes the def's SystemPrompt and Personality

#### Scenario: Startup reload

- GIVEN two dynamic subagents are persisted in SQLite
- WHEN the application starts
- THEN both subagents are loaded and registered in the Registry
- AND they are available for @routing and delegation

#### Scenario: Dynamic subagent satisfies Subagent interface

- GIVEN a DynamicSubagent is registered
- WHEN the Spawner invokes it like any compiled subagent
- THEN it executes normally with no special-case handling

### Requirement: Subagent Interview

The system MUST provide a `/create-agent` command in the Brain that triggers a wizard-style interview flow via the TUI. The interview MUST collect: name, description, tools (multi-select from available tools), skills, and personality (free-text). The interview MUST support back-navigation between steps. On completion, the system MUST create a SubagentDef, persist it to SQLite, register it dynamically, and confirm to the user: "Subagent 'X' created and active. Type @X to chat with it."

#### Scenario: Complete interview flow

- GIVEN the user types "/create-agent"
- WHEN the user answers all interview steps and confirms
- THEN a SubagentDef is created and persisted
- AND the subagent is immediately available for use

#### Scenario: Back navigation

- GIVEN the user is on the "tools" step of the interview
- WHEN the user selects "back"
- THEN the interview returns to the "description" step
- AND previous answers are preserved

#### Scenario: Validation during interview

- GIVEN the user enters an empty name
- WHEN the interview validates the input
- THEN an error is shown and the user MUST correct it before proceeding

### Requirement: Per-Subagent Engram Namespace

Each dynamic subagent MUST receive its own Engram namespace. The namespace key format MUST be `gaia/subagent/{name}/`. The namespace MUST be created on registration and used during Spawner.RunLoop() for all memory operations within that subagent's execution.

#### Scenario: Namespace isolation

- GIVEN dynamic subagents "alpha" and "beta"
- WHEN "alpha" saves a memory observation
- THEN the observation is stored under "gaia/subagent/alpha/"
- AND "beta" cannot access alpha's observations

#### Scenario: Namespace created on registration

- GIVEN a new dynamic subagent "gamma" is registered
- WHEN the registration completes
- THEN the Engram namespace "gaia/subagent/gamma/" exists and is ready for use
