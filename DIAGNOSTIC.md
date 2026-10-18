# GAIA — Diagnóstico del Proyecto

> Generado: 2026-10-18  
> Estado: análisis estático + ejecución de tests + lectura de código fuente

---

## ✅ Lo que funciona bien

- **Build limpio**: `go build ./...` → exit 0. Sin errores de compilación.
- **Test suite**: 45+ paquetes con tests, casi todos pasan. Cobertura en `core`, `agent`, `gateway`, `mneme`, `skills`, `delivery`, `review`, etc.
- **Arquitectura hexagonal real**: ports/adapters bien separados. El `Brain` recibe sus dependencias por constructor.
- **Yaegi CodeAct implementado**: `internal/modules/codemode/` — Yaegi integrado, `GaiaSDK` con `ReadFile`, `WriteFile`, `Grep`, `Exec`, `Find`. Los panics son intencionales (atrapados por el sandbox del engine).
- **`go vet` limpio**: Sin advertencias estáticas.
- **Los 12 subagentes registrados**: 8 SDD + 4 ops, cableados al Brain con namespace de memoria aislado.
- **Gateways múltiples**: Telegram, Discord, Slack, WhatsApp con multiplexer unificado.
- **MoA (Mixture of Agents)**: Fan-out a múltiples providers wired en Brain.
- **goleak en tests**: Hay preocupación activa por goroutine leaks.

---

## 🔴 Bugs activos — atacar primero

### BUG-1: Tamper detection rota en backup [SOLUCIONADO]

**Archivo**: `internal/backup/backup_test.go:204-225` / `internal/backup/restore.go`

**Estado**: Solucionado. Se agregó el drenado completo del `gzip.Reader` (`io.Copy(io.Discard, gr)`) tras el loop de extracción del tarball para forzar la validación de integridad (CRC32/checksum) del stream gzip antes de aplicar o procesar el manifest. El test `TestBackup_Restore_ChecksumMismatchRollback` ahora pasa exitosamente.

---

### BUG-2: `SetReasoningEffort` y `SetPersona` son stubs vacíos [SOLUCIONADO]

**Archivo**: `internal/core/kernel.go:182-193` / `internal/core/ports/ports.go` / `internal/adapters/llm/`

**Estado**: Solucionado.
- Se agregó `ReasoningEffort` a `ports.ChatOptions` y la functional option `ports.WithReasoningEffort(level)`.
- Se cableó la propagación de `reasoning_effort` en los adapters OpenAI (`req.ReasoningEffort`) y Copilot (`payload["reasoning_effort"]`).
- `SetReasoningEffort` en `Brain` ahora almacena y valida el nivel, y el loop de ejecución lo pasa a cada llamada `Stream`/`Chat`.
- `SetPersona` en `Brain` ahora resuelve el overlay de comportamiento mediante `persona.NewOverlays(nil)` y se inyecta dinámicamente en el contexto del historial (`getHistory`).
- Se ampliaron los presets de personalidad incorporados en `persona/overlay.go` con `professional`, `strict` y `friendly`.

---

## 🟠 Features incompletas — segunda prioridad

### INCOMPLETE-1: Judgment Day CLI no está cableado [SOLUCIONADO]

**Archivo**: `cmd/gaia/review.go:184` / `internal/review/judgment/`

**Estado**: Solucionado.
- Se cableó la ejecución de `--judgment-day` en `gaia review start`.
- Configura el `spawner` con subagentes (`reviewer`, `implementer`, etc.), inicia la transacción con `review.NewEngine(projectRoot, nil).Start(files)` e invoca `judgment.NewJudgmentDay(spawner).Run(ctx, tx)`.
- Imprime el reporte de rondas, hallazgos de Judge A y Judge B, hallazgos unificados, y si aprueba persiste el receipt en el store CAS de `gates.NewCASReceiptStore(projectRoot)`.

---

### INCOMPLETE-2: `autoCreateSkill` cableado al learning loop

**Archivos**: `internal/agent/spawner.go`, `cmd/gaia/main.go`

