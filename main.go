package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sm010422/lazyredis/pkg/config"
	redisclient "github.com/sm010422/lazyredis/pkg/redis"
	"github.com/sm010422/lazyredis/pkg/ui"
)

var version = "0.3.0"

func main() {
	cfg := config.Parse()

	// Load saved profiles; errors are non-fatal (fall back to CLI flags).
	profiles, _ := config.LoadProfiles()

	// Try to find a profile matching the CLI flags so the active profile is
	// highlighted correctly in the selector modal.
	activeIdx := -1
	for i, p := range profiles {
		if p.Host == cfg.Host && p.Port == cfg.Port {
			activeIdx = i
			break
		}
	}

	// If no match found, synthesize a transient profile from CLI flags.
	if activeIdx < 0 && cfg.Host != "" {
		profiles = append([]config.Profile{{
			Name:  cfg.Addr(),
			Host:  cfg.Host,
			Port:  cfg.Port,
			Color: "blue",
		}}, profiles...)
		activeIdx = 0
	}

	r, err := redisclient.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer r.Close()

	app := ui.New(cfg, r, profiles, activeIdx)
	_ = version

	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
