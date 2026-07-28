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

thAImaturgy (v2) is a **DM assistant / oracle** TUI. The human is the Dungeon Master;
the app loads a pre-authored adventure module and answers the DM's questions grounded in
that module, while tracking the running session state. (v1 — where the AI *was* the DM
and the human played a character — has been removed and its infrastructure repurposed.)

**Guiding principle:** all logic lives in `internal/` and is UI-agnostic, so the TUI
(`cmd/thaimaturgy`) and the Fyne desktop GUI (`cmd/thaimaturgy-gui`, renders maps/art
inline) are thin frontends over the same core. Build/run the GUI with `make build-gui` /
`make run-gui`.

### Layer Structure

```
cmd/thaimaturgy/main.go       Entry point (TUI)
cmd/thaimaturgy-gui/main.go   Entry point (Fyne GUI; inline maps/art)
cmd/thaimaturgy-edit/         Entry point (module authoring editor; forms → .tar.gz)
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
    format.go              Renders adventure content to text (shared by tools/commands/TUI)
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
  aibuild/                 AI-driven module authoring: hands extracted text+images to an
                           LLM (with vision) → structured Adventure; sanitizes references
  platform/open.go         OS image-viewer launcher (open/xdg-open/start)
  tui/
    model.go               Bubble Tea model + update logic (screens/state)
    views.go               View rendering for each screen
    styles.go              Lip Gloss styles
  tts/                     Optional OpenAI TTS (narrate read-aloud text)
  types/                   Shared tool types
```

### Key Data Flow

1. DM input in `tui/model.go` → `engine/commands.go` (`ParseCommand`/`CommandHandler`).
2. Slash commands act locally (look up module content, mutate session, open images).
   Free-form text → `engine/oracle.go` (`Oracle.Ask`).
3. The oracle builds a grounded system prompt (adventure summary + current room + present
   NPCs + session state + recent timeline) and runs a tool-calling loop.
4. Tools (`engine/tools.go`) either **retrieve** authored content (`get_room`, `get_npc`,
   `get_event`, `search_module`) or **mutate** the session (`set_location`,
   `trigger_event`, `set_flag`, `log_note`, …) via `ToolRouter.Execute()`.
5. TUI renders the three session panels: Location, Oracle transcript, Session log.

### Adventure modules

A module is a `.tar.gz`: `adventure.json` at the root + referenced `assets/` images.
Imported to `~/.thaimaturgy/adventures/<id>/`; sessions saved in `~/.thaimaturgy/sessions/`.
Schema: `docs/adventure-schema.md`. Authoring: `docs/authoring-guide.md`. Example:
`examples/adventures/the-sunken-crypt/`. Package with `make example-module` / `make modules`.

### TUI Screens

Defined in `tui/model.go`:
- `ScreenBoot` - Splash screen
- `ScreenConfig` - Language + API key setup wizard
- `ScreenLibrary` - Imported adventures, resumable sessions, import/settings/help/quit
- `ScreenImport` - Enter a module path to import
- `ScreenSession` - 3-panel play view (Location / Oracle / Session log) + input
- `ScreenHelp` - Help screen

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

Config stored in `~/.thaimaturgy/config.json` (no secrets); adventures in
`~/.thaimaturgy/adventures/`, sessions in `~/.thaimaturgy/sessions/`.
