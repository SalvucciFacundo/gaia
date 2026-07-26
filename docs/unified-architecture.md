# Unified Gateway/TUI Architecture

## Status: Design Proposal — Not Implemented

---

## 1. Problem Statement

Today GAIA runs as two separate processes:
- `gaia` — TUI (Bubbletea) with a Brain instance
- `gaia gateway start` — Gateway (Telegram/Discord/Slack) with a **different** Brain instance

This means:
- Messages in TUI are invisible to Telegram and vice versa
- `/handoff` saves to SQLite but requires manually switching contexts
- No real continuity between devices — it's like talking to 3 different agents

## 2. Target Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    GAIA (single process)                  │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │              BRAIN (única instancia)                │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │  │
│  │  │ TUI      │ │ Gateway  │ │ Session Manager  │   │  │
│  │  │ Bubbletea│ │ Manager  │ │                  │   │  │
│  │  │          │ │          │ │ Controla qué     │   │  │
│  │  │          │ │ ┌──────┐ │ │ sesión está      │   │  │
│  │  │          │ │ │Tele  │ │ │ activa y cómo    │   │  │
│  │  │          │ │ │gram  │ │ │ rutea mensajes   │   │  │
│  │  │          │ │ ├──────┤ │ │                  │   │  │
│  │  │          │ │ │Disc  │ │ │ 3 modos:         │   │  │
│  │  │          │ │ │ord   │ │ │ - unify(default) │   │  │
│  │  │          │ │ ├──────┤ │ │ - isolate       │   │  │
│  │  │          │ │ │Slack │ │ │ - ask           │   │  │
│  │  │          │ │ └──────┘ │ │                  │   │  │
│  │  └──────────┘ └──────────┘ └──────────────────┘   │  │
│  └────────────────────────────────────────────────────┘  │
│                           │                               │
│                    SQLite (compartida)                     │
└───────────────────────────────────────────────────────────┘
```

## 3. Session Manager — El Corazón

### 3.1 Concepto

El Session Manager decide **dónde va cada mensaje** cuando llega desde cualquier plataforma. Tiene 3 modos:

### 3.2 Modes

#### Mode: `unify` (default)

Un solo contexto global. TODOS los mensajes de TODAS las plataformas van a la misma sesión.

```
TUI:      "refactoriza auth a JWT"
Telegram: "acordate de comprar leche"

Historial:
  [tui] refactoriza auth a JWT
  [telegram] acordate de comprar leche
```

**Ideal para**: una sola persona trabajando en un proyecto desde múltiples dispositivos.

#### Mode: `isolate`

Cada plataforma tiene su propia sesión independiente. Comportamiento idéntico a Hermes.

```
TUI:      "refactoriza auth a JWT"       → Sesión "tui-default"
Telegram: "acordate de comprar leche"    → Sesión "telegram-default"

Cada sesión tiene su propio historial, su propio contexto.
```

**Ideal para**: casos de uso completamente diferentes por plataforma.

#### Mode: `ask` (o "smart prompt")

Cuando un mensaje llega desde una plataforma diferente a la activa, GAIA pregunta:

```
[Telegram] ¿Querés continuar donde dejaste en la TUI o empezar algo nuevo?

1. Continuar sesión principal ("refactorizar auth a JWT")
2. Nueva conversación en Telegram
3. Recordármelo después (encolar)
```

**Esta es la idea clave**: el usuario elige en el momento, no está forzado a una decisión de configuración.

### 3.3 Implementación

```go
type SessionMode string
const (
    SessionUnify    SessionMode = "unify"
    SessionIsolate  SessionMode = "isolate"
    SessionAsk      SessionMode = "ask"
)

type SessionManager struct {
    mode        SessionMode
    activeID    string     // sesión activa global
    platformIDs map[string]string // platform → sessionID (para isolate)
    brain       *Brain
}

