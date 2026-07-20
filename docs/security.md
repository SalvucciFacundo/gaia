# Security Model

GAIA is a programming agent with access to file system, terminal, git, and the web. Security is applied at multiple layers to protect against both accidental and malicious threats.

---

## API Key & Secret Storage

| Risk | Mitigation |
|---|---|
| Keys stored in plain text | Keys stored in OS keychain (Windows Credential Manager, macOS Keychain, Linux secret-service) or encrypted config file |
| Keys leaked in conversation | Automatic redaction of `sk-*`, `ghp_*`, `Bearer *`, and custom patterns from messages and tool output |
| Keys leaked in tool output | Redaction engine scans all tool stdout/stderr before returning to LLM |
| Subagent access to secrets | **Secret scoping**: each subagent only sees the secrets it needs |

API keys can be configured via:
1. **Environment variables** — `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, etc.
2. **Config file** — `~/.gaia/config.yaml` (auto-redacted from output)
3. **OS keychain** — Recommended for production

---

## Confirmation Modes

GAIA supports 4 confirmation levels:

| Mode | Behavior | Best for |
|---|---|---|
| **always** (default) | Ask before every dangerous operation | New users, learning the agent |
| **per-session** | Ask once per session; all subsequent ops auto-confirm | Daily coding sessions |
| **per-action** | Confirm only the current action; next action asks again | One-off commands |
| **never** (YOLO) | Never ask; all operations execute without confirmation | CI/CD, automation, experienced users |

Change modes in-session:

```
/trust session      → Trust all actions this session
/trust once         → Trust only the next action
/trust always       → Revert to always-ask mode
/trust never        → YOLO mode — no confirmations
```

Configure the default in `config.yaml`:

```yaml
security:
  confirmation_mode: always
```

Headless mode (`gaia exec`) respects confirmation mode. If `always`, headless operations block unless `--yes` is passed.

---

## Tool Execution Security

| Risk | Mitigation |
|---|---|
| Shell injection via tool arguments | All shell commands use parameterized execution (no string interpolation). Path and argument validation before execution. |
| Path traversal (accessing files outside project) | `ValidatePath()` resolves symlinks, validates against an allowed root. |
| URL safety (SSRF, malicious endpoints) | `ValidateURL()` blocks private IP ranges, localhost, internal services by default. |
| Dangerous commands (rm -rf, dd, etc.) | Threat pattern detection flags known dangerous command patterns. Require explicit override. |
| Fork bombs / resource exhaustion | Iteration budget caps per subagent. Timeout per command. Max output size limit. |

### Shell Allowlist

The shell module maintains an allowlist of safe commands:

```
git, go, npm, npx, pnpm, yarn, cargo, rustc, python, python3,
node, deno, bun, make, cmake, mvn, gradle, docker, kubectl,
curl, wget, tar, zip, unzip, gzip, cat, head, tail, grep,
awk, sed, sort, cut, tr, wc, find, xargs, echo, printf, whoami
```

Commands not on the allowlist require explicit user confirmation.

---

## Skill Security

| Risk | Mitigation |
|---|---|
| Malicious skill loading | **Skill provenance**: track origin (official hub, community tap, user-created). **AST audit**: parse skill files for dangerous patterns before loading. |
| Skill exfiltration | Skills run in a restricted context with defined read/write scope. Network access gated by tool permissions. |
| Skill privilege escalation | Skills cannot modify other skills or GAIA's own config. Skills cannot disable security features. |

Audit installed skills:

```bash
gaia audit skills           # Scan for dangerous patterns
gaia skills list --verbose  # Show provenance for each skill
```

---

## Git Security

| Risk | Mitigation |
|---|---|
| Committing secrets | Pre-commit hook checks for credentials, tokens, keys. |
| Force push | Disabled by default. Requires explicit override. |
| Committing to protected branches | Blocked unless explicitly overridden by user. |

Audit your project for committed secrets:

```bash
gaia audit secrets
```

---

## Communication Security

| Risk | Mitigation |
|---|---|
| Man-in-the-middle on LLM API calls | TLS required for all provider connections. Certificate verification enabled. |
| MCP server security | MCP servers run as subprocesses with restricted permissions. Not exposed to the network. |
| Webhook HMAC | Webhook subscriptions verified via HMAC-SHA256 signature. |

---

## Security Audit Commands

```bash
gaia doctor              # Check security config, key storage, permissions
gaia audit secrets       # Scan project for committed secrets
gaia audit skills        # Scan installed skills for dangerous patterns
gaia security log        # Show security-relevant events (approvals, denials)
```
