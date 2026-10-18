package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"gaia/internal/adapters/db"
	"gaia/internal/blackboard"
)

func handleBlackboardCLI(args []string) {
	if len(args) == 0 {
		printBlackboardHelp()
		return
	}

	switch args[0] {
	case "snapshot", "snapshots":
		handleBlackboardSnapshotCLI(args[1:])
	case "help", "--help", "-h":
		printBlackboardHelp()
	default:
		fmt.Printf("Comando de blackboard desconocido: %s\n\n", args[0])
		printBlackboardHelp()
		os.Exit(1)
	}
}

func handleBlackboardSnapshotCLI(args []string) {
	if len(args) == 0 {
		listBlackboardSnapshotsCLI()
		return
	}

	switch args[0] {
	case "list", "ls":
		listBlackboardSnapshotsCLI()

	case "inspect", "get", "show":
		if len(args) < 2 {
			fmt.Println("Uso: gaia blackboard snapshot inspect <id>")
			os.Exit(1)
		}
		inspectBlackboardSnapshotCLI(args[1])

	case "restore":
		if len(args) < 2 {
			fmt.Println("Uso: gaia blackboard snapshot restore <id>")
			os.Exit(1)
		}
		restoreBlackboardSnapshotCLI(args[1])

	default:
		fmt.Printf("Subcomando de snapshot desconocido: %s\n\n", args[0])
		printBlackboardHelp()
		os.Exit(1)
	}
}

func listBlackboardSnapshotsCLI() {
	repo, err := db.NewSQLiteRepo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al abrir base de datos: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	list, err := repo.ListBlackboardSnapshots(ctx, "", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al listar snapshots: %v\n", err)
		os.Exit(1)
	}

	if len(list) == 0 {
		fmt.Println("No hay snapshots registrados en el Blackboard.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tPROFILE\tLABEL\tENTRIES\tSHA-256 (PREFIX)\tDATE")
	for _, s := range list {
		prefix := s.SHA256
		if len(prefix) > 12 {
			prefix = prefix[:12] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
			s.ID, s.Profile, s.Label, s.EntryCount, prefix, s.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	w.Flush()
}

func inspectBlackboardSnapshotCLI(id string) {
	repo, err := db.NewSQLiteRepo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al abrir base de datos: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	entries, meta, err := repo.GetBlackboardSnapshot(ctx, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al recuperar snapshot (verificación de integridad): %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Snapshot ID:       %s\n", meta.ID)
	fmt.Printf("Profile:           %s\n", meta.Profile)
	fmt.Printf("Session ID:        %s\n", meta.SessionID)
	fmt.Printf("Label:             %s\n", meta.Label)
	fmt.Printf("SHA-256 Checksum:  %s [VERIFIED OK]\n", meta.SHA256)
	fmt.Printf("Entries Count:     %d\n", meta.EntryCount)
	fmt.Printf("Created At:        %s\n", meta.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println("\nEntries:")

	for k, v := range entries {
		payload, _ := blackboard.ReadPayload(v)
		valStr := string(payload)
		if len(valStr) > 80 {
			valStr = valStr[:77] + "..."
		}
		spillInfo := ""
		if v.IsSpilled {
			spillInfo = fmt.Sprintf(" (Spilled to %s)", v.SpillPath)
		}
		fmt.Printf("  - %s: %s%s\n", k, valStr, spillInfo)
	}
}

func restoreBlackboardSnapshotCLI(id string) {
	repo, err := db.NewSQLiteRepo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al abrir base de datos: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	entries, meta, err := repo.GetBlackboardSnapshot(ctx, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error de seguridad/integridad al validar snapshot: %v\n", err)
		os.Exit(1)
	}

	projectRoot, _ := os.Getwd()
	bbDir := filepath.Join(projectRoot, ".gaia", "blackboard")
	bbInst := blackboard.New(bbDir)
	bbInst.Restore(entries)

	fmt.Printf("✓ Snapshot %s restaurado exitosamente en working memory (%d entries, SHA-256 verificado).\n", meta.ID, meta.EntryCount)
}

func printBlackboardHelp() {
	fmt.Println("Uso: gaia blackboard <subcomando> [opciones]")
	fmt.Println()
	fmt.Println("Subcomandos:")
	fmt.Println("  snapshot list              Lista todos los checkpoints guardados en SQLite con sus hashes SHA-256")
	fmt.Println("  snapshot inspect <id>      Inspecciona un checkpoint y valida criptográficamente su integridad")
	fmt.Println("  snapshot restore <id>      Restaura un checkpoint verificado al working memory del Blackboard")
	fmt.Println("  help                       Muestra esta ayuda")
}
