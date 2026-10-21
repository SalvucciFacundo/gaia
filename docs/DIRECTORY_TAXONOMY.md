# Guía de Taxonomía y Límites de Paquetes en GAIA

Esta guía define qué responsabilidad pertenece a cada carpeta del repositorio y las reglas estrictas de dependencias para evitar acoplamientos indebidos o código desubicado.

---

## 🏛️ Regla de Oro Hexagonal

```
     cmd/gaia (Composition Root: conecta todo)
        │
        ├──> internal/adapters (Infraestructura concreta: DB, LLM, Web, TUI)
        ├──> internal/gateway  (Mensajería externa: Telegram, Discord, Slack, WhatsApp)
        ├──> internal/modules  (Sandboxes del SO: shell, fileops, gitops, codemode)
        ├──> internal/tools    (Herramientas invocables por el LLM)
        ├──> internal/agent    (Estrategias autónomas: ODD, SDD, autofix, MoA, spawner)
        ├──> internal/commands (Comandos slash de usuario: /session, /config, etc.)
        ├──> internal/review   (Revisión adversarial y compuertas CAS)
        ├──> internal/mneme    (Memoria persistente: SQLite FTS5 + Vectores)
        │
        └──> internal/core     (Motor neutro, dominio y puertos)
                 ├── domain/   (Entidades y value objects puros)
                 └── ports/    (Interfaces que la infraestructura implementa)
```

**Regla de dependencia inward**:
- `internal/core/` **NUNCA** debe importar paquetes externos (`adapters`, `gateway`, `modules`, `agent`, `commands`, `review`).
- El Core solo expone interfaces en `internal/core/ports/`.
- Las implementaciones concretas viven en las capas externas e implementan dichos puertos (Inversión de Control).

---

## 📂 Taxonomía Directorio por Directorio

### 1. `internal/core/` (Núcleo de Ejecución Neutro)
- **¿Qué va acá?**: El ciclo fundamental de turnos (`kernel.go`), routing multisesión (`session.go`), matriz de permisos y seguridad (`guard.go`, `policy.go`), presupuesto de tokens (`cost.go`, `usage.go`), registro neutro de tools (`registry.go`) y modelos de dominio (`domain/`).
- **¿Qué NO va acá?**: Comandos slash (van a `internal/commands/`), compiladores o herramientas de auto-reparación (van a `internal/agent/`), pipelines de desarrollo de software (ODD/SDD van a `internal/agent/`), llamadas a bases de datos directas o clientes HTTP.

### 2. `internal/commands/` (Comandos Slash de Presentación/Usuario)
- **¿Qué va acá?**: Toda la lógica que responde a comandos iniciados por el usuario con barra inclinada (`/session`, `/config`, `/info`, `/skill`, `/media`, `/workflow`, `/integration`).
- **Contrato**: Implementa `ports.CommandDispatcher`.

### 3. `internal/agent/` (Estrategias Autónomas y Supervisión)
- **¿Qué va acá?**:
  - `spawner.go`: Spawning y aislamiento de subagentes en worktrees de Git.
  - `workflow_router.go`: Enrutador de flujos ODD / SDD (implementa `ports.WorkflowRouter`).
  - `sdd/`: Pipeline formal Spec-Driven Development (explorador, diseñador, especificador, implementador TDD, verificador, archivador).
  - `ops/`: Agentes especializados (debugger, reviewer, researcher, learner).
  - `autofix.go`: Verificador de mutaciones y ciclo de auto-reparación de compilador (implementa `ports.MutationVerifier`).
  - `moa.go`: Mixture-of-Agents fanout concurrente (implementa `ports.MoARunner`).
  - `memory/kg_sync.go`: Sincronización del grafo de conocimiento (implementa `ports.KnowledgeRecorder`).

### 4. `internal/tools/` (Catálogo y Herramientas Built-in para el LLM)
- **¿Qué va acá?**: Las definiciones y ejecutores de herramientas que el modelo puede solicitar en su array de tools:
  - `catalog.go`: Registro central y búsqueda semántica de herramientas.
  - `web_fetch.go` y `web_search.go`: Navegación e inspección web con protección SSRF.
  - `read_skill.go`, `create_skill.go`: Carga progresiva e interactiva de skills.
  - `browser/`: Automatización headless CLI de navegadores.

