package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/parksangmin/lazyredis/pkg/config"
	redisclient "github.com/parksangmin/lazyredis/pkg/redis"
	"github.com/parksangmin/lazyredis/pkg/ui"
)

var version = "0.2.0"

func main() {
	cfg := config.Parse()

	r, err := redisclient.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer r.Close()

	app := ui.New(cfg, r)

	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Route typeCacheMsg through Update2
	_ = version
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
