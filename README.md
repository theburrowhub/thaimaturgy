# thAImaturgy

```
 _____ _        _    ___ __  __       _
|_   _| |__    / \  |_ _|  \/  | __ _| |_ _   _ _ __ __ _ _   _
  | | | '_ \  / _ \  | || |\/| |/ _` | __| | | | '__/ _` | | | |
  | | | | | |/ ___ \ | || |  | | (_| | |_| |_| | | | (_| | |_| |
  |_| |_| |_/_/   \_\___|_|  |_|\__,_|\__|\__,_|_|  \__, |\__, |
                                                    |___/ |___/
```

**An AI oracle for the Dungeon Master.** thAImaturgy is a tool for a human DM running a
pre-authored D&D-style adventure. You load an **adventure module** (a `.tar.gz` with the
full adventure and its maps/art), and the app becomes a grounded assistant: it answers
your questions about what should happen, hands you read-aloud text and NPC roleplay,
resolves quick mechanics, and tracks the running state of your session.

> **v2.0 — a change of purpose.** Earlier versions had the AI *play the DM* while a human
> played a character. v2 inverts that: **you are the DM**, and the AI is your oracle,
> grounded in the specific adventure you loaded. It never replaces the DM or controls the
> players — it gives you the right information at the right moment.

## What it does

- **Loads adventure modules** — a `.tar.gz` holding `adventure.json` (backstory, NPCs
  with stats *and* motivations, zones/rooms, events, start/ending) plus referenced map
  and art images.
- **Grounds the oracle in the module** — every answer draws on the adventure's canon;
  the model retrieves rooms/NPCs/events on demand so even large modules fit.
- **Tracks the course of play** — you (and the oracle) record the current room, NPCs
  met, events triggered, story flags, quests, and free-form notes, so answers stay
  contextual as the adventure unfolds.
- **Roleplay & mechanics support** — NPC voice/motivations/secrets, read-aloud text,
  stat blocks, and dice (`/roll`, ability checks).
- **Images** — `/map` and `/art` open maps and artwork in your system's image viewer.
  (A GUI with inline images is planned — see Roadmap.)

## Session view

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ thAImaturgy | anthropic | claude-sonnet-4 | The Sunken Crypt                   │
├──────────────────┬─────────────────────────────────┬──────────────────────────┤
│    LOCATION      │            ORACLE               │       SESSION LOG        │
│                  │                                 │                          │
│ ZONE             │ » How does the guard react if   │ [21:04] Entered Gate     │
│ Village of...    │   they bribe him?               │ [21:05] Met Gate Guard   │
│                  │                                 │ [21:07] Flag gate set    │
│ ROOM             │ Grask is a coward at heart...   │ [21:09] Rolled 2d6 = 8   │
│ The Gate  [/art] │ (cites npc [grask]) He'll take  │                          │
│                  │ the coin and point them to the  │                          │
│ EXITS            │ sealed door. Read-aloud: "..."  │                          │
│  → crypt         │                                 │                          │
│ NPCs HERE        │                                 │                          │
│  Gate Guard      │                                 │                          │
├──────────────────┴─────────────────────────────────┴──────────────────────────┤
│ » /goto crypt-antechamber                                                      │
└────────────────────────────────────────────────────────────────────────────────┘
```

## Quick start

```bash
# Build and package the bundled example adventure
make build
make example-module            # → dist/modules/the-sunken-crypt.tar.gz

# Run
make run
```

In the app: **Import module…** → enter `dist/modules/the-sunken-crypt.tar.gz` → pick the
adventure to start a session. Then ask the oracle a question, or use `/help` for commands.

## Installation

```bash
git clone https://github.com/theburrowhub/thaimaturgy.git
cd thaimaturgy
go build -o thaimaturgy ./cmd/thaimaturgy
./thaimaturgy
```

## Configuration

Set your API key via environment variables (keys are never written to the config file):

```bash
export THAIM_OPENAI_API_KEY=sk-your-api-key       # or OPENAI_API_KEY
export THAIM_ANTHROPIC_API_KEY=sk-ant-your-key    # or ANTHROPIC_API_KEY
export THAIM_PROVIDER=anthropic                    # openai | anthropic
export THAIM_MODEL=claude-sonnet-4-20250514
```

Config lives in `~/.thaimaturgy/config.json`. On first run a wizard collects your
language, provider, and API key.

## DM commands

Type a question to consult the oracle, or a slash command:

| Command | Description |
|---------|-------------|
| `/import <path>` | Import an adventure module (`.tar.gz`) |
| `/goto <room_id>` | Move the party to a room (marks it visited) |
| `/room`, `/look` | Show the current room |
| `/zone [id]` · `/npc <id>` · `/npcs` · `/event <id>` · `/item <id>` | Look up authored content |
| `/search <query>` | Search the whole module |
| `/map [zone]` · `/art <id\|path>` | Open a map/art image in your OS viewer |
| `/note <text>` · `/flag key=true` | Feed the running session state |
| `/roll <dice>` · `/quests` · `/party` · `/status` | Utilities |
| `/save [name]` · `/load [name]` · `/quit` | Session management |

Navigation: `TAB` switch panels · `Ctrl+↑/↓` or `PgUp/PgDn` scroll · `ESC` library ·
`^S` save · `^N` toggle voice · `^Q` quit.

## Creating adventures

- **[docs/adventure-schema.md](docs/adventure-schema.md)** — the full `adventure.json`
  schema.
- **[docs/authoring-guide.md](docs/authoring-guide.md)** — step-by-step authoring and
  packaging.
- **`examples/adventures/the-sunken-crypt/`** — a complete example to copy from.

Modules are stored in `~/.thaimaturgy/adventures/`; play sessions in
`~/.thaimaturgy/sessions/`.

## Development

```bash
make check        # fmt-check + vet + tests (race)
make test         # tests only
make modules      # package every example adventure into dist/modules/
```

### Project structure

```
cmd/thaimaturgy/        Entry point (TUI)
internal/
  domain/               Core types: adventure.go (module), session.go (play state),
                        character.go, message.go, config.go
  engine/               oracle.go (LLM loop), tools.go (retrieval + session tools),
                        commands.go (DM commands), format.go, dice.go
  providers/            LLM provider interface + OpenAI/Anthropic
  storage/              module.go (import/validate .tar.gz), storage.go (config/sessions)
  platform/             OS image-viewer helper
  tui/                  Bubble Tea model + views + styles
  tts/                  Optional OpenAI text-to-speech (narrate read-aloud)
examples/adventures/    Example modules
docs/                   Schema + authoring guides
```

## Roadmap

- **GUI** (`cmd/thaimaturgy-gui`) reusing the same `internal/` core, rendering maps and
  art **inline** so the DM doesn't need an external viewer.

## License

MIT License

## Acknowledgments

- [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), [Bubbles](https://github.com/charmbracelet/bubbles)
