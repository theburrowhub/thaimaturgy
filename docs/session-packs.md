# Session packs (world + rulesystem)

A session combines two independent catalogs:

| Pack | Answers |
|------|---------|
| **worldpack** (`world_id`) | Where? Who? Is magic real? |
| **rulesystem** (`rulesystem_id`) | How do rolls and combat work? |

## Config (not wired to engine yet)

```json
{
  "world_id": "mistfall_coast",
  "rulesystem_id": "d100",
  "language": "es",
  "notes": "Campaign with Aldo"
}
```

Go type: `worldpack.SessionConfig` in `internal/worldpack/session.go`.

## Creature stat blocks

Use `stat_blocks` keyed by rulesystem (`d100`, `dnd5e`, `savage_worlds`).
Future: `worldpack.CreatureStatBlock(entry, session.RulesystemID)`.

## Examples

| Play | world_id | rulesystem_id |
|------|----------|---------------|
| Generic fantasy | `shattered_vale` | `dnd5e` |
| Nautical pulp | `caribdus` | `savage_worlds` |
| Coastal horror (Aldo) | `mistfall_coast` | `d100` |

CLI aliases (`dnd5e`, `50_brazas`, `aldo`) are convenience only.
