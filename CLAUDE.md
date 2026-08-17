# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# Build
make build              # Build the app binary to bin/thaimaturgy (Fyne GUI)
make build-bot          # Build the Telegram bot to bin/thaimaturgy-bot
make run                # Build and run the app
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

thAImaturgy (v2) is a **DM assistant / oracle** desktop app (Fyne GUI). The human is the
Dungeon Master; the app loads a pre-authored adventure module and answers the DM's
questions grounded in that module, while tracking the running session state. It also has
a **virtual-DM** mode (the AI runs the game for a party of player characters) which can be
hosted for multiplayer over Telegram.

**Guiding principle:** all logic lives in `internal/` and is UI-agnostic, so the desktop
app (`cmd/thaimaturgy`, Fyne GUI, renders maps/art inline) and the Telegram bot
(`cmd/thaimaturgy-bot`) are thin frontends over the same core. Build/run with
`make build` / `make run`; the bot with `make build-bot`. (The earlier Bubble Tea TUI has
been removed — the GUI is the single app binary.)

### Layer Structure

```
cmd/thaimaturgy/main.go       Entry point (Fyne desktop GUI; inline maps/art; editor view)
cmd/thaimaturgy-bot/main.go   Entry point (Telegram multiplayer bot; thin wrapper on internal/tgbot)
internal/
  domain/                  Core types
    adventure.go           Authored module (immutable): Adventure, Zone, Room, NPC,
                           Event, Item, StatBlock… + ValidateAdventure + lookups
    session.go             Mutable play state: SessionState (structured + free-form
                           timeline), Session (binds state+adventure+config)
    character.go           AbilityScores/Modifier (reused by StatBlock; party tracking)
    message.go, config.go  Conversation; config + oracle system prompts (EN/ES)
  engine/
    oracle.go              LLM loop + context builder (grounded in the module)
    tools.go               Tool set + ToolRouter (retrieval + session mutation + dice)
    commands.go            DM command parser/handler (/goto, /npc, /map, /note, …)
    format.go              Renders adventure content to text (shared by tools/commands/GUI)
    dice.go                Dice rolling engine (unchanged from v1)
  providers/               LLM provider interface + OpenAI/Anthropic/Gemini (text +
                           vision via Message.Images); providers.New(config) factory.
                           Anthropic/Gemini support OAuth tokens (reused local logins)
  auth/                    Detect local credentials (env keys, Claude Code login via
                           Keychain/file, Gemini CLI login) + AutoConfigure(config)
  storage/
    module.go              Import/extract/validate .tar.gz modules (zip-slip safe),
                           list/load adventures, resolve image paths
    package.go             PackageModule (dir → .tar.gz) + ExtractModule (used by editor)
    storage.go             Config, env/API keys, session save/load
  ingest/                  Pure-Go extraction: PDF text+images and folder images
                           (ExtractPDF/CollectDirImages; FromPDF/FromDirectory mechanical)
  aibuild/                 AI-driven module authoring: curates extracted images with
                           vision (classify/caption, drop decorative), hands text+images
                           to an LLM (with continuation for large outputs) → structured
                           Adventure; sanitizes references, enriches the image catalog
  nativeui/                Native OS file/save/folder pickers + message dialogs
                           (wraps ncruces/zenity); used by the GUI and editor
  tgbot/                   Telegram multiplayer front-end (reusable): hosts a virtual-DM
                           session over a chat (/party, /pick, /do, /dm…); used by the
                           bot binary and, in-process, by the desktop app
  mcpserve/                Shared `__mcp-tools` subcommand (serves the ToolRouter over MCP
                           for the Claude-CLI backend); used by the app and the bot
  tts/                     Optional OpenAI TTS (narrate read-aloud text)
  types/                   Shared tool types
```

### Key Data Flow

1. DM input in the GUI (`cmd/thaimaturgy`) → `engine/commands.go` (`ParseCommand`/`CommandHandler`).
2. Slash commands act locally (look up module content, mutate session, open images).
   Free-form text → `engine/oracle.go` (`Oracle.Ask`); a multiplayer round →
   `Oracle.RunGroupTurn`.
3. The oracle builds a grounded system prompt (adventure summary + current room + present
   NPCs + session state + recent timeline) and runs a tool-calling loop.
4. Tools (`engine/tools.go`) either **retrieve** authored content (`get_room`, `get_npc`,
   `get_event`, `search_module`) or **mutate** the session (`set_location`,
   `trigger_event`, `set_flag`, `log_note`, player-character tools…) via `ToolRouter.Execute()`.
5. The GUI renders the three session panels: Location/Character, Oracle/DM transcript, Session log.

### Adventure modules

A module is a `.tar.gz`: `adventure.json` at the root + referenced `assets/` images.
Imported to `~/.thaimaturgy/adventures/<id>/`; sessions saved in `~/.thaimaturgy/sessions/`.
Schema: `docs/adventure-schema.md`. Authoring: `docs/authoring-guide.md`. Example:
`examples/adventures/the-sunken-crypt/`. Package with `make example-module` / `make modules`.

### App Screens (GUI, `cmd/thaimaturgy`)

- **Library** — imported adventures (play / edit / delete), resumable sessions (with save
  time + rename), New/author, Import, Settings.
- **Session** — 3 panels (adventure browser or party sheet / Oracle-or-DM transcript /
  Session log) with a mode toggle (Oracle ↔ Virtual DM), dice mini-app, and a Telegram
  host button (virtual-DM mode only).
- **Editor** — module authoring (forms → `.tar.gz`), AI import.
- **Settings** — provider/model, languages, timeouts, API keys, Telegram token/chat id.

### Provider Configuration

Providers: `openai`, `anthropic`, `gemini`. Credentials are auto-detected at startup
(`auth.AutoConfigure`) and the chosen source is reported to the user. Sources, in order:

```bash
# 1) Environment API keys (never persisted)
THAIM_OPENAI_API_KEY     # or OPENAI_API_KEY
THAIM_ANTHROPIC_API_KEY  # or ANTHROPIC_API_KEY
THAIM_GEMINI_API_KEY     # or GEMINI_API_KEY / GOOGLE_API_KEY
THAIM_PROVIDER           # openai | anthropic | gemini
THAIM_MODEL              # Model ID
# 2) Reused local logins: Claude Code (Keychain / ~/.claude/.credentials.json),
#    Gemini CLI (~/.gemini/oauth_creds.json) — used as OAuth bearer tokens.
```

Config is an organized **`config.yaml`** in the OS config dir
(`os.UserConfigDir()/thaimaturgy/config.yaml`), shared by all frontends,
auto-generated on first run and migrated from any legacy `config.json` (no secrets
written). Sections: provider, ui, session, oracle, import, tts, spoiler_guard. The nested YAML maps
to/from the flat `domain.Config` in `storage/config_yaml.go`. Adventures live in
`~/.thaimaturgy/adventures/`, sessions in `~/.thaimaturgy/sessions/`.
