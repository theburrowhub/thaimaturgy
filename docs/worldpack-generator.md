# Worldpack Generator

The **worldpack** package produces portable world content catalogs for thAImaturgy — cities, regions, locations, NPCs, creatures, items, and encounter tables. Packs mirror the `rulesystem` generator pattern but target **world content** instead of rules.

Packs are **not wired into the live engine yet**. They are standalone JSON/YAML artifacts the oracle can query once integrated.

## API Version

`worldpack/v1`

## Quick Start

```bash
# List built-in templates
go run ./cmd/worldpack-gen -list

# Generate The Shattered Vale (default template)
go run ./cmd/worldpack-gen -template dnd5e_shattered_vale -out examples/worldpacks

# Alias: dnd5e -> dnd5e_shattered_vale
go run ./cmd/worldpack-gen -template dnd5e -out examples/worldpacks -inspect

# Generate all templates
go run ./cmd/worldpack-gen -all -out examples/worldpacks

# Validate and rebuild indexes explicitly
go run ./cmd/worldpack-gen -template dnd5e -validate -build-indexes -inspect
```

## CLI Flags

| Flag | Description |
|------|-------------|
| `-list` | List built-in templates |
| `-template` | Template ID (`dnd5e_shattered_vale`, alias `dnd5e`) |
| `-all` | Generate every built-in template |
| `-out` | Output directory or explicit `.json`/`.yaml` file |
| `-inspect` | Print human-readable pack summary |
| `-validate` | Run strict validation (non-zero exit on failure) |
| `-build-indexes` | Rebuild lookup indexes before save |
| `-format` | `json` (default) or `yaml` |
| `-name` | Override pack display name |
| `-lang` | Language code (default `en`) |

## Package Layout

```
internal/worldpack/
  schema.go      # Pack struct and API types
  canonical.go   # 20 world query tools (get_city, roll_encounter_table, …)
  validate.go    # Referential integrity checks
  inspect.go     # Text inspection report
  pack.go        # Load/Save and entity lookups
  index.go       # BuildIndexes + SearchWorld + roll helpers
  generate.go    # Template → pack generation
  registry.go    # Built-in template registry
  helpers.go     # Shared utilities
  profiles/
    builder.go              # NewBaseWorld, AddCity, AddCreatureFromSRD, …
    dnd5e_shattered_vale.go # Rich generic fantasy world (1000+ lines JSON)
    register.go             # Registers dnd5e_shattered_vale + alias dnd5e
cmd/worldpack-gen/main.go
```

## Canonical World Tools

Twenty query tools are defined in `canonical.go`:

- Geography: `get_region`, `get_city`, `get_district`, `get_location`, `list_city_locations`, `find_nearby_locations`, `get_map`, `describe_travel`
- Population: `get_npc`, `list_location_npcs`, `get_faction`
- Encounters: `list_location_creatures`, `roll_encounter_table`, `list_bestiary`, `filter_creatures_by_habitat`, `get_creature`
- Treasure: `list_location_items`, `get_item`
- Reference: `search_world`, `get_lore`

## Shattered Vale Template

`dnd5e_shattered_vale` is a **generic fantasy** setting (not Forgotten Realms):

- **5 regions**: Northern Marches, Sunlit Coast, Whisperwood, Ironspine Mountains, Undercrypt
- **3 cities**: Millhaven (trade hub), Ironhold (fortress), Thornwall (frontier)
- **22+ locations** including market, tavern, temple, barracks, docks, thieves' alley
- **16 NPCs** with personalities, motivations, and stat blocks where appropriate
- **All 15 embedded SRD creatures** via `srd.Lookup` with habitats and encounter notes
- **26 items** (weapons, armor, potions, gear)
- **9 encounter tables** (forest, road, urban night, dungeon, coast, …)
- **3 factions**: Merchants' League, Order of the Dawn, Red Hand Bandits

Generated JSON is typically **100KB+** (1000+ lines when pretty-printed).

## Engine Compatibility

`Pack.Compatibility.ToolMap` maps worldpack tools to future engine hooks:

| Worldpack Tool | Engine Hook |
|----------------|-------------|
| `get_location` | `get_room` |
| `get_npc` | `get_npc` |
| `get_creature` | `lookup_creature` |
| `search_world` | `search_module` |

## Makefile Targets (patch into repo Makefile)

```makefile
# Build worldpack generator CLI
build-worldpack-gen:
	go build -o bin/worldpack-gen ./cmd/worldpack-gen

# Generate example worldpack JSON files
worldpack-examples: build-worldpack-gen
	mkdir -p examples/worldpacks
	./bin/worldpack-gen -all -out examples/worldpacks -validate
```

## Tests

```bash
go test ./internal/worldpack/... -count=1
```

## Adding a New World Template

1. Create `internal/worldpack/profiles/my_world.go` with a `func MyWorld() *worldpack.Pack` factory.
2. Use `profiles/builder.go` helpers: `NewBaseWorld`, `AddCity`, `AddCreatureFromSRD`, etc.
3. Register in `profiles/register.go` via `worldpack.RegisterBuiltin`.
4. Add tests asserting minimum content counts and validation.

Blank-import `profiles` from `cmd/worldpack-gen/main.go` (already done) so `init()` registration runs.
