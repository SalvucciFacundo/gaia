package main

import (
	"fmt"
	"os"
	"strings"

	"gaia/internal/config"
	"gaia/internal/profiles"
)

func handleProfileCLI(args []string) {
	if len(args) == 0 {
		printProfileList()
		return
	}

	switch args[0] {
	case "list", "ls":
		printProfileList()

	case "use", "switch", "set-default":
		if len(args) < 2 {
			fmt.Println("Uso: gaia profile use <id>")
			fmt.Println("Ejemplo: gaia profile use dev")
			os.Exit(1)
		}
		switchProfile(args[1])

	case "enable":
		if len(args) < 2 {
			fmt.Println("Uso: gaia profile enable <id>")
			os.Exit(1)
		}
		enableProfile(args[1])

	case "disable":
		if len(args) < 2 {
			fmt.Println("Uso: gaia profile disable <id>")
			os.Exit(1)
		}
		disableProfile(args[1])

	case "show", "current":
		showCurrentProfile()

	case "help", "--help", "-h":
		printProfileHelp()

	default:
		fmt.Printf("Comando de perfil desconocido: %s\n\n", args[0])
		printProfileHelp()
		os.Exit(1)
	}
}

func getProfileManager() (*profiles.Manager, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("error cargando configuración: %w", err)
	}
	mgr := profiles.NewManager(cfg, config.Save)
	mgr.RegisterDefaults()
	return mgr, nil
}

func printProfileList() {
	mgr, err := getProfileManager()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	list := mgr.List()
	fmt.Println("Perfiles disponibles en GAIA:")
	fmt.Println(strings.Repeat("-", 75))
	fmt.Printf("%-12s %-22s %-12s %-7s %s\n", "ID", "NOMBRE", "ESTADO", "TOOLS", "DESCRIPCIÓN")
	fmt.Println(strings.Repeat("-", 75))

	for _, p := range list {
		status := "desactivado"
		if p.Enabled {
			status = "habilitado"
		}
		if p.Active {
			status = "ACTIVO (*)"
		}

		fmt.Printf("%-12s %-22s %-12s %-7d %s\n",
			p.ID,
			p.Name,
			status,
			p.ToolCount,
			truncate(p.Description, 45),
		)
	}
	fmt.Println(strings.Repeat("-", 75))
	fmt.Println("Comandos útiles:")
	fmt.Println("  gaia profile use <id>     - Cambiar perfil activo permanentemente")
	fmt.Println("  gaia profile enable <id>  - Habilitar un perfil desactivado")
	fmt.Println("  gaia profile disable <id> - Deshabilitar un perfil")
	fmt.Println("  gaia profile show         - Ver detalles del perfil activo")
}

func switchProfile(id string) {
	mgr, err := getProfileManager()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.SetActive(id); err != nil {
		fmt.Printf("No se pudo activar el perfil '%s': %v\n", id, err)
		fmt.Printf("Tip: Si el perfil está desactivado, ejecutá primero: gaia profile enable %s\n", id)
		os.Exit(1)
	}

	active := mgr.Active()
	fmt.Printf("✓ Perfil activo actualizado a '%s' (%s)\n", active.ID(), active.Name())
	fmt.Println("Los cambios se guardaron de forma permanente en tu configuración.")
}

func enableProfile(id string) {
	mgr, err := getProfileManager()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.Enable(id); err != nil {
		fmt.Printf("Error habilitando perfil '%s': %v\n", id, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Perfil '%s' habilitado exitosamente.\n", id)
	fmt.Printf("Para usarlo como perfil activo: gaia profile use %s\n", id)
}

func disableProfile(id string) {
	mgr, err := getProfileManager()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.Disable(id); err != nil {
		fmt.Printf("Error deshabilitando perfil '%s': %v\n", id, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Perfil '%s' deshabilitado exitosamente.\n", id)
}

func showCurrentProfile() {
	mgr, err := getProfileManager()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	active := mgr.Active()
	if active == nil {
		fmt.Println("No hay ningún perfil activo configurado.")
		return
	}

	fmt.Printf("Perfil Activo: %s (%s)\n", active.ID(), active.Name())
	fmt.Printf("Descripción:   %s\n\n", active.Description())

	tools := active.Tools()
	fmt.Printf("Herramientas provistas (%d):\n", len(tools))
	if len(tools) == 0 {
		fmt.Println("  (Ninguna herramienta específica configurada)")
	} else {
		for _, t := range tools {
			fmt.Printf("  • %-20s - %s\n", t.Name, t.Description)
		}
	}

	fmt.Println("\nSystem Prompt Base:")
	lines := strings.Split(active.SystemPrompt(), "\n")
	for _, l := range lines {
		fmt.Printf("  | %s\n", l)
	}
}

func printProfileHelp() {
	fmt.Println("Gestión de Perfiles de GAIA:")
	fmt.Println("  gaia profile list           - Listar todos los perfiles disponibles")
	fmt.Println("  gaia profile use <id>       - Cambiar el perfil activo permanentemente")
	fmt.Println("  gaia profile enable <id>    - Habilitar un perfil desactivado")
	fmt.Println("  gaia profile disable <id>   - Deshabilitar un perfil")
	fmt.Println("  gaia profile show           - Mostrar detalles del perfil activo")
}
