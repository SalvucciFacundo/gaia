# GAIA — Informe Integral de Auditoría, Estado y Roadmap Estratégico

> **Fecha de Actualización**: Octubre 2026  
> **Proyecto**: GAIA (Go Autonomous Intelligence Agent)  
> **Ubicación**: `/run/media/kuno/Disco local/Kuno/GO/Agent`  
> **Alcance de la Auditoría**: Análisis estático (`go vet`), ejecución de tests en 60+ paquetes, revisión línea por línea del Kernel (`kernel.go`), auditoría de seguridad/concurrencia de adaptadores e infraestructura, y benchmark competitivo contra 8 agentes líderes del mercado (Claude Code, Cursor, Aider, Cline, Hermes Agent, Goose, Amazon Q CLI y OpenHands).

---

## 📑 Índice de Contenidos
1. [Resumen Ejecutivo](#1-resumen-ejecutivo)
2. [Lo que Ya Se Arregló (Fase 1 Completada)](#2-lo-que-ya-se-arregló-fase-1-completada)
3. [Lo que Falta por Arreglar (Bugs, Seguridad y Arquitectura)](#3-lo-que-falta-por-arreglar-bugs-seguridad-y-arquitectura)
   - [3.1 Vulnerabilidades Críticas de Seguridad](#31-vulnerabilidades-críticas-de-seguridad)
   - [3.2 Bloqueos Fatales y Concurrencia](#32-bloqueos-fatales-y-concurrencia)
   - [3.3 Fallas de Memoria y Lógica del Agente](#33-fallas-de-memoria-y-lógica-del-agente)
   - [3.4 Deuda Técnica y Estabilidad](#34-deuda-técnica-y-estabilidad)
4. [Lo que Se Puede Implementar para Mejorar (Benchmark Competitivo)](#4-lo-que-se-puede-implementar-para-mejorar-benchmark-competitivo)
5. [Orden de Implementación Recomendado](#5-orden-de-implementación-recomendado)

---

## 1. Resumen Ejecutivo

GAIA cuenta con una arquitectura de primer nivel basada en **Hexagonal/Clean Architecture**, **12+ subagentes especializados (SDD/ODD)**, **memoria híbrida Mnemosyne (FTS5 + Vector + RRF)**, **soporte multi-gateway** y **parcheo quirúrgico (`apply_patch`)**.

Sin embargo, el diagnóstico inicial que marcaba todos los problemas como resueltos no detectó **fallas profundas de concurrencia, inyecciones SQL y escapes de sandbox** en las capas de integración. La auditoría exhaustiva reveló estos puntos ciegos, permitiendo establecer un plan de blindaje definitivo antes de agregar nuevas capacidades de cara a competir contra Claude Code, Cursor y Aider.

---

## 2. Lo que Ya Se Arregló (Fase 1 Completada)

Todos los ítems de la Fase 1 fueron implementados, verificados y cubiertos por tests unitarios:

| ID | Componente | Problema Original | Solución Aplicada | Estado |
|---|---|---|---|---|
| **BUG-R1** | `internal/backup/backup_test.go` | El test `TestBackup_Restore_ChecksumMismatchRollback` era *flaky* y fallaba intermitentemente. Corrompía un byte a `len-50`, cayendo dentro del padding nulo de 512 bytes del archivo tar donde gzip no disparaba error de integridad. | Se ajustó la alteración hacia la mitad del stream (`len/2`), garantizando impacto en los bloques de encabezados y datos activos. Verificado con 30 corridas consecutivas sin caché. | ✅ **Resuelto** |
| **BUG-R2** | `internal/tools/catalog.go` | La tool `run_tests` era un **stub ficticio** que siempre devolvía `"Tests passed for package: ..."` con `Success: true` sin ejecutar nada en la máquina. El LLM creía ciegamente que los tests pasaban. | Se reemplazó por un ejecutor real multi-lenguaje que detecta la raíz del proyecto (`go.mod`, `Cargo.toml`, `package.json`, `pyproject.toml`) y despacha las pruebas reales (`go test`, `cargo test`, `npm test`, `pytest`). Si fallan, devuelve `Success: false` con la salida real de error. | ✅ **Resuelto** |
| **ARCH-3** | `internal/core/kernel.go` | Al ejecutar `/direct <mensaje>`, `handleSDDTrigger` llamaba a `processDirect`, el cual guardaba el mensaje en la base de datos pero **retornaba de inmediato sin invocar al LLM**, tragándose el prompt del usuario. | Se corrigió para que `/direct` e `/inline` omitan el pipeline SDD pero sigan su curso llamando a `ProcessMessage` con el contenido limpio, ejecutando el ciclo cognitivo del modelo normalmente. | ✅ **Resuelto** |
| **ARCH-4** | `internal/core/kernel.go` | `buildAsyncSDDPipelineSummary` asumía ciegamente que `len(results) == len(domain.SDDPhases(""))`. Si una fase fallaba o se cancelaba, provocaba un `panic: runtime error: index out of range`. | Se agregaron límites seguros (`i < len(phases)`), nombres de fase por defecto (`Phase N`) y protección contra punteros nulos (`r != nil`) en la recolección de artefactos y riesgos. | ✅ **Resuelto** |

---

## 3. Lo que Falta por Arreglar (Bugs, Seguridad y Arquitectura)

### 3.1 Vulnerabilidades Críticas de Seguridad

* [x] **Inyección SQL en `SQLiteRepo.sessionWhere()` (`internal/adapters/db/sqlite.go:27-33`):**  
  * **Problema**: Concatena `r.sessionID` directamente en la query SQL usando `fmt.Sprintf("session_id = '%s'", id)`.
  * **Impacto**: Como el `sessionID` puede originarse desde plataformas externas (Telegram chat ID, Discord thread ID) o comandos `/resume <id>`, un atacante puede inyectar SQL (`' OR '1'='1`) para leer o borrar conversaciones de otros usuarios.
  * **Solución**: Se parametrizaron todas las consultas con `WHERE session_id = ?`. Verificado con `sqlite_sqli_test.go`. ✅ **Resuelto**

* [x] **Escape de Sandbox en Yaegi CodeMode (`internal/modules/codemode`):**  
  * **Problema**: `GaiaSDK.ReadFile` y `WriteFile` no validaban rutas con `filepath.Rel`, permitiendo leer y escribir fuera del proyecto mediante path traversal (`../../`).
  * **Impacto**: Cualquier script ejecutado en CodeMode podía escapar del sandbox y acceder a archivos sensibles del sistema operativo.
  * **Solución**: Enjaulado estricto con `filepath.Rel(projectRoot, target)` en `GaiaSDK.resolvePath`. Cubierto con tests unitarios en `engine_test.go`. ✅ **Resuelto**

* [x] **Path Traversal en Comandos de Exportación (`internal/core/workflow_commands.go:76, 101` y `export.go:41`):**  
  * **Problema**: Parámetros de usuario en `/snapshot` y `/memory export` se unían directamente con `filepath.Join(dir, name + ".json")`.
  * **Impacto**: Un nombre como `../../../../tmp/malicious` permitía escribir archivos fuera del directorio permitido.
  * **Solución**: Sanitización mediante `filepath.Base(filepath.Clean(name))` y verificación de nombres reservados (`.`, `..`, `/`). ✅ **Resuelto**

* [x] **Web UI sin Autenticación en `0.0.0.0` y sin Protección CSRF (`internal/adapters/web/server.go:82`):**  
  * **Problema**: El servidor HTTP se enlazaba por defecto a todas las interfaces de red (`0.0.0.0:8080`) sin protección CSRF en endpoints de mutación (`/web/message`, `/web/policy/update`).
  * **Impacto**: Ataques de CSRF desde el navegador del usuario o accesos no autorizados desde la red local podían alterar políticas de seguridad o despachar comandos al Brain.
  * **Solución**: Bind por defecto asegurado en `127.0.0.1` (con opción `WithHost`), y middleware `csrfProtectionMiddleware` que intercepta métodos mutantes (POST/PUT/DELETE/PATCH) y bloquea peticiones de orígenes externos no confiables (`StatusForbidden 403`). Verificado con `TestWeb_CSRFProtection`. ✅ **Resuelto**

---

### 3.2 Bloqueos Fatales y Concurrencia

* [x] **Deadlock Fatal en `SessionManager.Route` (`internal/core/session.go:111` e `integration_commands.go:62`):**  
  * **Problema**: `SessionManager.Route` adquiere `sm.mu.Lock()` y llama sincrónicamente a `b.ProcessMessage()`. Si el mensaje es un subcomando como `/session unify` o `/session isolate`, se llama a `sm.SetMode()`, el cual intenta adquirir `sm.mu.Lock()` en la misma goroutine.
  * **Impacto**: Como `sync.RWMutex` en Go no es reentrante, la aplicación entra en deadlock instantáneo.
  * **Solución**: Se liberó el lock (`sm.mu.Unlock()`) antes de despachar a `b.ProcessMessage()`. ✅ **Resuelto**

* [x] **Data Races en el Struct `Brain` (`internal/core/kernel.go:40-78`):**  
  * **Problema**: `currentSessionID`, `pendingImages` y `onToken` eran leídos y modificados concurrentemente por gateways y pipelines.
  * **Impacto**: Race conditions al atender mensajes simultáneos desde diferentes canales.
  * **Solución**: Se introdujo `stateMu sync.RWMutex` en `Brain` para proteger `currentSessionID`, `pendingImages` y `onToken`. ✅ **Resuelto**

* [x] **Bloqueo en `CredentialPool` (`internal/adapters/llm/pool.go:85-99`):**  
  * **Problema**: Verificación de bloqueo de mutex durante peticiones.
  * **Impacto**: Serialización innecesaria de peticiones.
  * **Solución**: Se validó que `acquire()` libera el mutex inmediatamente retornando un callback `release()` que se ejecuta antes de cualquier operación de red. ✅ **Verificado / Resuelto**

* [x] **Bucle Infinito y Panic OOM en Cron Step Parser (`internal/cron/scheduler.go:104-114`):**  
  * **Problema**: Al evaluar expresiones cron de paso `*/N`, no se validaba que `step > 0`.
  * **Impacto**: Si un usuario ingresaba `*/0`, el bucle `for i := min; i <= max; i += step` se congelaba consumiendo 100% de CPU.
  * **Solución**: Validación estricta `if step <= 0 { return nil, fmt.Errorf("step must be greater than 0: got %d", step) }`. Verificado con tests unitarios en `cron_test.go`. ✅ **Resuelto**

---

### 3.3 Fallas de Memoria y Lógica del Agente

* [x] **Amnesia de Contexto tras Compactación (`internal/core/brain_compaction.go:246` y `kernel.go:324`):**  
  * **Problema**: `getHistory` buscaba el resumen de compactación con `GetHistoryFrom(ctx, limit, 0)` (offset 0 = mensajes más antiguos de la sesión).
  * **Impacto**: El resumen recién guardado al final de la sesión nunca aparecía.
  * **Solución**: Se actualizó a `GetLastMessages`, recuperando el resumen más reciente y preservando el contexto histórico post-compactación. ✅ **Resuelto**

* [x] **Orden Invertido de Mensajes de Herramientas (`internal/core/kernel.go:1450-1458`):**  
  * **Problema**: En `handleToolCalls`, los resultados de herramientas (`RoleTool`) se guardaban antes que el mensaje del asistente (`RoleAssistant`) con los `tool_calls`.
  * **Impacto**: APIs de OpenAI/Anthropic rechazaban el payload con HTTP 400 por secuencia de mensajes inválida.
  * **Solución**: Se persiste primero el mensaje del asistente con sus `tool_calls` y luego los mensajes de resultado de herramientas. ✅ **Resuelto**

* [x] **Recursión Infinita / Stack Overflow en `/goal` (`internal/core/workflow_commands.go:461-521`):**  
  * **Problema**: Al evaluar el objetivo persistente, si no se completaba, se invocaba recursivamente `b.ProcessMessage` sin límite de turnos.
  * **Impacto**: Consumo descontrolado de tokens o desbordamiento de pila.
  * **Solución**: Contador `goalTurns` en `Brain` con límite estricto de 25 turnos máximos automáticos antes de pausar y solicitar confirmación. ✅ **Resuelto**

* [x] **Implementaciones Fantasma en `/approve`, `/deny` y `/kg clear` (`internal/core/integration_commands.go:26-43`):**  
  * **Problema**: `/approve` y `/deny` solo imprimían un mensaje de texto formateado sin interactuar con `ConfirmGuard` ni `PolicyGuard`. `/kg clear` no llamaba a ningún método de borrado en `b.kgStore`.
  * **Impacto**: Daban una falsa sensación de control de seguridad y gestión de estado al usuario.
  * **Solución**: Se implementó `Clear()` en la interfaz `KnowledgeGraphStore` y en `SQLiteKnowledgeGraph` (limpiando tablas y virtual FTS5). Se conectó `/approve` y `/deny` al tracking `pendingConfirm` interactivo y elevación/restricción de `PolicyGuard` y `ConfirmGuard`. ✅ **Resuelto**

---

### 3.4 Deuda Técnica y Estabilidad

* [x] **`autofix` solo analiza el último archivo modificado (`internal/core/kernel.go:1463-1487`):**  
  Se agrupaban los archivos y se descartaban todos menos el último. Ahora se detectan y agrupan todas las raíces de proyecto únicas (`checkedRoots`) y se ejecuta el compilador/linter en cada una de ellas de forma deduplicada. ✅ **Resuelto**
* [x] **Concatenación de strings $O(n^2)$ en streaming (`internal/core/kernel.go:1256`):**  
  Usa `response.Content += chunk.Content` en un bucle cerrado. Migrado a `strings.Builder` con sincronización `stateMu` para `onToken`. ✅ **Resuelto**
* [x] **Comando `gaia gateway stop` inoperativo (`cmd/gaia/gateway.go:304-311`):**  
  Se implementó la persistencia atómica del PID en `$TMPDIR/gaia-gateway.pid` al arrancar el gateway, su limpieza diferida al apagarlo, y `handleGatewayStop` ahora lee el PID y envía la señal de terminación (`syscall.SIGTERM`). ✅ **Resuelto**
* [x] **No-determinismo en la caché de herramientas (`internal/core/registry.go:170-176`):**  
  Se ordenaron las claves del mapa lexicográficamente (`sort.Strings(keys)`) al construir la firma de caché en `buildCacheKey`, garantizando hits deterministas en invocaciones de solo lectura. ✅ **Resuelto**
* [x] **`CostTracker.Record()` nunca se invoca (`internal/core/cost.go:51`):**  
  Se conectó `b.costTracker.Record(b.modelName, history, response)` en cada ciclo de ejecución del LLM en `internal/core/kernel.go`, garantizando registro de tokens y costos reales para `/insights`. ✅ **Resuelto**
* [x] **Cascada monolítica de 55 `if`s de comandos en `kernel.go` (`internal/core/kernel.go:675-1015`):**  
  Se desacopló el enrutador de comandos a `command_dispatcher.go` mediante un `switch` idiomático limpio por comando exacto y prefijo, dejando `ProcessMessage` enfocado exclusivamente en el ciclo cognitivo del agente. ✅ **Resuelto**
* [x] **Timeout cableado de 30s en Mixture of Agents (`internal/core/kernel.go:254`):**  
  Se parametrizó con el campo `b.moaTimeout` y la opción `WithMoATimeout()`, con un valor base de 60s apto para modelos lentos de razonamiento. ✅ **Resuelto**

---

## 4. Lo que Se Puede Implementar para Mejorar (Benchmark Competitivo)

Tras comparar GAIA contra Claude Code, Cursor Composer, Aider, Cline, Hermes Agent, Goose, Amazon Q y OpenHands, se identificaron **8 características clave** para elevar a GAIA al estándar de los mejores asistentes autónomos de ingeniería:

### Nivel 1: Paridad Crítica (Funciones Esenciales)

1. **Shadow Git Checkpoints & `/undo` Interactivo en Disco:**
   * *Referencia*: Cline (Shadow Git) y Cursor (Timeline de Checkpoints).
   * *Qué gana GAIA*: Actualmente, GAIA solo tiene snapshots en memoria del Blackboard. Si un subagente modifica archivos y rompe el proyecto, el usuario no puede volver atrás el código sin perder el hilo de conversación. Con un árbol de Git sombra (`.gaia/shadow/`), cada tool de mutación (`file_write`, `apply_patch`, `bash`) genera un snapshot instantáneo. El usuario puede tipear `/undo` y restaurar los archivos exactamente al estado previo al turno.

2. **AST Repo Map Universal con Tree-Sitter + PageRank:**
   * *Referencia*: Aider (mapeo universal en 30+ lenguajes).
   * *Qué gana GAIA*: Actualmente `codegraph` utiliza `go/ast`, limitando la comprensión estructural profunda únicamente a proyectos en Go. Integrando Tree-Sitter (`go-tree-sitter`) se pueden extraer definiciones de clases, métodos, firmas y referencias en TypeScript, Python, Rust, C++ y Java, construyendo un mapa del repositorio de ~1.000 tokens que se inyecta en el prompt del Orchestrator.

3. **Bucle TDD Cerrado y Minimizador de Salida de Pruebas:**
   * *Referencia*: Aider (`--auto-test`), Claude Code y OpenHands.
   * *Qué gana GAIA*: Habiendo implementado el ejecutor real en `run_tests`, el siguiente paso es conectar un bucle TDD autónomo: tras aplicar un cambio y verificar que compila, se corren los tests automáticamente. Si fallan, un **minimizador de logs** extrae únicamente los nombres de los tests fallidos y el stacktrace relevante (~200 tokens en vez de 3.000 líneas de logs) y retroalimenta al agente hasta que pasen en verde.

### Nivel 2: Automatización de Alto Impacto

4. **Automatización de Ciclo Git y Pull Requests (`/pr`):**
   * *Referencia*: Claude Code (`/pr`), OpenHands (Issue-to-PR).
   * *Qué gana GAIA*: Permitir que el agente cree una rama temática (`feat/sdd-auth`), genere commits atómicos con mensajes semánticos convencionales y abra automáticamente un Pull Request en GitHub/GitLab vía `gh` CLI o API REST con un resumen detallado y los comprobantes del subagente Verificador.

5. **Enrutamiento de Dos Niveles: Architect / Editor:**
   * *Referencia*: Aider (`--architect`) y Cursor.
   * *Qué gana GAIA*: Reducción del 50% al 70% en el consumo de tokens costosos. Un modelo de razonamiento avanzado (ej. Claude 3.7 Sonnet o o3-mini) diseña la estrategia de solución y los archivos a modificar, y un modelo rápido y económico (ej. `gpt-4o-mini`, `claude-3-5-haiku` o `deepseek-chat`) recibe el contexto y emite los bloques exactos de `apply_patch`.

6. **Reglas Dinámicas por Directorio / Globs (`.gaia/rules/*.md`):**
   * *Referencia*: Cursor (`.cursor/rules/*.mdc`).
   * *Qué gana GAIA*: Inyección de instrucciones contextuales según los archivos que el agente esté tocando (ej. reglas específicas para `internal/ports/**` o estilos para `web/**`), evitando sobrecargar el prompt del sistema con reglas innecesarias para la tarea actual.

### Nivel 3: Aislamiento y Telemetría

7. **Sandbox Enchufable en Docker / MicroVM:**
   * *Referencia*: OpenHands (Docker Sandbox) y Hermes Agent (7 backends).
   * *Qué gana GAIA*: Ejecución de comandos `bash` y scripts en contenedores efímeros aislados sin red ni acceso al sistema anfitrión, fundamental para tareas autónomas nocturnas (`/overnight`).

8. **HUD de Telemetría de Costos y Prompt Caching en Vivo:**
   * *Referencia*: Claude Code (`/cost`) y Cline.
   * *Qué gana GAIA*: Visualización en tiempo real en la barra de estado de Bubbletea de: gasto acumulado en dólares, porcentaje de ahorro por Prompt Caching (Anthropic/OpenAI) y uso del presupuesto de la ventana de contexto.

---

## 5. Orden de Implementación Recomendado

```
Phase 2A — Seguridad y Robustez de Datos (Inmediato)
├── 1. Inyección SQL: Parametrizar SQLiteRepo en todas sus consultas
├── 2. Deadlock SessionManager: Desbloquear mutex antes de ProcessMessage
├── 3. Escape de Sandbox Yaegi: Restringir símbolos y jailing de rutas
└── 4. Amnesia de Compactación: Corregir offset de lectura del resumen

Phase 2B — Concurrencia y Estabilidad de Flujo
├── 5. Concurrencia Brain: sync.RWMutex para tráfico de gateways
├── 6. Orden de Mensajes Tool: Guardar assistant antes de tool results
├── 7. Recursión /goal: Bucle iterativo con límite estricto de turnos
├── 8. CredentialPool: Liberar lock tras selección de credencial
└── 9. Cron Loop: Validar step > 0 en scheduler

Phase 3 — Features de Alto Valor Competitivo
├── 10. Shadow Git Checkpoints & /undo en TUI
├── 11. Bucle TDD automático con minimizador de errores
├── 12. AST Repo Map Universal con Tree-Sitter
└── 13. Automatización de Git & Pull Requests (/pr)

Phase 4 — Optimización y Aislamiento Avanzado
├── 14. Enrutamiento Architect / Editor de dos niveles
├── 15. Reglas por glob (.gaia/rules/*.md)
├── 16. Sandbox Docker para comandos shell
└── 17. Telemetría de costos y HUD en TUI
```
