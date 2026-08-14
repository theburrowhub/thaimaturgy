# Rulesystem generator (experimental)

This document describes the **standalone rulesystem pack generator** added in
`internal/rulesystem` and `cmd/rulesystem-gen`. It is intentionally **not wired**
into the running oracle, virtual-DM tools, or character sheet yet — it only
**produces artifacts** that describe how thAImaturgy could support multiple RPG
systems later.

## Why

Today thAImaturgy is shaped around **D&D 5e** (`internal/domain/character.go`,
`internal/engine/tools.go`, embedded SRD creatures). The goal is to support
**generic oracle tools** (attack, spell/power, health, characters, inventory,
conditions, rest, initiative…) mapped per system, starting with:

| Template ID    | System family                          |
|----------------|----------------------------------------|
| `dnd5e`        | Dungeons & Dragons 5th Edition (SRD-ish) |
| `d100`         | Percentile / BRP-style (CoC, RuneQuest…) |
| `savage_worlds`| Savage Worlds (SWADE-inspired)         |

## Pack format

A pack is JSON or YAML with `api_version: rulesystem/v1`:

- **Dice & resolution** — how checks, attacks, spells/powers, and damage work
- **Attributes, skills, resources, conditions**
- **Tool bindings** — canonical tool → LLM tool name/parameters/engine hook
- **Character schema** — fields the system expects on a sheet
- **Prompts** — oracle context text for future multi-system sessions
- **Optional PDF excerpts** — mechanically extracted snippets (not AI summaries)

See `examples/rulesystems/` for generated samples.

## CLI

```bash
make build-rulesystem-gen

# List built-ins
bin/rulesystem-gen -list

# Generate all three starter packs
bin/rulesystem-gen -all -out examples/rulesystems/

# One template
bin/rulesystem-gen -template d100 -format yaml -out dist/rulesystems/

# PDF-assisted (auto-detect family + attach excerpts)
bin/rulesystem-gen -pdf ~/Downloads/my-rules.pdf -out dist/rulesystems/
```

PDF ingestion reuses `internal/ingest.ExtractPDF` (pure Go). Detection is
**heuristic** (keyword scoring), not LLM-based.

## Canonical tools

Defined in `internal/rulesystem/canonical.go`:

- `roll_dice`, `ability_check`, `skill_check`
- `attack`, `cast_spell`, `use_power`
- `update_health`, `apply_condition`, `remove_condition`
- `update_character`, `rest`, `initiative`
- `lookup_creature`, `award_experience`
- `inventory_add`, `inventory_remove`

Each built-in template maps these to system-specific names (e.g. D&D `update_hp`
vs SW `update_wounds`).

## Future integration (not in this PR)

1. Load a pack at session start (`session.rulesystem_id`)
2. Swap `engine/tools.go` definitions from the pack
3. Replace `domain.Character` with pack-driven schema or adapters
4. Optional **LLM enrichment** pass: send PDF excerpts + canonical schema → refined pack
5. Validate generated packs against playtests

## Legal note

Built-in templates are **generic scaffolds** inspired by common mechanics. They
are not reproductions of publisher rulebooks. For commercial systems, supply
your legally obtained PDF and treat output as a **DM aid scaffold** to review
before use.