Cableado mediante el callback `SetOnLearningThreshold` en `Spawner`. Cuando el subagente alcanza el threshold de ejecuciones en el `LearningLoop` (`RecordExecution`), se dispara `autoCreateSkill` creando el archivo `SKILL.md` con los patrones aprendidos. Probado con test unitario `TestSpawner_OnLearningThreshold`.

---

### INCOMPLETE-3: Limpieza de shutdown cosmético de Engram

**Archivo**: `cmd/gaia/main.go:461-476`

Se removió el helper cosmético `engramHelper` y el prefijo ficticio `ENGRAM_SESSION_CLOSE_SUMMARY`. El shutdown de GAIA ahora persiste el resumen de sesión (`SESSION_CLOSE_SUMMARY`) de forma nativa en el SQLite local (`repo`/`mneme`), alineado con la arquitectura zero-dependency de Mnemosyne sin dependencias externas.

---

## 🟡 Deuda técnica — atacar gradualmente

### DEBT-1: `kernel.go` God Object (modularizado de ~4.028 a 1.821 líneas)

**Archivo**: `internal/core/kernel.go`

Se redujo más del 50% de su tamaño extrayendo responsabilidades temáticas de `*Brain` en el mismo paquete `package core`:
1. `session_commands.go`: `/undo`, `/retry`, `/new`, `/save`, `/sessions`, `/title`, `/resume`, `/compress`, `/history`.
2. `workflow_commands.go`: `/goal`, `/subgoal`, `/snapshot`, `/branch`, `/queue`, `/steer`, `/background`.
3. `config_commands.go`: `/model`, `/fast`, `/busy`, `/codegraph`, `/memory` (pending/approve/reject).
4. `info_commands.go`: `/insights`, `/debug`, `/credits`, `/billing`, `/commands`, `/restart`, `/update`, `/topic`, `/help`, `/diff`, `/audit`.
5. `skill_commands.go`: `/learn`, `/suggestions`, `/blueprint`, `/curator`, skill proposal & learning triggers.
6. `media_commands.go`: `/image`, `/paste`.
7. `integration_commands.go`: `/sethome`, `/trust` approvals, `/session`, `/kg` (status/stats/clear/extract), `/mcp`.

El núcleo de `kernel.go` queda concentrado en el bucle cognitivo: streaming de respuestas, ejecución de herramientas (`handleToolCalls`), delegación y disparadores SDD/ODD.

---

### DEBT-2: Cancelación y propagación de Context en producción

**Archivos afectados resueltos**:
- `cmd/gaia/main.go`: Creado `rootCtx` mediante `signal.NotifyContext(..., SIGINT, SIGTERM)`. Se propaga a `ui`, `wizard`, `repo.CreateSession`, `dynamicLoader` y gateways unificados.
- `internal/adapters/tui/tui.go`: Añadido `SetContext(ctx)` y `getContext()`. Se reemplazó `context.Background()` en `ProcessMessage`, `Steer` y callbacks asíncronos para respetar la cancelación de la TUI/señales del SO.
- `internal/adapters/tui/wizard.go`: Añadido `SetContext(ctx)` y propagado a las llamadas de autenticación de GitHub Copilot y listado de modelos.
- `internal/cron/cron.go`: El loop de ejecución `runJob` ahora propaga su `ctx` activo a `executeJob` y `repo.MarkRun` en lugar de crear `context.Background()`.
- `cmd/gaia/review.go`: El comando CLI de Judgment Day ahora usa `signal.NotifyContext`.

---

### DEBT-3: Errores silenciados en producción

**Estado**: Solucionado.
- `internal/adapters/tui/tui.go`: Se dejó de silenciar `ProcessMessage` en steering devolviendo `processDoneMsg{err: err}`. Se implementó la visualización de errores (`❌ Error: ...`) en el historial del chat ante fallos de `ProcessMessage`.
- `internal/agent/ops/reviewer.go`: Se reemplazó el silencio de `saveErr` por `slog.Warn("Failed to save review receipt to filesystem", "change", tx.ChangeName, "error", saveErr)`.
- `internal/adapters/web/server.go`: Se agregó control de error y logging con `slog.Error` al invocar `views.RenderLayout(data).Render(r.Context(), w)` en `handleDashboard` y `handleSendMessage`, respondiendo con HTTP 500 en caso de fallo.

