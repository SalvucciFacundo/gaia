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

	case "tools":
		handleProfileToolsCLI(args[1:])

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
	fmt.Println("  gaia profile tools [id]   - Ver o configurar herramientas del perfil")
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

	tools := mgr.ActiveTools()
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

func handleProfileToolsCLI(args []string) {
	if len(args) == 0 {
		listProfileTools("")
		return
	}

	switch args[0] {
	case "enable":
		if len(args) < 3 {
			fmt.Println("Uso: gaia profile tools enable <id> <tool>")
			fmt.Println("Ejemplo: gaia profile tools enable assistant bash")
			os.Exit(1)
		}
		enableProfileTool(args[1], args[2])

	case "disable":
		if len(args) < 3 {
			fmt.Println("Uso: gaia profile tools disable <id> <tool>")
			fmt.Println("Ejemplo: gaia profile tools disable assistant bash")
			os.Exit(1)
		}
		disableProfileTool(args[1], args[2])

	case "reset":
		if len(args) < 2 {
			fmt.Println("Uso: gaia profile tools reset <id>")
			fmt.Println("Ejemplo: gaia profile tools reset assistant")
			os.Exit(1)
		}
		resetProfileTools(args[1])

	case "help", "--help", "-h":
		printProfileToolsHelp()

	default:
		listProfileTools(args[0])
	}
}

func listProfileTools(profileID string) {
	mgr, err := getProfileManager()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	targetID := profileID
	if targetID == "" {
		active := mgr.Active()
		if active == nil {
			fmt.Println("Error: no hay perfil activo configurado.")
			os.Exit(1)
		}
		targetID = active.ID()
	}

	profilesList := mgr.List()
	var targetInfo *profiles.ProfileInfo
	for _, p := range profilesList {
		if p.ID == targetID {
			targetInfo = &p
			break
		}
	}
	if targetInfo == nil {
		fmt.Printf("Error: el perfil '%s' no existe.\n", targetID)
		os.Exit(1)
	}

	activeTools := mgr.ProfileTools(targetID)
	enabledMap := make(map[string]bool)
	for _, t := range activeTools {
		enabledMap[t.Name] = true
	}

	catalog := mgr.Catalog()
	if catalog == nil {
		fmt.Println("Error: el catálogo de herramientas no está disponible.")
		os.Exit(1)
	}

	allTools := catalog.List()

	fmt.Printf("Herramientas para el perfil '%s' (%s):\n", targetInfo.ID, targetInfo.Name)
	fmt.Println(strings.Repeat("-", 85))
	fmt.Printf("%-14s %-18s %-14s %s\n", "ESTADO", "HERRAMIENTA", "CATEGORÍA", "DESCRIPCIÓN")
	fmt.Println(strings.Repeat("-", 85))

	for _, tool := range allTools {
		status := "[DESACTIVADA]"
		if enabledMap[tool.Name] {
			status = "[HABILITADA]"
		}
		fmt.Printf("%-14s %-18s %-14s %s\n",
			status,
			tool.Name,
			tool.Category,
			truncate(tool.Description, 45),
		)
	}

	fmt.Println(strings.Repeat("-", 85))
	fmt.Println("Comandos útiles:")
	fmt.Printf("  gaia profile tools enable %s <tool>  - Habilitar una herramienta\n", targetID)
	fmt.Printf("  gaia profile tools disable %s <tool> - Deshabilitar una herramienta\n", targetID)
	fmt.Printf("  gaia profile tools reset %s          - Restablecer herramientas por defecto\n", targetID)
}

func enableProfileTool(profileID, toolName string) {
	mgr, err := getProfileManager()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.EnableTool(profileID, toolName); err != nil {
		fmt.Printf("Error habilitando herramienta '%s' para el perfil '%s': %v\n", toolName, profileID, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Herramienta '%s' habilitada exitosamente para el perfil '%s'.\n", toolName, profileID)
}

func disableProfileTool(profileID, toolName string) {
	mgr, err := getProfileManager()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.DisableTool(profileID, toolName); err != nil {
		fmt.Printf("Error deshabilitando herramienta '%s' para el perfil '%s': %v\n", toolName, profileID, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Herramienta '%s' deshabilitada exitosamente para el perfil '%s'.\n", toolName, profileID)
}

func resetProfileTools(profileID string) {
	mgr, err := getProfileManager()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if err := mgr.ResetTools(profileID); err != nil {
		fmt.Printf("Error restableciendo herramientas para el perfil '%s': %v\n", profileID, err)
		os.Exit(1)
	}

	fmt.Printf("✓ Herramientas para el perfil '%s' restablecidas a los valores por defecto.\n", profileID)
}

func printProfileToolsHelp() {
	fmt.Println("Gestión de Herramientas de Perfiles:")
	fmt.Println("  gaia profile tools [id]                   - Listar herramientas del perfil y su estado")
	fmt.Println("  gaia profile tools enable <id> <tool>     - Habilitar una herramienta para el perfil")
	fmt.Println("  gaia profile tools disable <id> <tool>    - Deshabilitar una herramienta para el perfil")
	fmt.Println("  gaia profile tools reset <id>             - Restablecer herramientas del perfil a valores por defecto")
}

func printProfileHelp() {
	fmt.Println("Gestión de Perfiles de GAIA:")
	fmt.Println("  gaia profile list                         - Listar todos los perfiles disponibles")
	fmt.Println("  gaia profile use <id>                     - Cambiar el perfil activo permanentemente")
	fmt.Println("  gaia profile enable <id>                  - Habilitar un perfil desactivado")
	fmt.Println("  gaia profile disable <id>                 - Deshabilitar un perfil")
	fmt.Println("  gaia profile show                         - Mostrar detalles del perfil activo")
	fmt.Println("  gaia profile tools [id]                   - Listar herramientas del perfil y su estado")
	fmt.Println("  gaia profile tools enable <id> <tool>     - Habilitar una herramienta para el perfil")
	fmt.Println("  gaia profile tools disable <id> <tool>    - Deshabilitar una herramienta para el perfil")
	fmt.Println("  gaia profile tools reset <id>             - Restablecer herramientas del perfil a valores por defecto")
}
