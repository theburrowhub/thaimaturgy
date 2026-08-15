# Rulesystem generator (experimental v2)

Standalone **RPG system pack generator** for thAImaturgy. Produces rich,
machine-readable ruleset definitions (`rulesystem/v2`) that describe how the
oracle *would* support multiple game systems — **without wiring into the live
engine yet**.

## Goals

1. **Generic canonical tools** shared across systems (attack, spell/power, HP,
   conditions, inventory, rest, initiative, opposed checks, bennies, sanity…)
2. **System-specific mapping** (D&D 5e, percentile d100, Savage Worlds…)
3. **PDF-assisted scaffolding** — extract text, categorize excerpts, merge into
   rule chapters (mechanical, no LLM required)
4. **Future LLM enrichment** — `enrichment` block defines what an AI pass should
   refine from publisher PDFs

## Pack schema (v2 highlights)

| Section | Purpose |
|---------|---------|
| `attributes`, `skills`, `resources`, `conditions` | Core character model |
| `resolution` | How checks, attacks, saves, spells, damage, death work |
| `combat` | Round structure + action economy |
| `progression` | XP/levels/advances |
| `magic`, `social`, `equipment` | Subsystem models |
| `formulas` | Named expressions (`hp_max`, `parry`, …) |
| `workflows` | Multi-step resolution pipelines (attack → damage → save) |
| `mechanics` | Named rules the oracle should know |
| `tables` | Embedded reference tables (DC guidance, death saves, …) |
| `chapters` | Human-readable rule sections for prompts |
| `tools` | Canonical tool → LLM tool bindings with examples |
| `character`, `creature` | Sheet/stat-block schemas |
| `oracle_guide` | Principles, tool priority, scenarios, anti-patterns |
| `compatibility` | Map to today's `engine/tools.go` hooks |
| `enrichment` | LLM-ready refinement spec |

## Built-in templates

| ID | System | Highlights |
|----|--------|------------|
| `dnd5e` | D&D 5e SRD-ish | 6 abilities, 18 skills, 15 conditions, 9 workflows, 21 tools, 4 chapters |
| `d100` | Percentile BRP/CoC | Opposed rolls, sanity, major wounds, mythos magic |
| `savage_worlds` | SWADE-inspired | Wild die, aces, bennies, shaken/wounds/soak |

Aliases: `dnd-5e`, `brp`, `swade`, `call_of_cthulhu`, …

## CLI

```bash
make build-rulesystem-gen

# Overview table
bin/rulesystem-gen -list

# Generate all three example packs (~1500–2000 lines each)
make rulesystem-examples

# One template + inspection report
bin/rulesystem-gen -template dnd5e -inspect

# Validate strictly (exit 2 on failure)
bin/rulesystem-gen -template d100 -validate

# YAML output
bin/rulesystem-gen -template savage_worlds -format yaml -out dist/rulesystems/

# PDF merge (auto-detect family + categorize excerpts into chapters)
bin/rulesystem-gen -template dnd5e -pdf ~/rules.pdf -out dist/

# Attach LLM enrichment spec (still no API call)
bin/rulesystem-gen -template dnd5e -enrich -out dist/dnd5e.json

# Diff two packs
bin/rulesystem-gen -diff examples/rulesystems/dnd5e.json,dist/dnd5e.json
```

## Architecture

```
internal/rulesystem/
  schema.go          v2 types
  canonical.go       ~30 canonical oracle tools
  formula.go         expression evaluator for derived stats
  validate.go        deep structural validation
  inspect.go         human-readable pack report
  merge.go           PDF excerpts → rule chapters
  enrich.go          LLM enrichment spec builder
  pdf_analyze.go     categorize excerpts + family detection
  generate.go        template + PDF pipeline
  profiles/          rich built-in packs (dnd5e, d100, savage_worlds)
cmd/rulesystem-gen/  CLI
examples/rulesystems/  generated JSON samples
```

Registration uses `profiles/register.go` `init()` to avoid import cycles.

## Canonical tools (selection)

`roll_dice`, `ability_check`, `skill_check`, `saving_throw`, `opposed_check`,
`attack`, `cast_spell`, `use_power`, `update_health`, `damage_roll`,
`concentration_check`, `death_save`, `soak_damage`, `apply_condition`,
`remove_condition`, `update_character`, `rest`, `initiative`, `lookup_creature`,
`award_experience`, `inventory_add`, `inventory_remove`, `spend_benny`,
`draw_benny`, `improve_skill`, `social_conflict`, `fear_sanity`,
`apply_template`, `roll_on_table`, `advance_quest`

Each built-in template binds these to system-specific tool names, parameters,
workflows, preconditions, effects, and examples.

## Future integration (not implemented)

1. `session.rulesystem_id` selects pack at startup
2. `engine/tools.go` definitions generated from pack bindings
3. Adapter over `domain.Character` / `StatBlock`
4. Optional LLM enrichment pass using `enrichment` + PDF excerpts
5. Playtest-driven validation

## Legal

Built-in templates are generic mechanical scaffolds, not publisher text.
Use legally obtained PDFs and review generated packs before table use.
