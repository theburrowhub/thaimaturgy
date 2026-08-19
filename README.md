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
  stat blocks, and dice (`/roll`, ability checks). The AI never invents die results:
  it rolls through the `roll_dice`/`ability_check` tools (real RNG) and reports the
  result and DC, so you can compare against the difficulty class.
- **Images** — the desktop app renders maps and artwork **inline** (no external viewer).
- **Adventure browser** — a live tree of zones/rooms, NPCs, events and items; click any
  entry to view its full detail (with inline art) and act on it — move the party to a
  room, mark an NPC met or an event triggered — right beside the oracle chat.
- **Virtual DM & multiplayer** — toggle to a mode where the AI runs the game for a party
  of characters, and host it for several players over **Telegram** (`cmd/thaimaturgy-bot`,
  or the in-app "Telegram" button).
- **One core, thin frontends** — the Fyne desktop app (`cmd/thaimaturgy`) and the Telegram
  bot (`cmd/thaimaturgy-bot`) over the same `internal/` engine. The app and module editor
  use the operating system's **native** file/save/folder pickers and message dialogs.

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
# Package the bundled example adventure
make example-module            # → dist/modules/the-sunken-crypt.tar.gz

# Run the desktop app (maps/art render inline)
make run

# Optional: build the Telegram multiplayer bot
make build-bot
```

In the app: **Import module…** → select `dist/modules/the-sunken-crypt.tar.gz` → pick the
adventure to start a session. Then ask the oracle a question, or use `/help` for commands.

## Installation

```bash
git clone https://github.com/theburrowhub/thaimaturgy.git
cd thaimaturgy
go build -o thaimaturgy ./cmd/thaimaturgy
./thaimaturgy
```

### Prebuilt releases

Tagged releases publish multi-arch Docker images to GHCR and binary bundles to the
GitHub Releases page:

```bash
docker pull ghcr.io/theburrowhub/thaimaturgy-server:latest
```

See [docs/releases.md](docs/releases.md) for the full list (server/bot/novel + GUI
binaries, the `claude-cli` image) and how to cut/consume a release.

## Configuration & credentials

thAImaturgy supports **OpenAI, Anthropic, and Google Gemini**, and finds credentials in
this order, **auto-configuring itself** and telling you which it picked up:

1. **Environment API keys** (never written to disk):

   ```bash
   export THAIM_OPENAI_API_KEY=sk-...        # or OPENAI_API_KEY
   export THAIM_ANTHROPIC_API_KEY=sk-ant-... # or ANTHROPIC_API_KEY
   export THAIM_GEMINI_API_KEY=AIza...       # or GEMINI_API_KEY / GOOGLE_API_KEY
   export THAIM_PROVIDER=anthropic           # openai | anthropic | gemini
   export THAIM_MODEL=claude-sonnet-4-20250514
   ```

2. **Reused local logins** — if you're already signed in with another tool, it's picked
   up automatically:
   - **Claude Code** — the OAuth login from the macOS Keychain (`Claude Code-credentials`)
     or `~/.claude/.credentials.json`.
   - **Gemini CLI** — the OAuth login in `~/.gemini/oauth_creds.json`.

On startup the app prints a message like *"Auto-detected Anthropic (Claude) via Claude
Code login (Keychain) — configured automatically."* If nothing is found, a first-run
wizard collects a provider and API key.

### The config file

Settings live in a single, organized **`config.yaml`** in your OS config directory
(`~/Library/Application Support/thaimaturgy/` on macOS, `~/.config/thaimaturgy/` on
Linux, `%AppData%\thaimaturgy\` on Windows), shared by the app and the Telegram bot. It is
**auto-generated on first run** from what was detected and can then be edited by hand.
API keys are never written to it (session-only; persist them via env or a local login);
the Telegram bot token, if you set one, *is* stored here (the file is written `0600`).
Sections:

```yaml
provider:   # name (openai|anthropic|gemini), model, temperature, max_tokens, *_api_key
ui:         # language (en|es), show_scanlines, border_style
session:    # auto_save, auto_save_interval, default_setting
oracle:     # max_tool_iterations, recent_timeline, summarize_after, request_timeout_seconds
import:      # vision_max_images, vision_max_image_mb, max_doc_chars, max_output_tokens
tts:        # enabled, voice, model, speed
```

Data (adventures, sessions) stays under `~/.thaimaturgy/`. A legacy
`~/.thaimaturgy/config.json` is migrated to YAML automatically.

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

The quickest way is the **visual module editor** (a separate binary):

```bash
make run-edit      # form-based editor: build content, import images, package .tar.gz
```

It edits every field through forms, imports images into the module's `assets/`,
validates with the same rules the player uses, and exports an importable `.tar.gz`.
It can also **AI-build a module** from a PDF or a folder of images: the document's text
and images are handed to your configured LLM (with vision), which designs the zones,
NPCs, events, and items and references the extracted maps/art back into them. Requires an
API key; treat the output as a first draft to refine.

Reference and hand-authoring:

- **[docs/adventure-schema.md](docs/adventure-schema.md)** — the full `adventure.json`
  schema.
- **[docs/authoring-guide.md](docs/authoring-guide.md)** — step-by-step authoring and
  packaging (editor and by hand).
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
cmd/thaimaturgy/        Entry point (Fyne desktop app; inline images; editor view)
cmd/thaimaturgy-bot/    Entry point (Telegram multiplayer bot)
internal/
  domain/               Core types: adventure.go (module), session.go (play state),
                        character.go, message.go, config.go
  engine/               oracle.go (LLM loop), tools.go (retrieval + session tools),
                        commands.go (DM commands), format.go, dice.go
  providers/            LLM provider interface + OpenAI/Anthropic
  storage/              module.go (import/validate .tar.gz), storage.go (config/sessions)
  tgbot/                Telegram multiplayer front-end (reusable)
  mcpserve/             Shared `__mcp-tools` subcommand (Claude-CLI MCP backend)
  tts/                  Optional OpenAI text-to-speech (narrate read-aloud)
examples/adventures/    Example modules
docs/                   Schema + authoring guides
```

## License

MIT License

## Acknowledgments

- [Fyne](https://fyne.io) (desktop GUI), [telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) (Telegram bot)
