package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"gaia/internal/overnight"
)

func handleOvernightCLI(args []string) {
	if len(args) == 0 {
		printOvernightHelp()
		return
	}

	switch args[0] {
	case "start", "run":
		if len(args) < 2 {
			fmt.Println("Uso: gaia overnight start \"<objetivo o tarea>\" [--profile=<perfil>]")
			os.Exit(1)
		}
		goal := args[1]
		profile := "dev"
		for _, a := range args[2:] {
			if strings.HasPrefix(a, "--profile=") {
				profile = strings.TrimPrefix(a, "--profile=")
			}
		}
		startOvernightPlanCLI(goal, profile)

	case "list", "ls":
		listOvernightPlansCLI()

	case "review":
		if len(args) < 2 {
			fmt.Println("Uso: gaia overnight review <plan_id>")
			os.Exit(1)
		}
		reviewOvernightPlanCLI(args[1])

	case "merge", "apply":
		if len(args) < 2 {
			fmt.Println("Uso: gaia overnight merge <plan_id>")
			os.Exit(1)
		}
		mergeOvernightPlanCLI(args[1])

	case "discard", "clean":
		if len(args) < 2 {
			fmt.Println("Uso: gaia overnight discard <plan_id>")
			os.Exit(1)
		}
		discardOvernightPlanCLI(args[1])

	case "help", "--help", "-h":
		printOvernightHelp()

	default:
		fmt.Printf("Comando overnight desconocido: %s\n\n", args[0])
		printOvernightHelp()
		os.Exit(1)
	}
}

func startOvernightPlanCLI(goal, profile string) {
	projectRoot, _ := os.Getwd()
	overnightDir := filepath.Join(projectRoot, ".gaia", "overnight")
	_ = os.MkdirAll(overnightDir, 0755)

	interviewer := overnight.NewPreflightInterviewer(nil, overnightDir)
	runner := overnight.NewRunner(nil, nil, overnightDir)

	fmt.Printf("=== GAIA Overnight Autonomous Alignment ===\n")
	fmt.Printf("Objetivo: %s\n", goal)
	fmt.Printf("Perfil:   %s\n\n", profile)

	ctx := context.Background()
	questions, err := interviewer.AnalyzeGoal(ctx, goal, profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al generar preguntas de pre-vuelo: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("--- Cuestionario Pre-Vuelo (Alineación antes de ejecutar) ---")
	reader := bufio.NewReader(os.Stdin)

	for i := range questions {
		q := &questions[i]
		fmt.Printf("\n[%d/%d] %s (Criticidad: %s)\n", i+1, len(questions), q.Question, q.Criticality)
		if len(q.Options) > 0 {
			for idx, opt := range q.Options {
				fmt.Printf("  %d) %s\n", idx+1, opt)
			}
		}
		fmt.Printf("Respuesta sugerida: [%s]\n> ", q.DefaultAnswer)

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			q.UserAnswer = q.DefaultAnswer
		} else {
			// Check if numeral choice
			var chosenIndex int
			if n, err := fmt.Sscanf(input, "%d", &chosenIndex); err == nil && n == 1 && chosenIndex >= 1 && chosenIndex <= len(q.Options) {
				q.UserAnswer = q.Options[chosenIndex-1]
			} else {
				q.UserAnswer = input
			}
		}
		fmt.Printf("✓ Asignado: %s\n", q.UserAnswer)
	}

	planID := fmt.Sprintf("plan-%d", time.Now().Unix())
	plan := &overnight.ActionPlan{
		ID:        planID,
		Goal:      goal,
		Profile:   profile,
		Questions: questions,
		Status:    overnight.PlanStatusPlanning,
		CreatedAt: time.Now(),
	}

	for _, q := range questions {
		plan.Directives = append(plan.Directives, fmt.Sprintf("%s: %s", q.ID, q.UserAnswer))
	}

	// Provision Sandbox
	if err := runner.ProvisionSandbox(ctx, plan, projectRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error al crear sandbox: %v\n", err)
		os.Exit(1)
	}

	if err := interviewer.SavePlan(plan); err != nil {
		fmt.Fprintf(os.Stderr, "Error al guardar plan: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✓ Plan %s sellado con éxito.\n", plan.ID)
	fmt.Printf("  Sandbox: %s (%s)\n", plan.SandboxPath, plan.SandboxMode)
	if plan.BranchName != "" {
		fmt.Printf("  Branch:  %s\n", plan.BranchName)
	}
	fmt.Println("\nEl agente está listo para iniciar la ejecución autónoma desatendida.")
	fmt.Println("A la mañana siguiente ejecutá:")
	fmt.Printf("  gaia overnight review %s\n", plan.ID)
}

func listOvernightPlansCLI() {
	projectRoot, _ := os.Getwd()
	overnightDir := filepath.Join(projectRoot, ".gaia", "overnight")

	files, err := filepath.Glob(filepath.Join(overnightDir, "plan-*.json"))
	if err != nil || len(files) == 0 {
		fmt.Println("No hay planes autónomos registrados.")
		return
	}

	interviewer := overnight.NewPreflightInterviewer(nil, overnightDir)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PLAN ID\tPROFILE\tSTATUS\tSANDBOX\tGOAL")

	for _, f := range files {
		if strings.HasSuffix(f, "-review.json") {
			continue
		}
		base := filepath.Base(f)
		planID := strings.TrimSuffix(base, ".json")
		plan, err := interviewer.LoadPlan(planID)
		if err != nil {
			continue
		}
		goal := plan.Goal
		if len(goal) > 40 {
			goal = goal[:37] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			plan.ID, plan.Profile, plan.Status, plan.SandboxMode, goal)
	}
	w.Flush()
}

func reviewOvernightPlanCLI(planID string) {
	projectRoot, _ := os.Getwd()
	overnightDir := filepath.Join(projectRoot, ".gaia", "overnight")
	interviewer := overnight.NewPreflightInterviewer(nil, overnightDir)
	runner := overnight.NewRunner(nil, nil, overnightDir)

	plan, err := interviewer.LoadPlan(planID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al cargar plan %s: %v\n", planID, err)
		os.Exit(1)
	}

	ctx := context.Background()
	pack, err := runner.BuildReviewPack(ctx, plan)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al compilar review pack: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("=== Consolidated Review Pack: %s ===\n", plan.ID)
	fmt.Printf("Objetivo: %s\n", plan.Goal)
	fmt.Printf("Estado:   %s\n", plan.Status)
	fmt.Printf("Sandbox:  %s (%s)\n", plan.SandboxPath, plan.SandboxMode)
	if plan.BranchName != "" {
		fmt.Printf("Rama Git: %s\n", plan.BranchName)
	}

	if len(pack.DeferredActions) > 0 {
		fmt.Printf("\n[Acciones diferidas pendientes de revisión: %d]\n", len(pack.DeferredActions))
		for _, a := range pack.DeferredActions {
			fmt.Printf("  - Tool: %s (Razón: %s, Estado: %s)\n", a.ToolName, a.Reason, a.Status)
		}
	}

	if pack.TestResults != "" {
		fmt.Println("\n--- Resultados de Tests Automatizados ---")
		fmt.Println(pack.TestResults)
	}

	if pack.Diff != "" {
		fmt.Println("\n--- Git Unified Diff ---")
		diffLines := strings.Split(pack.Diff, "\n")
		limit := 40
		if len(diffLines) < limit {
			limit = len(diffLines)
		}
		for i := 0; i < limit; i++ {
			fmt.Println(diffLines[i])
		}
		if len(diffLines) > limit {
			fmt.Printf("... y %d líneas más.\n", len(diffLines)-limit)
		}
	} else if len(pack.GeneratedFiles) > 0 {
		fmt.Println("\n--- Archivos Generados en Staging ---")
		for _, f := range pack.GeneratedFiles {
			fmt.Printf("  + %s\n", f)
		}
	}

	fmt.Println("\nComandos:")
	fmt.Printf("  gaia overnight merge %s    - Aprobar e incorporar cambios al workspace principal\n", plan.ID)
	fmt.Printf("  gaia overnight discard %s  - Descartar y limpiar sandbox\n", plan.ID)
}

func mergeOvernightPlanCLI(planID string) {
	projectRoot, _ := os.Getwd()
	overnightDir := filepath.Join(projectRoot, ".gaia", "overnight")
	interviewer := overnight.NewPreflightInterviewer(nil, overnightDir)
	runner := overnight.NewRunner(nil, nil, overnightDir)

	plan, err := interviewer.LoadPlan(planID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al cargar plan %s: %v\n", planID, err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := runner.Merge(ctx, plan, projectRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error al hacer merge del plan: %v\n", err)
		os.Exit(1)
	}

	_ = interviewer.SavePlan(plan)
	fmt.Printf("✓ Plan %s incorporado exitosamente al workspace principal. Sandbox destruido.\n", plan.ID)
}