---

### DEBT-4: `GaiaSDK.Find()` ignora error de `filepath.Walk`

**Estado**: Solucionado.
- **Archivo**: `internal/modules/codemode/sdk.go`
- Se captura el error de `filepath.Walk`. En la función de recorrido, si ocurre un error en la raíz (`p == s.projectRoot`), se propaga para abortar el paseo inmediatamente. Si el recorrido retorna error, se dispara un `panic(fmt.Sprintf("Find failed for pattern %q: %v", pattern, err))` que es capturado de manera segura por el sandbox del `Engine` (consistente con el contrato "Must" de `ReadFile`, `WriteFile`, `Grep` y `Exec` en `GaiaSDK`).
- Se agregó el test unitario `TestGaiaSDK_FindError` en `internal/modules/codemode/engine_test.go` verificando que un fallo en el recorrido sea capturado como error por el sandbox.

---

### DEBT-5: Dual-path de configuración de browser (refactor incompleto)

**Estado**: Solucionado.
- **Archivo**: `cmd/gaia/main.go:391-398`
- Se removió la bifurcación legacy `else if cfg.Browser.Enabled && cfg.Browser.ExecutablePath != ""`, consolidando la inicialización del módulo del browser exclusivamente a través de la ruta canónica `cfg.Gateway.BrowserTools`.

---

### DEBT-6: Validación O(n²) en DynamicLoader

**Estado**: Solucionado.
- **Archivo**: `cmd/gaia/main.go:410-424`
- Se reemplazó la búsqueda lineal anidada sobre `availableTools` (`[]string`) construyendo un conjunto hash `availableToolSet` (`map[string]struct{}`) una única vez.
- Cada validación de herramienta en `allowed` pasó de $O(n \times m)$ a búsqueda en tiempo constante $O(1)$, reduciendo la complejidad total de la verificación a tiempo lineal $O(n + m)$.

---

## 📊 Tabla de prioridades

| ID | Descripción | Tipo | Prioridad |
|---|---|---|---|
| BUG-1 | Tamper detection en backup/restore | Bug / Seguridad | ✅ Resuelto |
| BUG-2 | SetReasoningEffort / SetPersona son stubs | Feature incompleta | ✅ Resuelto |
| INCOMPLETE-1 | Judgment Day CLI cableado | Feature | ✅ Resuelto |
| INCOMPLETE-2 | `autoCreateSkill` nunca se llama | Dead code / Feature | ✅ Resuelto |
| INCOMPLETE-3 | Engram session close cosmético | Limpieza arquitectura | ✅ Resuelto |
| DEBT-1 | kernel.go God Object (~4.028 → 1.821 líneas) | Arquitectura | ✅ Modularizado |
| DEBT-2 | context.Background() en producción (26x) | Robustez | ✅ Resuelto |
| DEBT-3 | Errores silenciados en producción | Observabilidad | ✅ Resuelto |
| DEBT-4 | GaiaSDK.Find() ignora error de Walk | Bug menor | ✅ Resuelto |
| DEBT-5 | Dual-path config de browser | Refactor incompleto | ✅ Resuelto |
| DEBT-6 | Validación O(n²) en DynamicLoader | Performance menor | ✅ Resuelto |

---

## 🔭 Observación de fondo

El proyecto tiene una tensión entre el SPEC original ("programming-exclusive autonomous agent") y el scope real (gateways de voz, WhatsApp, Web UI, Wails desktop, audio TTS). El `kernel.go` acumula toda esa complejidad sin subdivisión.

**La regla de oro para las próximas features**: antes de agregar algo nuevo al kernel, mover al menos una responsabilidad existente a su propio archivo. Divide y vencerás de forma incremental.
