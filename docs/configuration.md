# GAIA Configuration Reference

GAIA is configured via a single YAML file loaded from:
1. `--config <path>` flag (highest priority)
2. `$GAIA_CONFIG` environment variable
3. `$GAIA_HOME/config.yaml`
4. `~/.config/gaia/config.yaml` (default fallback)

---

## 📄 Complete Annotated `config.yaml`

```yaml
# ── Active Profile & Profiles Configuration ──────────────────────
# Defines which profile is active by default and customizes per-profile tools and models.
current_profile: "dev"                # "dev", "assistant", "art", or any custom profile ID

profiles:
  dev:
    enabled: true
    model: "claude-3-5-sonnet-20241022" # Optional: overrides default LLM model for this profile
    tools:                             # Omit to use default profile tools, or customize explicitly:
      - exec_command
      - read_file
      - write_file
      - browser_navigate
      - browser_action
      - browser_screenshot
      - web_search
      - web_fetch
      - create_skill
      - read_skill

  assistant:
    enabled: true
    model: "gpt-4o"
    tools:
      - web_search
      - web_fetch
      - browser_navigate
      - read_file
      - read_skill

  art:
    enabled: false
    model: "gemini-2.5-flash"
    tools:
      - browser_navigate
      - browser_screenshot
      - read_file

# ── Primary LLM & Routing ────────────────────────────────────────
llm:
  provider: "anthropic"                # "anthropic", "openai", "deepseek", "ollama", "groq", "openrouter", etc.
  model: "claude-3-5-sonnet-20241022"
  reasoning_effort: "medium"           # "low", "medium", "high" (supported on OpenAI o1/o3-mini & GitHub Copilot)
  persona: "default"                   # "default", "teacher", "friendly", "strict", "professional"
  fallback_chain:                      # Providers to try when encountering rate limits or 429 errors
    - "openai"
    - "deepseek"
    - "ollama"
  trust_mode: "interactive"            # "interactive", "always", "paranoid"

# ── API Keys Pool ────────────────────────────────────────────────
# Keys can also be injected via environment variables (e.g., OPENAI_API_KEY).
api_keys:
  anthropic: "sk-ant-..."
  openai: "sk-proj-..."
  deepseek: "sk-..."
  groq: "gsk_..."
  elevenlabs: "eleven_..."

# ── OpenAI-Compatible REST & SSE API Server ──────────────────────
# Controls `gaia serve` / `gaia api` behavior.
server:
  host: "0.0.0.0"                      # Bind address (use 127.0.0.1 for local only)
  port: 8080                           # Server port
  api_key: ""                          # Optional: Require Bearer token authentication
  cors_origins:                        # Allowed CORS origins
    - "*"
  read_timeout: 30s
  write_timeout: 120s

# ── Multi-Channel Messaging Gateways ─────────────────────────────
# Controls `gaia gateway run` behavior.
gateway:
  enabled: true

  # Telegram Bot
  telegram:
    enabled: true
    token: "123456789:ABCDefgh..."
    allowlist:                         # Authorized Telegram usernames or user IDs
      - "123456789"
      - "myusername"

  # WhatsApp Meta Cloud API
  whatsapp:
    enabled: false
    api_token: "EAAB..."               # Permanent System User Access Token
    phone_number_id: "10987654321"     # WhatsApp Business Phone Number ID
    verify_token: "my-webhook-secret"  # Webhook Challenge Token for Meta webhook setup
    app_secret: "a1b2c3d4..."          # Meta App Secret for HMAC-SHA256 signature verification
    listen_addr: ":3003"               # Webhook listening port
    allowed_users:                     # Authorized WhatsApp phone numbers (E.164 format)
      - "+1234567890"

  # Discord Bot
  discord:
    enabled: false
    token: "MTE..."                    # Discord Bot Token
    allowlist:
      - "user#1234"

  # Slack Events API
  slack:
    enabled: false
    bot_token: "xoxb-..."              # Bot User OAuth Token
    signing_secret: "sec_..."          # Slack Signing Secret
    listen_addr: ":3002"
    allowed_users: []

# ── Multimodal Perception & Audio Engine ─────────────────────────
multimodal:
  enabled: true
  api_key: ""                          # Defaults to api_keys.openai if empty

  # Inbound Audio Transcription (Whisper STT)
  audio:
    enabled: true
    provider: "openai"                 # "openai", "whisper-local", "groq"
    model: "whisper-1"
    max_audio_size_mb: 25

  # Vision (Image Inspection)
  vision:
    enabled: true
    provider: "openai"
    model: "gpt-4o"
    max_image_size_mb: 10

# Outbound Speech Synthesis (Voice Notes)
tts:
  enabled: true
  provider: "openai"                   # "openai", "elevenlabs"
  model: "tts-1"                       # "tts-1", "tts-1-hd", or elevenlabs model
  voice: "alloy"                       # OpenAI: alloy, echo, fable, onyx, nova, shimmer
                                       # ElevenLabs: voice ID (e.g. 21m00Tcm4TlvDq8ikWAM)
  speed: 1.0
  format: "mp3"                        # "mp3", "opus", "aac", "flac"

# ── Headless Browser Automation (MCP) ────────────────────────────
# Configured under gateway.browser_tools. Points to the browser MCP server (e.g. puppeteer/playwright).
gateway:
  browser_tools:
    enabled: true
    command: "npx @modelcontextprotocol/server-puppeteer"

# ── Persistence & Database ───────────────────────────────────────
db:
  path: "~/.config/gaia/gaia.db"       # Main session, state, and facts database

# ── Policy & Security ────────────────────────────────────────────
policy:
  tier: "sandbox"                      # "read" (read-only), "sandbox" (safe dev), "full" (unrestricted)
  blocked_commands:
    - "rm -rf /"
    - "mkfs"
    - "dd if=/dev/zero"

system:
  requires_confirmation: true          # Prompt human before executing high-risk commands
  language: "auto"                     # Preferred assistant interaction language ("es", "en", "auto")

# ── Execution Backends ───────────────────────────────────────────
terminal:
  backend: "local"                     # "local", "docker", "ssh"
  docker:
    container: "gaia-sandbox"
    workdir: "/workspace"
  ssh:
    host: "dev-server.internal"
    port: 22
    user: "developer"
    key_path: "~/.ssh/id_rsa"
```

