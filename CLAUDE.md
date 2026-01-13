# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# Build
make build              # Build binary to bin/thaimaturgy
make run                # Build and run
make dev                # Run with go run (faster iteration)

# Testing
make test               # Run tests with race detector
make test-verbose       # Run tests with verbose output
make test-coverage      # Run tests with coverage report

# Code Quality
make lint               # Run golangci-lint
make fmt                # Format code with gofmt
make vet                # Run go vet
make check              # Run all checks (format, vet, test)

# Dependencies
make deps               # Download dependencies
make tidy               # Tidy go.mod
```

## Architecture Overview

thAImaturgy is a retro-styled AI-powered dungeon master TUI application built with Go.

### Layer Structure

```
cmd/thaimaturgy/main.go    Entry point, initializes storage, config, and TUI
internal/
  domain/                  Core domain types (Character, World, GameState, Config)
  engine/                  Game logic
    orchestrator.go        AI conversation loop with tool calling
    tools.go               AI tool definitions and execution (ToolRouter)
    commands.go            User command parser and handler (/roll, /save, etc)
    dice.go                Dice rolling engine
  providers/               LLM provider interface and implementations
    provider.go            Provider interface definition
    openai.go              OpenAI implementation
    anthropic.go           Anthropic implementation
  storage/                 Persistence (config, saves, env files)
  tui/                     Bubble Tea TUI
    model.go               Main TUI model with screens (Boot, Menu, Wizard, Game)
    styles.go              Lip Gloss styles
  tts/                     Text-to-speech client (OpenAI TTS)
  types/                   Shared types for tools
```

### Key Data Flow

1. User input in TUI (`tui/model.go`) goes to `engine/commands.go` for parsing
2. Non-command input triggers `engine/orchestrator.go` which calls LLM providers
3. LLM can invoke tools defined in `engine/tools.go` (dice rolls, HP updates, inventory, etc.)
4. Tool results modify `domain.GameSession` state
5. TUI renders updated state

### AI Tool System

The AI DM can call tools defined in `engine/tools.go`:
- `roll_dice`, `skill_check`, `saving_throw` - Dice mechanics
- `update_hp`, `update_gold`, `award_xp` - Character state
- `add_item`, `remove_item`, `set_condition` - Inventory/conditions
- `set_location`, `add_quest` - World state

Tools are executed via `ToolRouter.Execute()` which updates `GameSession` and returns results to the LLM.

### TUI Screens

Defined in `tui/model.go`:
- `ScreenBoot` - Splash screen
- `ScreenConfig` - API key setup wizard
- `ScreenMenu` - Main menu
- `ScreenWizard` - Character creation
- `ScreenGame` - Main gameplay with 3-panel layout
- `ScreenSaves` - Load game
- `ScreenHelp` - Help screen

### Provider Configuration

Set via environment variables:
```bash
THAIM_OPENAI_API_KEY    # or OPENAI_API_KEY
THAIM_ANTHROPIC_API_KEY # or ANTHROPIC_API_KEY
THAIM_PROVIDER          # openai or anthropic
THAIM_MODEL             # Model ID
```

Config stored in `~/.thaimaturgy/config.json`, saves in `~/.thaimaturgy/saves/`.
