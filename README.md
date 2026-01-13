# thAImaturgy

```
 _____ _        _    ___ __  __       _
|_   _| |__    / \  |_ _|  \/  | __ _| |_ _   _ _ __ __ _ _   _
  | | | '_ \  / _ \  | || |\/| |/ _` | __| | | | '__/ _` | | | |
  | | | | | |/ ___ \ | || |  | | (_| | |_| |_| | | | (_| | |_| |
  |_| |_| |_/_/   \_\___|_|  |_|\__,_|\__|\__,_|_|  \__, |\__, |
                                                    |___/ |___/
```

A retro-styled AI-powered dungeon master TUI application. Experience classic text adventures with the power of modern LLMs acting as your Dungeon Master.

## Features

- **Retro TUI Interface**: CGA/EGA-inspired color palette with classic box-drawing characters
- **AI Dungeon Master**: Powered by OpenAI or Anthropic LLMs
- **D&D 5e Character Sheet**: Full character tracking with stats, inventory, conditions
- **Dice Rolling**: Standard dice notation support (1d20, 2d6+3, etc.)
- **Save/Load System**: Persistent game sessions
- **Tool Calls**: AI can roll dice, manage inventory, update HP, and more

## Screenshot

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ thAImaturgy | openai | gpt-4o-mini | Aldric the Brave                       │
├──────────────────┬────────────────────────────────┬─────────────────────────┤
│   CHARACTER      │         NARRATION              │      EVENT LOG          │
│                  │                                │                         │
│ Aldric           │ You stand at the entrance of   │ [14:32] Rolled 1d20+3   │
│ Level 3 Human    │ the ancient dungeon. Torches   │         = 17            │
│ Fighter          │ flicker in rusty sconces,      │ [14:31] Gained 50 gold  │
│                  │ casting dancing shadows on     │ [14:30] Quest added:    │
│ HP: 28/28        │ moss-covered stone walls...    │         Find the Gem    │
│ AC: 16  Init: +2 │                                │                         │
│                  │ What do you do?                │                         │
│ STR: 16 (+3)     │                                │                         │
│ DEX: 14 (+2)     │ - Proceed cautiously           │                         │
│ CON: 15 (+2)     │ - Light your own torch         │                         │
│ INT: 10 (+0)     │ - Listen for sounds            │                         │
│ WIS: 12 (+1)     │ - Search for traps             │                         │
│ CHA: 8  (-1)     │                                │                         │
│                  │                                │                         │
│ Gold: 50 XP: 300 │                                │                         │
├──────────────────┴────────────────────────────────┴─────────────────────────┤
│ > I cautiously proceed down the corridor, sword drawn...                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/theburrowhub/thaimaturgy.git
cd thaimaturgy

# Build
go build -o thaimaturgy ./cmd/thaimaturgy

# Run
./thaimaturgy
```

### Go Install

```bash
go install github.com/theburrowhub/thaimaturgy/cmd/thaimaturgy@latest
```

## Configuration

### Environment Variables

Set your API key using environment variables:

```bash
# For OpenAI (default provider)
export THAIM_OPENAI_API_KEY=sk-your-api-key
# or
export OPENAI_API_KEY=sk-your-api-key

# For Anthropic
export THAIM_ANTHROPIC_API_KEY=sk-ant-your-api-key
# or
export ANTHROPIC_API_KEY=sk-ant-your-api-key

# Optional: Set provider and model
export THAIM_PROVIDER=openai  # or anthropic
export THAIM_MODEL=gpt-4o-mini  # or claude-3-5-sonnet-20241022
```

### Configuration File

Config is stored in `~/.thaimaturgy/config.json`:

```json
{
  "provider": "openai",
  "model": "gpt-4o-mini",
  "temperature": 0.8,
  "max_tokens": 2048,
  "show_scanlines": false,
  "border_style": "rounded",
  "default_setting": "fantasy",
  "auto_save": true,
  "auto_save_interval": 300
}
```

**Note**: API keys are never stored in the config file for security. Always use environment variables.

## Usage

### Starting the Game

1. Run `thaimaturgy`
2. Press ENTER on the boot screen
3. Select "New Campaign" from the menu
4. Create your character:
   - Enter a name
   - Choose your race
   - Choose your class
   - Review your ability scores (press R to reroll)
   - Confirm to begin

### Commands

Type commands starting with `/` or `:`:

| Command | Description |
|---------|-------------|
| `/help`, `/h`, `/?` | Show help |
| `/new`, `/n` | Start new campaign |
| `/save [name]`, `/s` | Save current game |
| `/load [name]`, `/l` | Load saved game |
| `/quit`, `/q` | Exit game |
| `/roll <dice>`, `/r` | Roll dice (e.g., `/roll 2d6+3`) |
| `/status`, `/st` | Show character status |
| `/inv`, `/i` | Show inventory |
| `/inv add <item>` | Add item to inventory |
| `/inv rm <item>` | Remove item |
| `/cond add <cond>` | Add condition |
| `/cond rm <cond>` | Remove condition |
| `/char set key=val` | Set character attributes |
| `/provider <name>` | Set LLM provider |
| `/model <id>` | Set model |
| `/temp <0-2>` | Set temperature |

### Gameplay

- Type any text without `/` to interact with the AI Dungeon Master
- The DM will describe scenes and present options
- Use suggested actions or describe your own actions
- The AI will roll dice when outcomes are uncertain

### Navigation

| Key | Action |
|-----|--------|
| TAB | Switch between panels |
| PgUp/PgDown | Scroll content |
| ESC | Return to menu |
| ENTER | Submit command/action |
| Ctrl+C | Quit |

## Saves

Game saves are stored in `~/.thaimaturgy/saves/`:

```json
{
  "save_name": "my_adventure",
  "character": {
    "name": "Aldric",
    "race": "Human",
    "class": "Fighter",
    "level": 3,
    "abilities": { "str": 16, "dex": 14, ... },
    "max_hp": 28,
    "current_hp": 28,
    "inventory": [...],
    "conditions": []
  },
  "world": {
    "setting": "fantasy",
    "current_location": {...},
    "quests": [...],
    "day_number": 1
  },
  "conversation": {...},
  "event_log": {...}
}
```

## Supported Models

### OpenAI
- `gpt-4o` - Most capable
- `gpt-4o-mini` - Fast and affordable (default)
- `gpt-4-turbo` - Previous generation
- `gpt-3.5-turbo` - Budget option

### Anthropic
- `claude-3-5-sonnet-20241022` - Best balance
- `claude-3-opus-20240229` - Most capable
- `claude-3-haiku-20240307` - Fastest

## Development

### Running Tests

```bash
go test ./...
```

### Project Structure

```
thaimaturgy/
├── cmd/thaimaturgy/
│   └── main.go           # Entry point
├── internal/
│   ├── domain/           # Core types
│   │   ├── character.go  # Character model
│   │   ├── world.go      # World state
│   │   ├── message.go    # Conversation
│   │   ├── event.go      # Event system
│   │   ├── config.go     # Configuration
│   │   └── game.go       # Game session
│   ├── engine/           # Game engine
│   │   ├── dice.go       # Dice roller
│   │   ├── commands.go   # Command parser
│   │   ├── tools.go      # AI tools
│   │   └── orchestrator.go
│   ├── providers/        # LLM providers
│   │   ├── provider.go   # Interface
│   │   ├── openai.go     # OpenAI
│   │   └── anthropic.go  # Anthropic
│   ├── storage/          # Persistence
│   │   └── storage.go
│   └── tui/              # Terminal UI
│       ├── model.go      # Bubble Tea model
│       └── styles.go     # Lipgloss styles
├── go.mod
├── go.sum
└── README.md
```

## License

MIT License

## Acknowledgments

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Style definitions
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
