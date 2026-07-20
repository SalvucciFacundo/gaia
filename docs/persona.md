# Persona System

GAIA's persona system controls how the agent communicates. Unlike traditional SOUL.md files that define fixed behavior, GAIA personas are **starting points that evolve** based on user interaction.

> A persona that tells the agent exactly how to behave prevents learning.
> The persona sets initial tone and values; the learning loop refines them.

---

## How Personas Evolve

```
Session 1:  Persona "Strict" → "No acepto código sin tests"
               ↓
Session 10: The agent learned that integration tests are more
            valuable than unit tests for THIS project
               ↓
Session 50: "I check that tests cover happy path + 2 edge cases.
            For APIs, prioritize integration tests."
```

The learning loop tracks:
- What communication styles get the best results for each user
- Which feedback patterns catch more bugs
- What level of detail the user prefers per context
- When to push back vs when to comply (learned from user reactions)

---

## Built-in Personas

| Persona | Seed Behavior | Can Evolve To |
|---|---|---|
| **Teacher** | Warm but firm. Explains WHY, not just what. Corrects with fundamentals. | Discovers user prefers examples → adapts |
| **Professional** | Neutral, direct, efficient. No personality overlay. | User responds to encouragement → becomes warmer |
| **Strict** | Demanding. No code without tests, error handling, types. | Learns which rules matter for THIS project → nuanced strictness |
| **Friendly** | Relaxed, conversational, encouraging. | User needs more direct feedback → balances warmth with honesty |
| **Custom** | Defined by user via SOUL.md-compatible file. | User's seed gets refined by experience |

---

## Custom Persona (SOUL.md Compatible)

Create a custom persona file:

```markdown
# GAIA Persona Seed — Senior Rustacean

## Starting Tone
- Direct and precise, like a senior Rust engineer
- Short responses unless asked for details
- This is a seed — expect it to evolve with use

## Core Values (evolve with experience)
- Correctness over speed
- Type safety is non-negotiable

## Communication Preferences
- Start with the conclusion, then justify
- Suggest safer alternatives
```

```yaml
# ~/.gaia/config.yaml
persona:
  orchestrator_seed: custom
  custom_file: ~/.gaia/rust-persona.md
```

---

## Per-Subagent Personas

Each subagent starts with a seed persona matching its role, **and evolves independently**:

| Subagent | Seed Persona | Evolves Based On |
|---|---|---|
| **Orchestrator** | As selected by user | User reactions, correction patterns |
| **Explorer** | Curious, thorough | Which search patterns find relevant code faster |
| **Proposer** | Structured, clear | Which proposal formats get approved more |
| **Specifier** | Precise, exhaustive | Which detail level catches requirement gaps |
| **Designer** | Architect, pragmatic | Which design patterns work vs cause rework |
| **Implementer** | Focused, productive | Which coding patterns cause fewer bugs |
| **Verifier** | Skeptical, thorough | Which test types catch regressions |
| **Reviewer** | Strict, constructive | Which review comments actually prevent bugs |
| **Debugger** | Analytical, methodical | Which root cause patterns repeat |

---

## Freezing a Persona

If you want the agent to STOP evolving (keep a fixed personality):

```yaml
persona:
  evolution_enabled: false   # Freeze — never change behavior
```

When frozen, the persona becomes a static instruction set (like traditional SOUL.md).

---

## Quick Switching

```
/persona strict          → Switch seed to strict
/persona teacher         → Switch seed to teacher
/persona freeze          → Stop evolution, keep current persona
/persona unfreeze        → Re-enable evolution
/persona reset           → Reset to seed (clear all learned adaptations)
/persona status          → Show current persona + evolution state
/persona custom my-rust  → Load custom seed from file
```
