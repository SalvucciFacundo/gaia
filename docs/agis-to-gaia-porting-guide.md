# Plan Maestro y Guía de Porting: AGIS ➔ GAIA

Este documento detalla el inventario completo, mapeo de componentes y plan de ejecución por fases para portar e integrar las capacidades de **AGIS** (Autonomous Go Intelligent System) dentro del ecosistema unificado de **GAIA**, alimentando principalmente el perfil `assistant` y enriqueciendo el core de GAIA.

---

## 1. Inventario y Mapeo de Componentes

| Componente AGIS | Estado / Ubicación en GAIA | Estrategia de Porting |
| :--- | :--- | :--- |
| **`internal/memory` (Mnemosyne)** | **PORTADO** (`internal/mneme/`) | Ya completado. Portado con SQLite, FTS5, búsqueda vectorial/híbrida RRF, user model y migración 0008 de proyectos. |
| **`internal/persona`** | **PENDIENTE** (`internal/persona/`) | Portar `soul.go`, `evolution.go`, `overlay.go`, `SOUL.default.md`. Configura la personalidad base, adaptabilidad continua y overlays del Asistente. |
| **`internal/gateway` (Mensajería)** | **ACTUALIZACIÓN MAYOR** (`internal/gateway/`) | GAIA tiene adaptadores mock/mcp simples. AGIS tiene integraciones completas: WhatsApp (`whatsmeow`), Telegram (`telegram.go`), Discord (`discord.go`), Slack (`slack.go`), media/audio multimodal y multiplexer. |
| **`internal/session`** | **PENDIENTE** (`internal/session/`) | Portar `manager.go` multi-canal: enrutamiento de conversaciones por plataforma (WhatsApp vs CLI vs Telegram) y vinculación con memoria. |
| **`internal/tools`** | **INTEGRACIÓN EN PERFIL** (`internal/profiles/assistant.go` & `internal/tools/`) | Herramientas de asistente: `web_search.go` (DuckDuckGo/SearXNG), `web_fetch.go`, `read_skill.go`, `create_skill.go`, `memory_search.go`. Integrarlas nativamente en el perfil `assistant`. |
| **`internal/subagents` (Destilación)** | **INTEGRACIÓN EN AGENT** (`internal/agent/`) | `ephemeral_repo.go`, `distill.go`, `engine.go`. Subagentes con repositorios efímeros en RAM y síntesis destilada de resultados. |
| **`internal/backup`** | **PORTAR A GAIA** (`internal/backup/` & CLI) | Snapshotting completo de SQLite (`mneme.db`), configuraciones, skills y rollback/restore verificado con sha256 y manifiesto. |
| **`internal/server` (HTTP API)** | **EVALUAR / EXTENDER** (`internal/server/` o CLI `serve`) | Endpoints REST/SSE para streaming de chat, CORS y autenticación API token. Conectar con el perfil activo. |
| **`internal/updater`** | **PORTAR O INTEGRAR** (`internal/updater/`) | Auto-actualización atómica binaria con verificación de checksums, backup previo y rollback automático. |

---

## 2. Plan de Porting en 5 Fases

```
┌────────────────────────────────────────────────────────┐
│ FASE 1: Persona & Identidad (SOUL & Evolución)         │
│   - Port internal/persona                              │
│   - Integrar en Perfil Assistant & Prompts             │
└──────────────────────────┬─────────────────────────────┘
                           │
┌──────────────────────────▼─────────────────────────────┐
│ FASE 2: Tools de Asistente General                     │
│   - Web Search & Web Fetch                             │
│   - Memory Search & Skill Learning en caliente         │
│   - Registro en AssistantProfile                       │
└──────────────────────────┬─────────────────────────────┘
                           │
┌──────────────────────────▼─────────────────────────────┐
│ FASE 3: Sesiones Multi-Canal & Gateway Robusto         │
│   - Multi-channel Session Manager                      │
│   - Port de WhatsApp (Whatsmeow) & Telegram            │
│   - Multiplexer y soporte de audio/multimodal          │
└──────────────────────────┬─────────────────────────────┘
                           │
┌──────────────────────────▼─────────────────────────────┐
│ FASE 4: Subagentes Efímeros & Destilación              │
│   - Ephemeral memory / repos para agentes aislados     │
│   - Motor de destilación de respuestas complejas       │
└──────────────────────────┬─────────────────────────────┘
                           │
┌──────────────────────────▼─────────────────────────────┐
│ FASE 5: Operaciones de Producción (Backup & Updater)   │
│   - Backup & Restore con manifiesto SHA256             │
│   - Auto-update atómico con rollback                   │
│   - Subcomandos CLI correspondientes                   │
└────────────────────────────────────────────────────────┘
```

