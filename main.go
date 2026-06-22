package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/parksangmin/lazyredis/pkg/config"
	redisclient "github.com/parksangmin/lazyredis/pkg/redis"
	"github.com/parksangmin/lazyredis/pkg/ui"
)

func main() {
	cfg := config.Parse()

	redis := redisclient.New(cfg)
	defer redis.Close()

	model := ui.New(cfg, redis)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
