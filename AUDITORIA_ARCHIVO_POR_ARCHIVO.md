# Plan de Auditoría Quirúrgica Archivo por Archivo (Go Best Practices)

Este documento organiza la revisión exhaustiva y quirúrgica de cada archivo Go de GAIA, aplicando los estándares de los skills oficiales de Golang instalados (`golang-concurrency`, `golang-error-handling`, `golang-code-style`, `golang-context`, `golang-security`, etc.).

---

## 📌 Metodología de Revisión Individual

Cada archivo se evalúa bajo estos 7 vectores clave:
1. **Concurrencia (`golang-concurrency`)**: Goroutines con ciclo de vida definido, canales con dirección explícita, sin data races ni bloqueos de mutex reentrantes.
2. **Contextos y Cancelación (`golang-context`)**: `context.Context` como primer parámetro, propagación adecuada de timeouts y respeto estricto a `ctx.Done()`.
3. **Manejo de Errores (`golang-error-handling`)**: Errores envueltos con `%w`, sin silenciamiento de errores (`_ = err`), sentinels con `errors.Is`/`errors.As`.
4. **Seguridad (`golang-security`)**: Sanitización de inputs, prevención de SSRF, path traversal y límites de buffers/memoria.
5. **Estilo e Idiomatismo (`golang-code-style`, `golang-naming`)**: Principio de responsabilidad única, interfaces pequeñas orientadas al consumidor, nombres concisos.
6. **Recursos y Leaks**: Uso de `defer` inmediato tras `Open`/`Create`/`Dial`, limpieza garantizada de sockets, temporizadores y pools.
7. **Testabilidad**: Ausencia de dependencias ocultas o stubs falsos.

---

## 📂 Inventario de Módulos y Archivos a Auditar (Uno a Uno)

### Módulo 1: Core & Kernel (`internal/core/`)
* [x] `internal/core/kernel.go` (Brain orchestrator, ciclo de iteración, streaming — refactorizado y saneado de concurrencia/SRP)
* [x] `internal/core/session.go` (Enrutamiento multisesión y locks — liberado mutex antes de callbacks para prevenir deadlocks, eliminación de data race sobre currentSessionID con SetActiveConversation, corrección de pánico en slicing de sessionID y suite completa de tests de concurrencia)
* [x] `internal/core/brain_accessors.go` (Acceso concurrente seguro a estado interno y fronteras limpias)
* [x] `internal/core/brain_compaction.go` (Micro-compactación de contexto, rehidratación de sesión `SessionSummary` y desacoplamiento de inyección KG hacia `kernel.go`)
* [x] `internal/core/cost.go` (CostTracker y cálculo de tokens por llamada de modelo — auditado, puramente interno de core)
* [x] Migración `export.go` a `internal/mneme/export.go`: desacoplado del kernel de `internal/core/` (la exportación de memoria a Obsidian pertenece a Mneme).
* [x] `internal/core/guard.go` (ConfirmGuard y PolicyGuard — desacoplado de dependencias externas directas a `mneme`)
* [x] `internal/core/policy_categories.go` (Matriz de categorías y permisos por herramienta — auditado)
* [x] `internal/core/registry.go` (ToolRegistry y caché determinista — auditado)
* [x] `internal/core/usage.go` (Métricas de uso de tokens y ventanas de contexto — auditado)
* [x] Migración ODD / SDD a `internal/agent/`: `odd_router.go`, `sdd_trigger.go`, `sdd_pipeline.go` y `workflow_router.go` desacoplados de `internal/core/` mediante `ports.WorkflowRouter` (IoC sin dependencias cíclicas).
* [x] Migración `autofix` a `internal/agent/autofix.go`: desacoplado de `internal/core/tool_dispatcher.go` mediante `ports.MutationVerifier` (IoC sin llamadas directas a compiladores desde el kernel).
* [x] Unificación y migración de MoA a `internal/agent/moa.go`: eliminación de `internal/core/moa.go` y desacoplamiento mediante `ports.MoARunner`, despachando el comando `/moa` desde `internal/commands/workflow_commands.go`.
* [x] Migración Knowledge Graph sync a `internal/agent/memory/kg_sync.go`: desacoplado de `internal/core/` (`kernel.go`, `tool_dispatcher.go`, `subagent_bridge.go`) mediante `ports.KnowledgeRecorder` (IoC sin llamadas directas a persistencia SQLite desde el kernel).

### Módulo 1.1: Comandos Desacoplados (`internal/commands/`) — *Extraído de Core según BOUNDARIES.md*
* [x] `internal/commands/dispatcher.go` & `dispatcher_test.go` (Implementa `ports.CommandDispatcher`)
* [x] `internal/commands/commands_test.go` (Tests de integración y aislamiento de comandos)
* [x] `internal/commands/session_commands.go`
* [x] `internal/commands/workflow_commands.go`
* [x] `internal/commands/integration_commands.go`
* [x] `internal/commands/config_commands.go`
* [x] `internal/commands/skill_commands.go`
* [x] `internal/commands/info_commands.go`
* [x] `internal/commands/media_commands.go`

