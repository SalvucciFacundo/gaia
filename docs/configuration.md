# Configuration Reference

GAIA is configured via `~/.gaia/config.yaml`. A template is created on first run.

---

## Full Configuration

```yaml
# ~/.gaia/config.yaml

# ── LLM Provider Configuration ──────────────────────────────
llm:
  default_provider: anthropic      # Default provider for all subagents
  default_model: claude-sonnet-4-20250514

# ── Per-Subagent Model Configuration ───────────────────────
# Each subagent can use a different provider and model.
# Omitted subagents use the default llm config.
# You can repeat the same model with different reasoning_effort.
subagents:
  designer:
    provider: anthropic
    model: claude-sonnet-4-20250514
    fallback: claude-haiku-3-5-20241022
    reasoning_effort: high           # high, medium, low

  implementer:
    provider: openai
    model: gpt-4o
    fallback: gpt-4o-mini
    reasoning_effort: medium

  explorer:
    provider: openrouter
    model: qwen/qwen3-30b-a3b:free
    reasoning_effort: low

  verifier:
    provider: anthropic
    model: claude-haiku-3-5-20241022
    reasoning_effort: low

  reviewer:
    provider: anthropic
    model: claude-opus-4-20250514
    reasoning_effort: high

# ── Persona Configuration ───────────────────────────────────
persona:
  orchestrator_seed: teacher        # teacher, professional, strict, friendly, custom
  evolution_enabled: true           # false = freeze persona, never evolve
  evolution_review: prompt          # prompt = ask before evolving, auto = evolve silently
  language: auto                    # auto, es, en, pt, etc.
  custom_file: ~/.gaia/persona.md   # Custom persona file (SOUL.md compatible)

# ── Skills Configuration ───────────────────────────────────
skills:
  dir: ~/.gaia/skills
  auto_install_wizard: true
  taps: []
    # - name: community-go
    #   url: https://github.com/user/skills

# ── Memory Configuration ───────────────────────────────────
memory:
  engram_enabled: true
  knowledge_graph: true
  nudge_interval: 10                 # Prompt memory save every N tool calls

# ── Context Engineering ────────────────────────────────────
context:
  max_history: 20                    # Max recent messages to keep
  compaction_threshold: 50           # Compact after N messages
  recall_enabled: true               # Enable memory recall per turn
  kg_recall_enabled: true            # Enable knowledge graph recall

# ── Terminal Backends ──────────────────────────────────────
terminal:
  backend: local                     # local, docker, ssh
  timeout: 180
  docker:
    container: ubuntu                # Container name for docker backend
    image: ubuntu:22.04              # Image for auto-start
  ssh:
    host: ""
    port: 22
    user: ""
    key_path: ~/.ssh/id_rsa

# ── Security ───────────────────────────────────────────────
security:
  confirmation_mode: always          # always, per-session, per-action, never
  secret_redaction: true             # Auto-redact keys from messages
  path_restriction: true             # Restrict file access to project root
  url_validation: true               # Validate URLs before fetch
  skill_audit: true                  # AST-audit skills before loading
  allowed_hosts: []                  # URL whitelist (empty = no restriction)
    # - github.com
    # - api.github.com
  blocked_paths:                     # Never allow access to these paths
    - ~/.ssh
    - ~/.gnupg
  keychain: auto                     # auto, os-keychain, encrypted-file, plaintext

# ── User Interface ─────────────────────────────────────────
ui:
  mode: tui                          # tui, desktop
  theme: rose-pine                   # TUI theme

# ── MCP Servers ────────────────────────────────────────────
mcp:
  servers: []
    # - name: my-server
    #   command: npx
    #   args: ["-y", "@modelcontextprotocol/server-filesystem"]
    #   env:
    #     MY_VAR: value

# ── Cron Jobs ──────────────────────────────────────────────
cron:
  jobs: []

# ── Gateway (Messaging) ────────────────────────────────────
gateway:
  enabled: false
  telegram:
    token: ""
  discord:
    enabled: false

# ── Webhooks ───────────────────────────────────────────────
webhook:
  port: 8765
  hmac_secret: ""

# ── LSP Servers ────────────────────────────────────────────
lsp:
  servers: []
    # - language: go
    #   command: gopls
    #   args: []
    # - language: python
    #   command: pylsp
    #   args: []

# ── Script Injection ───────────────────────────────────────
scripts:
  pre_process: []
    # - path: ~/.gaia/scripts/check-env.sh
    #   always: false               # true = run every turn, false = per agent run
```

---

## Environment Variables

| Variable | Override | Default |
|---|---|---|
| `OPENAI_API_KEY` | `llm.provider=openai` API key | — |
| `ANTHROPIC_API_KEY` | `llm.provider=anthropic` API key | — |
| `GEMINI_API_KEY` | `llm.provider=gemini` API key | — |
| `OLLAMA_URL` | Ollama server URL | `http://localhost:11434` |
| `GITHUB_TOKEN` | Copilot auth token | — |
| `GAIA_CONFIG` | Config file path | `~/.gaia/config.yaml` |
| `GAIA_HOME` | GAIA data directory | `~/.gaia` |
| `GAIA_SKILLS_DIR` | Skills directory | `~/.gaia/skills` |
| `GAIA_LOG_LEVEL` | Log level | `info` |