func (sm *SessionManager) Route(ctx context.Context, platform string, content string) error {
    switch sm.mode {
    case SessionUnify:
        // Todos los mensajes van a la misma sesión
        return sm.brain.ProcessMessage(ctx, "["+platform+"] "+content)

    case SessionIsolate:
        // Cada plataforma tiene su sesión
        sessID, ok := sm.platformIDs[platform]
        if !ok {
            sessID = createSession(platform)
            sm.platformIDs[platform] = sessID
        }
        sm.repo.SetSessionID(sessID)
        return sm.brain.ProcessMessage(ctx, content)

    case SessionAsk:
        // Preguntar al usuario la primera vez que llega de otra plataforma
        if sm.activeID != "" && sm.lastPlatform != platform {
            return sm.promptUser(platform, content)
        }
        return sm.brain.ProcessMessage(ctx, content)
    }
}
```

## 4. Mecanismo de "Smart Prompt" (Mode: ask)

### Flujo

```
T=0s:  Usuario en TUI → "refactoriza auth a JWT"
       Brain procesa, sesión activa = "main"

T=30s: Usuario desde Telegram → "cómo va eso?"
       SessionManager detecta: plataforma diferente, sesión activa
       → Responde al usuario:
         ┌─────────────────────────────────────────┐
         │  📱 Llegó un mensaje desde Telegram      │
         │                                          │
         │  La sesión activa es:                    │
         │    "refactorizar auth a JWT" (TUI)       │
         │                                          │
         │  ¿Cómo manejamos este mensaje?           │
         │                                          │
         │  [1] Unificar — sigue la misma sesión     │
         │  [2] Separar — nueva sesión para Telegram │
         │  [3] Preguntar siempre (default)         │
         │  [4] Usar esta respuesta como default    │
         └─────────────────────────────────────────┘

T=31s: Usuario elige [1] Unificar
       → "cómo va eso?" se agrega al mismo contexto
       → El agente responde considerando TODO el historial
```

### Recordar la decisión

Si el usuario elige "Unificar" 3 veces seguidas desde Telegram, el sistema puede auto-detectar la preferencia y dejar de preguntar para esa plataforma.

```go
type PlatformPreference struct {
    Platform    string
    LastMode    SessionMode  // unify o isolate
    AskCount    int
    AutoCount   int
}
```

## 5. Mensajes con prefijo de plataforma

Para que el agente entienda de dónde viene cada mensaje en modo `unify`:

```
[telegram] María: acordate de comprar leche
[tui] refactoriza el módulo auth a JWT
[discord] Juan: revisa esta PR porfa
```

Esto le da contexto al LLM para responder apropiadamente según la plataforma y el remitente.

## 6. PolicyGuard por Plataforma

Cada plataforma puede tener un tier distinto:

```yaml
policy:
  platforms:
    tui:
      tier: full
    telegram:
      tier: sandbox
    discord:
      tier: read
```

TUI confianza total, Telegram operaciones normales, Discord solo lectura. El PolicyGuard ya soporta esto — solo falta configurarlo por plataforma.

## 7. Prioridad de Implementación

```
Phase 1: Un solo proceso, TUI + Gateway simultáneos
  - main.go arranca ambos si hay gateways configurados
  - El Brain se comparte

Phase 2: Session Manager con modo unify
  - Todos los mensajes a la misma sesión
  - Prefijo de plataforma en los mensajes

Phase 3: Modo ask (smart prompt)
  - Detectar cambio de plataforma
  - Preguntar al usuario qué hacer
  - Recordar preferencias

Phase 4: PolicyGuard por plataforma
  - Configurar tiers distintos según la interfaz
```

## 8. Modo Legacy

`gaia gateway start` y `gaia` (sin args) siguen funcionando exactamente como hoy. El nuevo modo es opt-in:

```bash
gaia --unify                    # TUI + Gateway juntos
gaia --unify --session=ask      # Con smart prompt
```

O auto-detectado si hay gateways configurados en config.yaml.