### Módulo 2: Herramientas y Ejecutores (`internal/tools/`)
* [x] `internal/tools/catalog.go` (Catálogo de herramientas built-in y `run_tests` multi-runtime con detección automática de raíz de proyecto)
* [x] `internal/tools/tool_search.go` (Búsqueda semántica y por categoría de herramientas)
* [x] `internal/tools/load_tool.go` (Carga dinámica bajo demanda de esquemas de herramientas)
* [x] `internal/tools/read_skill.go` (Lectura progresiva de SKILL.md según estándar agentskills.io)
* [x] `internal/tools/create_skill.go` (Creación y actualización de procedural skills)
* [x] `internal/tools/memory_search.go` (Búsqueda en memoria a largo plazo Mnemosyne)
* [x] `internal/tools/web_fetch.go` & `internal/tools/web/fetch/*` (Fetch web seguro con parser HTML a Markdown y protección SSRF estricta contra IPs privadas)
* [x] `internal/tools/web_search.go` & `internal/tools/web/search/*` (Búsqueda multi-proveedor: DuckDuckGo, Brave, Tavily, SearXNG con fallback)
* [x] `internal/tools/browser/*` (Automatización headless CLI con detección de navegadores y sandboxing)

### Módulo 3: Módulos del Sistema / Sandboxes (`internal/modules/`)
* [x] `internal/modules/codemode/engine.go` & `sdk.go` (Sandbox Yaegi CodeAct y SDK pre-importado)
* [x] `internal/modules/fileops/fileops.go` & `patch.go` (Operaciones de archivos y `apply_patch` exact/fuzzy)
* [x] `internal/modules/shell/shell.go`, `executor.go`, `docker.go`, `ssh.go` (Ejecución shell segura con allowlist y backends locales/docker/ssh)
* [x] `internal/modules/gitops/gitops.go` (Operaciones Git de solo lectura con aislamiento de directorio)
* [x] `internal/modules/security/security.go` (Validación de path traversal, prevención SSRF y redacción de secretos)
* [x] `internal/modules/multimodal/multimodal.go` (Inspección de imágenes y audio sin inflar contexto primario)
* [x] `internal/modules/blackboard/module.go` (L1 Blackboard Ledger y snapshots)

### Módulo 4: Adaptadores de Entrada/Salida (`internal/adapters/`)
* [x] `internal/adapters/web/server.go` (Servidor Web UI, CSRF y SSE)
* [x] `internal/adapters/tui/tui.go` & `completer.go` (Interfaz Bubbletea TUI)
* [x] `internal/adapters/db/sqlite.go` (Persistencia SQLite y seguridad de queries)
* [x] `internal/adapters/db/knowledge.go` (Base de hechos del Knowledge Graph)
* [x] `internal/adapters/db/cron.go` & `task_repo.go` (Repositorios de tareas y cron)
* [x] `internal/adapters/llm/pool.go` & `router.go` (Pool de credenciales y enrutamiento LLM)
* [x] `internal/adapters/llm/anthropic.go`, `openai.go`, etc. (Adaptadores de proveedores)
* [x] `internal/adapters/audio/*` (STT Whisper y TTS ElevenLabs/OpenAI)
* [x] `internal/adapters/desktop/*` (Integración Desktop/Wails)

### Módulo 5: Gateways de Mensajería (`internal/gateway/`)
* [x] `internal/gateway/multiplexer.go` (Multiplexor concurrente de gateways)
* [x] `internal/gateway/gateway.go` & `adapter.go` (Interfaces y ciclo de vida)
* [x] `internal/gateway/telegram.go` (Adaptador Telegram)
* [x] `internal/gateway/discord.go` (Adaptador Discord)
* [x] `internal/gateway/slack.go` (Adaptador Slack)
* [x] `internal/gateway/whatsapp.go` & `whatsapp_mcp_adapter.go` (Adaptador WhatsApp directo y vía MCP)
* [x] `internal/gateway/signal_mcp_adapter.go` (Adaptador Signal vía MCP)
* [x] `internal/gateway/approver.go` (Flujo de aprobación remota y auto-deny)
* [x] `internal/gateway/media.go` (Descarga y normalización de medios multimodales)

### Módulo 6: Memoria y Base de Conocimiento (`internal/mneme/`)
* [x] `internal/mneme/hybrid.go` & `rrf.go` (Fusión híbrida FTS5 + Vectores)
* [x] `internal/mneme/sqlite.go` & `fts.go` (Motor SQLite Mnemosyne)
* [x] `internal/mneme/curator.go` & `summarizer.go` (Curación y compactación de memoria)
* [x] `internal/mneme/export.go` (Exportación de memoria a Obsidian)

### Módulo 7: Subagentes, SDD y Review (`internal/agent/`, `internal/review/`, `internal/subagents/`)
* [x] `internal/agent/sdd/*` (Pipeline SDD y ejecución por fases)
* [x] `internal/agent/ops/*` (Supervisión y métricas de subagentes)
* [x] `internal/review/engine.go` & `judgment/*` (Revisión adversarial y compuertas)
* [x] `internal/subagents/distill.go` & `ephemeral_repo.go` (Destilación de contexto y repositorio efímero)

### Módulo 8: Comandos CLI y Servidor (`cmd/gaia/`)
* [x] `cmd/gaia/main.go` (Punto de entrada y dispatch de CLI — Composition Root desacoplado)
* [x] `cmd/gaia/serve.go` (Servidor API REST / OpenAI compatible con fallback lazy)
* [x] `cmd/gaia/gateway.go` (Daemon de gateways y control de ciclo de vida / PID)
* [x] `cmd/gaia/exec.go`, `review.go`, `policy.go`, etc. (Subcomandos CLI puros sin stubs ni mocks)

---

## Estado Actual de la Revisión

* **Total de Módulos**: 8 de 8 (100% Completados)
* **Archivos Clave Auditados**: ~75 archivos principales (100% libres de mocks y acoplamientos indebidos en código productivo)
* **Suite de Tests Global**: `go test ./...` pasando al 100% en verde.
