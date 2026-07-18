# Agent Loop Specification

## Purpose

Connected Brain ↔ TUI ↔ LLM message flow with streaming display and session persistence.

## Requirements

### Requirement: Message Flow

The system MUST route user input from TUI → `Brain.ProcessMessage()` → LLM call → response → TUI display. The Brain SHALL orchestrate the full turn including tool-call loops.

#### Scenario: User sends prompt

- GIVEN the TUI is active and the user types "hello"
- WHEN the message is submitted
- THEN Brain.ProcessMessage is called, the LLM responds, and the TUI renders the response

#### Scenario: Tool call in response

- GIVEN the LLM returns a tool_call in its response
- WHEN the Brain processes the response
- THEN the tool is executed and the result is fed back to the LLM for continuation

### Requirement: Streaming Display

The system MUST render LLM streaming tokens in the TUI in real-time. Partial tokens MUST appear as they arrive without blocking the UI.

#### Scenario: Streaming render

- GIVEN an LLM stream is active
- WHEN tokens arrive
- THEN the TUI appends each token to the current message view

### Requirement: Session Persistence

The system SHALL persist conversation history per session. On restart, the Brain MUST restore the session's message history.

#### Scenario: Session restore

- GIVEN a session with 5 prior messages
- WHEN the application restarts
- THEN the Brain loads the 5 messages and continues the conversation