### 5. `internal/modules/` (Sandboxes del Sistema Operativo)
- **¿Qué va acá?**: Módulos que interactúan con el entorno local de forma aislada y segura:
  - `fileops/`: Lectura, escritura y aplicación quirúrgica de diffs/patches (`file_patch`).
  - `shell/`: Ejecución de comandos terminal bajo allowlist y aislamiento en Docker/SSH/Local.
  - `gitops/`: Operaciones de repositorio Git de sólo lectura y estado.
  - `codemode/`: Sandbox de ejecución de código en memoria (Yaegi).
  - `security/`: Sanitización de path traversal, SSRF y redacción de secretos.
  - `multimodal/`: Inspección de imágenes y audio.
  - `blackboard/`: Snapshotting y ledger L1.

### 6. `internal/adapters/` (Infraestructura Concreta)
- **¿Qué va acá?**: Adaptadores que implementan los puertos de salida (`ports/*`):
  - `db/`: Persistencia SQLite (`sqlite.go`), repositorios de tareas, cron y knowledge graph.
  - `llm/`: Proveedores de modelos (Anthropic, OpenAI, DeepSeek, Copilot, etc.) y pool de credenciales.
  - `tui/`: Interfaz de usuario de terminal en Bubbletea.
  - `web/`: Servidor Web dashboard (templ + htmx).
  - `audio/`: STT (Whisper) y TTS (ElevenLabs / OpenAI).
  - `desktop/`: Integración nativa de escritorio (Wails).

### 7. `internal/gateway/` (Gateways de Mensajería Externa)
- **¿Qué va acá?**: Conectores para plataformas de mensajería que normalizan eventos entrantes a `ports.IncomingMessage`:
  - `multiplexer.go`: Enrutador y despachador concurrente de gateways.
  - `telegram.go`, `discord.go`, `slack.go`, `whatsapp.go`, `signal_mcp_adapter.go`.
  - `approver.go`: Aprobaciones interactivas remotas.

### 8. `internal/mneme/` (Motor de Memoria Mnemosyne)
- **¿Qué va acá?**: Persistencia a largo plazo de conversaciones, hechos del usuario y observaciones:
  - `sqlite.go`: Conexión SQLite con WAL, busy timeout y `SetMaxOpenConns(1)`.
  - `fts.go`: Búsqueda léxica FTS5.
  - `vector.go` y `hybrid.go`: Búsqueda vectorial densa y cosine similarity.
  - `rrf.go`: Fusión híbrida de rangos recíprocos (Reciprocal Rank Fusion).
  - `curator.go` y `summarizer.go`: Compactación y destilación de turnos.
  - `export.go`: Exportación hacia Obsidian.

### 9. `internal/review/` (Revisión Adversarial y Compuertas de Entrega)
- **¿Qué va acá?**:
  - `engine.go`: Máquina de estados de revisión y clasificación de riesgo.
  - `judgment/`: Protocolo de árbitros ciegos A y B con correcciones quirúrgicas.
  - `gates/`: Compuertas pre-commit, pre-push, pre-pr y `CASReceiptStore` (almacenamiento en Git Object Database).

### 10. `cmd/gaia/` (Punto de Entrada y Composition Root)
- **¿Qué va acá?**: Binario ejecutable de GAIA:
  - `main.go`: Parseo de flags, arranque de TUI o despacho a subcomandos CLI.
  - `serve.go`: Daemon de API REST compatible con OpenAI.
  - `gateway.go`: Daemon para ejecución en segundo plano de mensajería multiplexada.
  - Subcomandos de control: `exec.go`, `review.go`, `policy.go`, `skill.go`, `profile.go`, `backup.go`, `doctor.go`, `update.go`.

---

## 🚦 Matriz Rápida de Decisión: "¿Dónde pongo mi archivo?"

| Tipo de Componente | Carpeta Destino | Ejemplo |
| :--- | :--- | :--- |
| Nuevo comando slash que escribe el usuario (`/foo`) | `internal/commands/` | `foo_commands.go` |
| Nueva herramienta invocable por el LLM en el prompt | `internal/tools/` | `foo_tool.go` |
| Nuevo conector de mensajería (ej. Matrix, Teams) | `internal/gateway/` | `matrix.go` |
| Nuevo proveedor de LLM (ej. Mistral, Cohere) | `internal/adapters/llm/` | `mistral.go` |
| Lógica del ciclo de vida de turnos o límites del kernel | `internal/core/` | `kernel.go`, `guard.go` |
| Nueva entidad o struct de datos pura de dominio | `internal/core/domain/` | `models.go` |
| Nueva interfaz para desacoplar infraestructura | `internal/core/ports/` | `ports.go` |
| Pipeline automatizado de ingeniería o subagente especializado | `internal/agent/` o `internal/agent/sdd/` | `optimizer.go` |
| Operación con el sistema de archivos o SO bajo sandbox | `internal/modules/` | `container.go` |
| Almacenamiento o búsqueda de memoria semántica | `internal/mneme/` | `indexer.go` |
