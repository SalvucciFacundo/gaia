# Context Layers — How GAIA Manages Conversation History

GAIA uses three complementary layers to manage conversation context. Each layer serves a different purpose, and together they balance **cost** (token usage) against **fidelity** (not losing important details).

---

## The Three Layers

```mermaid
flowchart TD
    subgraph L1 ["LAYER 1: Recent Messages (Verbatim)"]
        L1_Desc["• Last N messages complete and untouched (Default: 10 messages ~3-5k tokens)\n• Zero loss fidelity — preserves every nuance\n• Configured via budget.keep_recent_messages"]
    end

    subgraph L2 ["LAYER 2: Compacted History (Auto Summary)"]
        L2_Desc["• Messages beyond the recent window are condensed via LLM summary\n• Activates when history exceeds compaction_threshold (Default: 50 messages)\n• Preserves architectural decisions, context, and conversation flow"]
    end

    subgraph L3 ["LAYER 3: Knowledge Graph (Structured Facts)"]
        L3_Desc["• Key technical facts extracted from responses and stored in SQLite\n• Relevant facts dynamically injected before each prompt\n• Default: Opt-in (enable via /kg on)"]
    end

    L1 --> L2
    L2 --> L3
```

---

## How It Works at Each Turn

```mermaid
sequenceDiagram
    autonumber
    actor User as 👤 User
    participant Orch as 🧠 GAIA Orchestrator
    participant L1 as ⚡ Layer 1 (Recent)
    participant L2 as 📜 Layer 2 (Summary)
    participant L3 as 🗄️ Layer 3 (KG Facts)
    participant LLM as 🤖 LLM Model

    User->>Orch: Sends prompt or command
    activate Orch
    Orch->>L1: Retrieve last N messages verbatim
    Orch->>L2: Get compacted summary (if threshold > 50)
    Orch->>L3: Query relevant technical facts (if /kg on)
    Orch->>LLM: Assemble & stream prompt [System + Summary + KG + Recent + Prompt]
    deactivate Orch
    activate LLM
    LLM-->>User: Streams real-time token response
    deactivate LLM
    Orch->>L1: Append turn to history
    Orch->>L3: Extract & persist technical indicators to SQLite
```

---

## Layer 3: Knowledge Graph Recall

### When to Enable

| Scenario | Recommended | Reason |
|---|---|---|
| **Short sessions (<50 messages)** | `OFF` | Layer 1 + 2 is enough |
| **Long sessions (>100 messages)** | `ON` | Saves tokens on repeated context |
| **Detailed debugging** | `OFF` | Don't risk losing subtle details |
| **Research / exploration** | `ON` | Facts help recall past findings |
| **Cost-sensitive (API costs)** | `ON` | Significantly reduces token usage |

### What Gets Extracted

The system looks for lines containing technical indicators:

- `uses` — *"The system uses JWT for authentication"*
- `implements` — *"implements the Repository pattern"*
- `migrate` / `changed` / `refactored` — *"migrated from MySQL to PostgreSQL"*
- `decision:` — *"decision: use refresh tokens"*
- `recommend` — *"recommend 7-day expiry"*
- `configured` — *"configured with 25 connection pool"*

### What Does NOT Get Extracted

- Code blocks (``` ... ```)
- Headers (# Title)
- Lines under 30 characters
- Generic chat ("OK", "sure", "let me check")

---

### In-Session Commands

```bash
/kg          # Show status and fact count
/kg on       # Enable KG recall for this session
/kg off      # Disable KG recall
/kg stats    # Show facts grouped by topic
/kg clear    # Clear all stored facts
```

### Configuration Example

```yaml
# ~/.config/gaia/config.yaml
budget:
  keep_recent_messages: 10    # Layer 1: messages kept verbatim
  compaction_threshold: 50    # Layer 2: when to compact
  keep_recent_messages: 20    # Messages kept after compaction
```

---

## Layers Comparison

| Aspect | Layer 1 (Recent) | Layer 2 (Compaction) | Layer 3 (KG) |
|---|---|---|---|
| **Fidelity** | 100% | ~80% (summary) | ~30% (facts only) |
| **Token cost** | High (grows) | Low (fixed) | Very low (~500 tokens) |
| **Always on** | Yes | Yes | No (opt-in) |
| **Controls** | `keep_recent_messages` | `compaction_threshold` | `/kg on/off` |
| **Best for** | Recent context | Historical context | Cross-session recall |

---

## Trade-off Summary

- **More layers = more context but higher token budget.**
- **Fewer layers = cheaper but potentially less relevant historical context.**

The default configuration (Layer 1 + 2) is the best balance for most development sessions. Enable Layer 3 when working on large codebases across long sessions to maximize token efficiency.
