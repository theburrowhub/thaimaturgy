package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/theburrowhub/thaimaturgy/internal/storage"
	"github.com/theburrowhub/thaimaturgy/internal/tui"
)

func main() {
	store, err := storage.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize storage: %v\n", err)
		os.Exit(1)
	}

	if err := store.LoadEnvFile(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load .env file: %v\n", err)
	}

	config, err := store.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	model := tui.NewModel(store, config)

	cleanup := func() {
		if err := model.Cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to cleanup: %v\n", err)
		}
	}
	defer cleanup()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cleanup()
		os.Exit(0)
	}()

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		cleanup()
		os.Exit(1)
	}

	if err := store.SaveConfig(config); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to save config: %v\n", err)
	}
}
