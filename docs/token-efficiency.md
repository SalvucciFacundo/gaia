# Token Efficiency

GAIA uses a **knowledge graph** to recall only relevant context per turn, saving **70%+ tokens** on long sessions compared to traditional agent replay.

---

## The Problem

Traditional agents replay the **entire conversation** every turn:

| Session Length | Traditional | GAIA |
|---|---|---|
| 10 messages | ~5k tokens | ~8.5k tokens |
| 50 messages | ~25k tokens | ~8.5k tokens |
| 100 messages | ~50k tokens | ~8.5k tokens |
| 500 messages | ~200k+ tokens (overflow) | ~8.5k tokens |

At 500 messages, traditional agents hit context limits. GAIA stays within a fixed budget.

---

## Per-Turn Context Budget

```
System Prompt (fixed)               ~2k tokens
+ Active Skills Index (Level 0)     ~3k tokens
+ Knowledge Graph Recall            ~500 tokens
+ Recent Messages (last 5)          ~2k tokens
+ Compacted Summary                 ~1k tokens
─────────────────────────────────────────
TOTAL per turn:                    ~8.5k tokens
```

---

## Mechanisms

### Knowledge Graph Recall

**Cost**: ~500 tokens per turn
**Saving**: Grows with session length

Before each LLM call, GAIA:
1. Extracts key concepts from the current context
2. Queries the knowledge graph for related facts
3. Injects relevant facts into the system prompt

### Context Compaction

**Cost**: ~1k tokens per turn
**Saving**: Caps prompt size on long sessions

When messages exceed the compaction threshold (default 50):
1. Stale messages are summarized into a compacted history
2. Only the last 5 messages remain verbatim
3. The compacted summary is injected instead of replaying all history

### Progressive Skills

**Cost**: ~3k tokens (Level 0 index)
**Saving**: Only loads what's needed

Skills are loaded in levels:
- **Level 0** (always in context): `[{name, description, tags}, ...]` — ~3k tokens
- **Level 1** (on demand): Full `SKILL.md` content
- **Level 2** (on demand): Specific reference files

### Memory Recall

**Cost**: ~0 tokens (no LLM involved)
**Saving**: Fewer exploration turns

FTS5 search across past observations retrieves relevant decisions and facts. No LLM call needed.

### Session Search

**Cost**: ~0 tokens (FTS5, no LLM)
**Saving**: Fast recall without context cost

Search past sessions for similar problems or solutions.

---

## Knowledge Graph Structure

```text
Topic (e.g., "Authentication System")
├── Concept (e.g., "JWT Token Flow")
│   ├── Fact (e.g., "Tokens expire after 24h, refresh every 7d")
│   ├── Fact (e.g., "Secret stored in AUTH_SECRET env var")
│   └── Fact (e.g., "Middleware validates on /api/* routes")
├── Concept (e.g., "Session Management")
│   ├── Fact (e.g., "Redis store, 30min TTL")
│   └── Fact (e.g., "Sessions invalidated on password change")
```

Each fact has:
- Labels (semantic tags)
- Summary (LLM-generated)
- Embedding vector (for semantic search)
- Edges to related concepts

---

## Token Budget Per Subagent

Each subagent has a configured token budget for its LLM calls. When the budget is low:

1. **Compact** oldest messages in the subagent's context
2. **Fall back** to a cheaper model (if configured)
3. **Return** partial results with a summary of remaining work

This prevents any single subagent from consuming the entire API budget.
