# Worldpack Generator

The **worldpack** package produces portable **setting catalogs** for thAImaturgy — one self-contained world per folder with geography, NPCs, politics, world rules (magic, technology…), bestiary, items, and encounter tables.

Packs are **not wired into the live engine yet**. They are standalone JSON/YAML artifacts the oracle can query once integrated.

## Design: one folder per world

Worlds are organized by **setting name**, not by game system:

```
examples/worlds/
  shattered_vale/world.json   # The Shattered Vale
  caribdus/world.json         # Caribdus (flooded archipelago)
  westeros/world.json         # (future) A Song of Ice and Fire setting
```

Source mirrors the same layout:

```
internal/worldpack/worlds/
  register.go
  shattered_vale/pack.go
  caribdus/pack.go
```

Each `world.json` is self-contained:

| Section | Purpose |
|---------|---------|
| `setting.world_rules` | Does magic exist? Tech level? Death/travel norms |
| `setting.politics` | Powers, conflicts, law |
| `regions`, `cities`, `locations` | Geography and places |
| `npcs`, `factions`, `lore` | People and story hooks |
| `creatures`, `items`, `encounter_tables` | Playable content |
| `tools`, `indexes` | Query API for the oracle |

**Rulesystem is a hint, not the folder key.** `suggested_rulesystem` and `playable_with` say which rule packs fit the tone — you can run Caribdus with Savage Worlds, D&D 5e, or d100.

## API Version

`worldpack/v1`

## Quick Start

```bash
# List built-in worlds
go run ./cmd/worldpack-gen -list

# Generate one world (output: examples/worlds/<id>/world.json)
go run ./cmd/worldpack-gen -template shattered_vale -out examples/worlds -inspect

# Aliases
go run ./cmd/worldpack-gen -template dnd5e -inspect          # -> shattered_vale
go run ./cmd/worldpack-gen -template 50_brazas -inspect      # -> caribdus
go run ./cmd/worldpack-gen -template 50_fathoms -inspect     # -> caribdus

# Generate every built-in world
go run ./cmd/worldpack-gen -all -out examples/worlds
```

## CLI Flags

| Flag | Description |
|------|-------------|
| `-list` | List built-in worlds |
| `-template` | World ID or alias (`shattered_vale`, `caribdus`, `dnd5e`, `50_brazas`, …) |
| `-all` | Generate every built-in world |
| `-out` | Output root directory (default `examples/worlds`) or explicit `.json`/`.yaml` file |
| `-inspect` | Print human-readable pack summary |
| `-validate` | Run strict validation (non-zero exit on failure) |
| `-build-indexes` | Rebuild lookup indexes before save |
| `-format` | `json` (default) or `yaml` |
| `-name` | Override pack display name |
| `-lang` | Language code (default `en`) |

## Package Layout

```
internal/worldpack/
  schema.go      # Pack struct, Setting, WorldRules, Politics
  builder.go     # NewBaseWorld, AddCity, AddCreatureFromSRD, …
  canonical.go   # 20 world query tools
  validate.go    # Referential integrity checks
  inspect.go     # Text inspection report
  pack.go        # Load/Save and entity lookups
  index.go       # BuildIndexes + SearchWorld + roll helpers
  generate.go    # World → examples/worlds/<id>/world.json
  registry.go    # Built-in world registry + aliases
  worlds/
    register.go
    shattered_vale/pack.go
    caribdus/pack.go
cmd/worldpack-gen/main.go
```

## Adding a new world

1. Create `internal/worldpack/worlds/<world_id>/pack.go` with `func Build() *worldpack.Pack`.
2. Register in `internal/worldpack/worlds/register.go` and `registry.go` (optional aliases).
3. Run `go run ./cmd/worldpack-gen -template <world_id> -inspect`.
4. Commit `examples/worlds/<world_id>/world.json`.

Keep ambientación, política, reglas del mundo, personajes y lugares **inside that world folder** — do not split by rulesystem.

## Built-in worlds

### The Shattered Vale (`shattered_vale`)

Generic fantasy riverlands — city-states, guild leagues, creeping wilderness after a magical cataclysm.

- **Suggested rulesystem:** `dnd5e` · **Also:** `savage_worlds`, `d100`
- 5 regions, 3 cities, 22 locations, 16 NPCs, 15 creatures

### Caribdus (`caribdus`)

Flooded archipelago — colonial fleets, pirate republics, sea-witch covens. Inspired by nautical pulp (50 Fathoms / 50 Brazas tone).

- **Suggested rulesystem:** `savage_worlds` · **Also:** `dnd5e`, `d100`
- 4 regions, 3 ports, 20 locations, 12 NPCs, 12 creatures

## Canonical World Tools

Twenty query tools in `canonical.go`:

- Geography: `get_region`, `get_city`, `get_district`, `get_location`, `list_city_locations`, `find_nearby_locations`, `get_map`, `describe_travel`
- Population: `get_npc`, `list_location_npcs`, `get_faction`
- Encounters: `list_location_creatures`, `roll_encounter_table`, `list_bestiary`, `filter_creatures_by_habitat`, `get_creature`
- Treasure: `list_location_items`, `get_item`
- Reference: `search_world`, `get_lore`
