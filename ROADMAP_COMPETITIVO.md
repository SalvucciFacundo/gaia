# GAIA Strategic Evolution & Competitive Roadmap

Este documento recopila las 4 mejoras arquitectónicas y de experiencia de usuario prioritarias para posicionar a GAIA al nivel de los agentes de ingeniería de software líderes del mercado (Claude Code, Cursor Composer, Aider).

---

## 🚀 Pilares Estratégicos Prioritarios

| # | Mejora | Tipo | Impacto | Estado |
|---|---|---|---|---|
| **1** | **Edición Quirúrgica: Fuzzy Search & Replace (`apply_patch`)** | Core / Tokens | 🔴 Crítico (-70% tokens, cero truncamiento) | ✅ Implementado |
| **2** | **Feedback Loop Cerrado: Auto-Reparación con Linter/Compilador** | Calidad / Resiliencia | 🔴 Crítico (cero código roto entregado) | 📋 Planificado |
| **3** | **Micro-Compaction de Contexto con Resumen Ejecutivo** | Memoria / Sesiones largas | 🟡 Alto (sobrevive a refactors gigantes) | 📋 Planificado |
| **4** | **DX de Terminal: Autocompletado `Tab`, `@archivo` y Aprobación Inline** | UX / Terminal | 🟡 Alto (usabilidad diaria) | 📋 Planificado |

---

## Detalle Técnico de las Mejoras

### 1. Edición Quirúrgica: Fuzzy Search & Replace (`apply_patch`)
- **Problema actual**: El agente recurre a reescribir archivos enteros o a invocar código dinámico. En archivos medianos o grandes (>300 líneas), reescribir todo es lento, gasta miles de tokens innecesarios y corre el riesgo de alucinar o borrar funciones preexistentes.
- **Solución propuesta**:
  - Implementar una herramienta nativa de parche quirúrgico (`apply_patch` / `edit_block`).
  - El modelo solo emite el bloque exacto que desea buscar y el bloque con el reemplazo.
  - Soporte de **Fuzzy Matching**: tolera pequeñas diferencias en espacios en blanco, indentación (tabs vs espacios) o saltos de línea (CRLF vs LF) sin fallar.
- **Beneficio**: Ahorro de hasta un **70%–80% de tokens** por turno de edición y latencia drásticamente menor.

---

### 2. Feedback Loop Cerrado: Auto-Reparación con Linter y Compilador (✅ Implementado)
- **Estado**: ✅ Implementado y validado en `internal/core/autofix` y `internal/core/kernel.go`.
- **Problema resuelto**: El agente ya no devuelve el control con código roto; detecta mutaciones de archivos (`file_write`, `file_patch`, `apply_patch`), ejecuta el chequeo en background del lenguaje detectado (Go, Rust, TypeScript, Python) e inyecta los errores de compilación directamente al LLM para que se auto-repare antes de concluir.
- **Beneficio**: El usuario solo recibe el turno cuando el código compila y pasa validaciones básicas.

---

### 3. Micro-Compaction de Contexto y Resumen Ejecutivo
- **Problema actual**: En sesiones largas, el contexto crece rápidamente con outputs extensos de herramientas, llenando la ventana de contexto y diluyendo las instrucciones iniciales.
- **Solución propuesta**:
  - Potenciar `brain_compaction.go` con un algoritmo de **micro-compactación incremental**:
    - Compacta turnos antiguos en un resumen estructurado (objetivo original, decisiones clave tomadas, archivos editados, estado actual).
    - Preserva intactas las instrucciones del sistema (`SOUL.md`), las restricciones del usuario y los snapshots de archivos más recientes.
- **Beneficio**: Sesiones de refactorización complejas sin pérdida de memoria ni alucinaciones por contexto saturado.

---

### 4. DX de Terminal: Autocompletado `Tab`, Menciones `@archivo` y Diffs Inline
- **Problema actual**: En la TUI, escribir rutas largas es manual y propenso a errores, y la revisión de diffs requiere saltar a `/diff`.
- **Solución propuesta**:
  - **Autocompletado con `Tab`** en el `textinput` de Bubbletea para archivos y comandos.
  - **Operador `@archivo` / `@simbolo`**: Permite escribir *"arreglá @internal/core/kernel.go"* y GAIA inyecta automáticamente el archivo relevante en el contexto sin que el modelo tenga que gastar un turno en buscarlo.
  - **Revisión interactiva inline**: Mostrar los bloques modificados con resaltado de color (`+` / `-`) en la TUI y permitir confirmar (`y` / `n`) antes de persistir a disco.
- **Beneficio**: Experiencia de desarrollo moderna, fluida y ágil idéntica a Claude Code o Cursor.