---

## 3. Desglose Detallado por Fase

### Fase 1: Persona & Identidad del Asistente
- **Archivos a portar**:
  - `internal/persona/soul.go`: Carga y validación de `SOUL.md`.
  - `internal/persona/overlay.go`: Capas de contexto dinámico (día/noche, humor, tono profesional vs coloquial).
  - `internal/persona/evolution.go`: Ajuste evolutivo de directrices basado en feedback del usuario.
  - `internal/persona/SOUL.default.md`: Identidad por defecto.
- **Integración**: Conectar en `internal/profiles/assistant.go` como generador del `SystemPrompt()`.

### Fase 2: Herramientas del Asistente Personal
- **Archivos a portar**:
  - `internal/tools/web_search.go`: Búsqueda web abierta (DuckDuckGo HTML scraping / SearXNG) sin API keys obligatorias.
  - `internal/tools/web_fetch.go`: Extracción y parseo de contenido HTML a markdown legible.
  - `internal/tools/memory_search.go`: Búsqueda directa sobre el motor `mneme` recién portado.
  - `internal/tools/create_skill.go` & `read_skill.go`: Auto-aprendizaje de skills guardadas localmente.
- **Integración**: Exponer estas herramientas en el método `Tools()` de `AssistantProfile`.

### Fase 3: Sesiones Multi-canal y Gateways de Mensajería
- **Archivos a portar**:
  - `internal/session/manager.go`: Mapeo de `ChannelID` (`whatsapp:+549...`, `telegram:12345`) a conversaciones aisladas de GAIA.
  - `internal/gateway/whatsapp.go`: Conexión directa vía protocolo nativo WhatsApp (`whatsmeow`).
  - `internal/gateway/telegram.go`: Bot de Telegram con polling y webhook.
  - `internal/gateway/discord.go` y `slack.go`: Canales de comunidad y equipos.
  - `internal/gateway/multiplexer.go` y `media.go`: Transcripción de audios de voz (Whisper / Local) y multiplexado de mensajes entrantes.

### Fase 4: Subagentes Efímeros y Destilación
- **Archivos a portar**:
  - `internal/subagents/ephemeral_repo.go`: Contexto de memoria volátil para tareas secundarias que no deben contaminar la base global hasta ser aprobadas.
  - `internal/subagents/distill.go`: Algoritmo de condensación que resume la salida de múltiples herramientas o subagentes antes de emitir la respuesta al canal.

### Fase 5: Mantenimiento, Backup y Actualizaciones
- **Archivos a portar**:
  - `internal/backup/backup.go` y `restore.go`: Respaldo completo de la base SQLite de Mnemosyne, configs y estado.
  - `cmd/gaia/backup.go`: Subcomandos `gaia backup create`, `gaia backup restore`, `gaia backup list`.
  - `internal/updater/`: Cliente de actualización binaria en caliente con verificación de firmas/hashes.

---

## 4. Criterios de Aceptación por Fase
- Cada fase debe incluir tests unitarios pasando al 100% en `go test ./...`.
- Cero acoplamiento indebido: el perfil `dev` y `art` no deben verse afectados por las herramientas o gateways del `assistant`.
- Toda la configuración nueva debe mapearse limpiamente en `domain.Config` y `config.yaml`.