---

## 🔑 Environment Variables Reference

Any configuration value can be configured or overridden using standard environment variables:

| Environment Variable | Description | Default |
|---|---|---|
| `GAIA_CONFIG` | Explicit path to `config.yaml` | `~/.config/gaia/config.yaml` |
| `GAIA_HOME` | GAIA data home directory | `~/.config/gaia` |
| `OPENAI_API_KEY` | OpenAI API key for LLM, Whisper, and TTS | — |
| `ANTHROPIC_API_KEY` | Anthropic API key for Claude models | — |
| `DEEPSEEK_API_KEY` | DeepSeek API key | — |
| `GROQ_API_KEY` | Groq API key for fast inference / Whisper | — |
| `ELEVENLABS_API_KEY` | ElevenLabs API key for speech synthesis | — |
| `OLLAMA_URL` | Base URL for local Ollama server | `http://localhost:11434` |
| `GAIA_LOG_LEVEL` | Log verbosity (`debug`, `info`, `warn`, `error`) | `info` |

---

## 💡 Configuration Recipes

### 1. Developer Workstation (Browser + Voice + Sandbox)
```yaml
current_profile: "dev"
llm:
  provider: "anthropic"
  model: "claude-3-5-sonnet-20241022"
gateway:
  browser_tools:
    enabled: true
    command: "npx @modelcontextprotocol/server-puppeteer"
policy:
  tier: "sandbox"
```

### 2. Autonomous Gateway Daemon (WhatsApp + Telegram + Audio)
```yaml
current_profile: "assistant"
gateway:
  enabled: true
  telegram:
    enabled: true
    token: "${TELEGRAM_BOT_TOKEN}"
    allowlist: ["${TELEGRAM_USER_ID}"]
  whatsapp:
    enabled: true
    api_token: "${WHATSAPP_API_TOKEN}"
    phone_number_id: "${WHATSAPP_PHONE_ID}"
    verify_token: "${WHATSAPP_VERIFY_TOKEN}"
    app_secret: "${WHATSAPP_APP_SECRET}"
    listen_addr: ":3003"
    allowed_users: ["+1234567890"]
multimodal:
  audio:
    enabled: true
    provider: "openai"
tts:
  enabled: true
  provider: "openai"
  voice: "alloy"
```

### 3. OpenAI-Compatible API Server (for Cursor / Open WebUI)
```yaml
server:
  host: "0.0.0.0"
  port: 8080
  api_key: "secret-bearer-key"
  cors_origins: ["*"]
current_profile: "dev"
```

