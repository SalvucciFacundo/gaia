# Token Efficiency & Hybrid Search

GAIA uses a **Knowledge Graph + FTS5 & Vector Hybrid Search** to recall only relevant context per turn, saving **70%+ tokens** on long sessions compared to traditional agent prompt replay.

---

## The Problem

Traditional agents replay the **entire conversation history** every turn, creating exponential token cost growth:

| Session Length | Traditional Agent | GAIA (Fixed Budget) |
|---|---|---|
| **10 messages** | ~5k tokens | ~8.5k tokens |
| **50 messages** | ~25k tokens | ~8.5k tokens |
| **100 messages** | ~50k tokens | ~8.5k tokens |
| **500 messages** | ~200k+ tokens *(overflow / crash)* | **~8.5k tokens** |

At 500 messages, traditional agents overflow their context windows. GAIA stays within a strict, predictable budget per turn.

---

## Per-Turn Context Budget

```mermaid
pie title Per-Turn Context Budget (~8.5k Tokens Total)
    "Active Skills Index (Level 0)" : 3000
    "System Prompt (fixed)" : 2000
    "Recent Messages (verbatim)" : 2000
    "Compacted Summary" : 1000
    "Knowledge Graph & Recall" : 500
```

---

## Context Optimization Mechanisms

### 1. Hybrid Search (FTS5 + Vector Embeddings)

- **Cost**: `0 LLM Tokens` (Executed locally via SQLite)  
- **Benefit**: Precise context extraction without expensive filtering LLM calls.

GAIA combines **SQLite FTS5 full-text keyword search** with **semantic vector embeddings** to retrieve historical facts, decisions, and codebase observations. This hybrid approach guarantees that relevant memories are retrieved even if exact keywords differ, without consuming LLM budget for context discovery.

### 2. Knowledge Graph Recall

- **Cost**: `~500 tokens` per turn  
- **Saving**: Scales context efficiency on long-running sessions.

Before each turn, GAIA:
1. Extracts active concepts from the user message.
2. Queries the Knowledge Graph using hybrid search.
3. Injects relevant semantic facts directly into the system prompt.

### 3. Context Compaction

- **Cost**: `~1k tokens` per turn (on compaction execution)  
- **Saving**: Caps prompt size permanently on long sessions.

When conversation length crosses the compaction threshold (default: 50 messages):
1. Older messages are summarized into a structured executive summary.
2. Only the last 5 messages are kept verbatim.
3. The compacted summary replaces raw message history in context.

### 4. Progressive Skill Loading

- **Cost**: `~3k tokens` (Level 0 index)  
- **Saving**: Prevents loading unused instructions into memory.

- **Level 0** (always in context): Short skill manifests `[{name, description, tags}]` (~3k tokens).
- **Level 1** (on demand): Full `SKILL.md` content loaded only when triggered.
- **Level 2** (on demand): Specific reference files.

### 5. Reminis Blackboard Working Memory & Context Destruction
- **Cost**: `0 Prompt Tokens` (Direct L1 RAM pass-by-reference)
- **Saving**: Eliminates $O(N^2)$ context explosion across subagents.

Inspired by [**Reminis**](https://github.com/SalvucciFacundo/Reminis), GAIA avoids passing parent conversational history to subagent workers:
1. Subagents receive only declared `InputKeys` from the in-memory **Blackboard**.
2. Large outputs $> 4\text{ KB}$ automatically spill over to local `.bin` files, passing only lightweight path references.
3. Subagents run with a hard 3-turn bound, push results to `OutputKeys`, and their message history is instantly garbage-collected upon completion (`defer func() { messages = nil }()`).

### 6. Go-Native Code Mode (CodeAct) with Yaegi
- **Saving**: Cuts multi-turn roundtrips by 80%.

Instead of executing 10 separate tool calls across 10 sequential LLM turns (e.g., read file, parse regex, filter lines, write result), GAIA uses **Yaegi** (`execute_code`) to execute a concise Go script that runs all operations programmatically in a single turn.

### 7. Read-Safe Parallel Tool Calling Batches
- **Saving**: Eliminates serial latency on multi-tool turns.

When the LLM requests multiple read-only tools (e.g., `file_read`, `file_tree`, `web_search`), GAIA partitions the batch and executes independent reads concurrently via Go routines while keeping state-mutating actions strictly sequential.

### 8. Surgical Search & Replace (`file_patch` / `apply_patch`)
- **Saving**: Saves 70%–80% of tokens on file modifications compared to full `file_write`.
- **Fuzzy Matching**: Tolerates whitespace, indentation (tabs vs. spaces), and newline differences without failing.
- **Closed-Loop Compiler Auto-Repair**: Whenever source files are modified, GAIA triggers an immediate background compiler/linter check (`go vet`, `cargo check`, `tsc --noEmit`, `ruff`). If syntax or type errors are introduced, GAIA injects the exact compiler output back into the turn so the LLM self-heals before concluding.

---

## Knowledge Graph Structure

```mermaid
graph TD
    Topic["📌 Topic: Authentication System"]
    
    C1["💡 Concept: JWT Token Flow"]
    C2["💡 Concept: Session Management"]
    
    F1["📝 Fact: Tokens expire after 24h, refresh every 7d"]
    F2["📝 Fact: Secret stored in AUTH_SECRET env var"]
    F3["📝 Fact: Middleware validates on /api/* routes"]
    
    F4["📝 Fact: Redis store, 30min TTL"]
    F5["📝 Fact: Sessions invalidated on password change"]
    
    Topic --> C1
    Topic --> C2
    
    C1 --> F1
    C1 --> F2
    C1 --> F3
    
    C2 --> F4
    C2 --> F5
```

Each fact contains:
- **Labels**: Semantic tags.
- **Summary**: Concise description.
- **FTS5 Index**: Fast keyword lookup.
- **Embedding Vector**: Dense vector representation for semantic similarity.

---

## Token Budget Per Subagent

Subagents operate under isolated token budgets. When a subagent's token budget approaches its threshold:
1. The subagent automatically summarizes its working memory.
2. Only terminal artifacts (e.g. `proposal.md`, `spec.md`, `design.md`) are returned to the parent session.
3. Intermediate reasoning traces are discarded, preventing context pollution.
