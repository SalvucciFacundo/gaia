# Tool Engine Specification

## Purpose

Tool execution registry with Module interface, built-in tools (shell, file, git), path validation, and structured results.

## Requirements

### Requirement: Tool Registry

The system MUST maintain a registry mapping tool names to implementations. The Brain SHALL look up tools by name when the LLM issues a tool_call.

#### Scenario: Tool lookup

- GIVEN the registry contains "shell_exec" and "file_read"
- WHEN the LLM calls "shell_exec"
- THEN the registry returns the shell_exec implementation

#### Scenario: Unknown tool

- GIVEN the registry has no "unknown_tool"
- WHEN the LLM calls "unknown_tool"
- THEN the system returns an error result to the LLM

### Requirement: Module Interface

Each tool MUST implement `Module` with `Name() string`, `Description() string`, `Parameters() Schema`, and `Execute(ctx, params) (Result, error)`.

#### Scenario: Shell module execution

- GIVEN the shell module is registered
- WHEN Execute is called with `{command: "ls"}`
- THEN the command runs and returns stdout/stderr

### Requirement: Built-in Tools

The system MUST provide shell (command execution), file (read/write/list), and git (status/log/diff) tools.

#### Scenario: File read

- GIVEN the file module is registered
- WHEN Execute is called with `{action: "read", path: "/tmp/test.txt"}`
- THEN the file contents are returned

### Requirement: Path Validation

The system MUST validate all file paths against an allowlist. The system MUST reject paths outside allowed directories.

#### Scenario: Path outside allowlist

- GIVEN allowlist is `[/home/user/project]`
- WHEN a tool receives path `/etc/passwd`
- THEN the system rejects the operation with a validation error

### Requirement: Result Format

Tool results MUST be returned as structured `{success: bool, output: string, error: string}` for LLM consumption.

#### Scenario: Successful result

- GIVEN a tool executes successfully
- WHEN the result is returned
- THEN `success: true` and `output` contains the tool output