func discardOvernightPlanCLI(planID string) {
	projectRoot, _ := os.Getwd()
	overnightDir := filepath.Join(projectRoot, ".gaia", "overnight")
	interviewer := overnight.NewPreflightInterviewer(nil, overnightDir)
	runner := overnight.NewRunner(nil, nil, overnightDir)

	plan, err := interviewer.LoadPlan(planID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al cargar plan %s: %v\n", planID, err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := runner.Discard(ctx, plan, projectRoot); err != nil {
		fmt.Fprintf(os.Stderr, "Error al descartar plan: %v\n", err)
		os.Exit(1)
	}

	_ = interviewer.SavePlan(plan)
	fmt.Printf("✓ Plan %s descartado y sandbox limpiado exitosamente.\n", plan.ID)
}

func printOvernightHelp() {
	fmt.Println("Uso: gaia overnight <subcomando> [opciones]")
	fmt.Println()
	fmt.Println("Subcomandos:")
	fmt.Println("  start \"<tarea>\" [--profile=<id>] Realiza el cuestionario pre-vuelo y crea el sandbox aislado")
	fmt.Println("  list                            Lista todos los planes autónomos")
	fmt.Println("  review <plan_id>                Muestra el paquete consolidado (diff, tests y acciones diferidas)")
	fmt.Println("  merge <plan_id>                 Aplica los cambios del sandbox al proyecto principal")
	fmt.Println("  discard <plan_id>               Descarta los cambios y elimina el sandbox")
	fmt.Println("  help                            Muestra esta ayuda")
}
