# World catalogs

Each subdirectory is one **setting** — self-contained:

| Folder | Setting |
|--------|---------|
| `shattered_vale/` | The Shattered Vale — fantasy riverlands |
| `caribdus/` | Caribdus — flooded archipelago (50 Brazas / 50 Fathoms inspired) |
| `mistfall_coast/` | Mistfall Coast — coastal d100 investigation (alias `aldo`) |

```bash
make worldpack-examples
bin/worldpack-gen -template shattered_vale -inspect
bin/worldpack-gen -template caribdus -inspect
bin/worldpack-gen -template mistfall_coast -inspect
```

Each `world.json` includes regions, cities, NPCs, bestiary, items, **world_rules** (magic, technology…), and **politics** — independent of which rules system you use at the table.
