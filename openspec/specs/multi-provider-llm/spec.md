# Multi-Provider LLM Specification

## Purpose

Provider-agnostic LLM routing with streaming support and per-provider adapters (OpenAI, Anthropic, Ollama, Copilot).

## Requirements

### Requirement: Provider Interface

The system MUST define a `Provider` port with `Chat(messages, opts) (Response, error)` and `Stream(messages, opts) (TokenStream, error)`. Each adapter MUST implement this port.

#### Scenario: Adapter registration

- GIVEN config selects "openai" as primary provider
- WHEN the system initializes
- THEN the OpenAI adapter is instantiated and registered as the active provider

### Requirement: Config-Driven Routing

The system SHALL select the provider from `config.yaml` (`llm.provider`). The system MUST support fallback: if the primary provider fails, the system SHALL try the next provider in `llm.fallback_chain`.

#### Scenario: Primary provider succeeds

- GIVEN config sets `llm.provider: anthropic`
- WHEN a chat request is sent
- THEN the Anthropic adapter handles the request

#### Scenario: Fallback on failure

- GIVEN `llm.fallback_chain: [openai, ollama]` and OpenAI returns an error
- WHEN a chat request is sent
- THEN the system retries with Ollama and returns its response

### Requirement: Streaming

The system MUST expose streaming as `io.Reader`-based token chunks. Adapters MUST normalize provider-specific SSE formats into this common stream.

#### Scenario: Stream tokens from Anthropic

- GIVEN Anthropic is the active provider
- WHEN a stream request is made
- THEN tokens arrive sequentially via the TokenStream reader

### Requirement: Provider-Specific Adapters

The system MUST provide adapters for OpenAI, Anthropic, Ollama, and Copilot. Each adapter MUST translate internal message format to the provider's API format and back.

#### Scenario: Ollama local model

- GIVEN config sets `llm.provider: ollama` with `llm.model: llama3`
- WHEN a chat request is sent
- THEN the Ollama adapter calls the local REST API with the correct model name
