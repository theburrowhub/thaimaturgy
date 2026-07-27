# Adventure Module Schema (v1.0)

An **adventure module** is a `.tar.gz` archive that packages one authored D&D-style
adventure plus its map and art images. thAImaturgy imports it, gives the LLM the
complete adventure as grounded context, and lets a human DM query it during play.

## Archive layout

```
my-adventure.tar.gz
├── adventure.json        # REQUIRED, at the archive root
└── assets/               # images referenced by adventure.json (any layout)
    ├── maps/zone-1.png
    └── art/npc-mayor.png
```

- `adventure.json` **must** sit at the root of the archive.
- Images are referenced from `adventure.json` by **relative path** (e.g.
  `assets/maps/zone-1.png`). The folder structure under the root is free-form.
- On import the archive is extracted to `~/.thaimaturgy/adventures/<id>/`. Import is
  rejected if it contains absolute paths, `..` traversal, or an oversized entry.

## Top-level object (`Adventure`)

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `schema_version` | string | recommended | Currently `"1.0"`. |
| `id` | string | **yes** | Unique, filesystem-safe (used as the folder name). |
| `title` | string | **yes** | Display name. |
| `author` | string | no | |
| `system` | string | no | e.g. `"D&D 5e"`. |
| `language` | string | no | `"en"`, `"es"`, … |
| `summary` | string | no | Short pitch; always sent to the LLM. |
| `background` | string | no | Hidden lore / the "truth" behind the adventure (DM-only). |
| `introduction` | string | no | How the adventure starts (hook, opening scene). |
| `conclusion` | string | no | Possible endings and how to resolve them. |
| `hooks` | string[] | no | Ways to draw the party in. |
| `zones` | Zone[] | **yes** (≥1) | Regions of the adventure. |
| `npcs` | NPC[] | no | Characters, with stats and roleplay. |
| `events` | Event[] | no | Scripted moments / branching decisions. |
| `items` | Item[] | no | Treasure and notable objects. |
| `factions` | Faction[] | no | Organizations with goals. |
| `lore` | LoreEntry[] | no | World background entries. |
| `images` | ImageRef[] | no | Optional catalog of image assets. |
| `meta` | object | no | Free-form metadata. |

### `Zone`

| Field | Type | Notes |
|-------|------|-------|
| `id` | string | Required, unique. |
| `name` | string | |
| `overview` | string | DM-facing summary of the zone. |
| `description` | string | |
| `map_image` | string | Relative path to a zone map (`/map` opens it). |
| `rooms` | Room[] | |
| `connections` | string[] | Other **zone** IDs reachable from here. |

### `Room`

| Field | Type | Notes |
|-------|------|-------|
| `id` | string | Required, unique across the whole module. |
| `name` | string | |
| `read_aloud` | string | Boxed text to read to players verbatim. |
| `dm_notes` | string | Hidden info: what happens here, secrets, tactics. |
| `image` | string | Relative path to room art (`/art <room_id>`). |
| `npc_ids` | string[] | NPCs present (must exist in `npcs`). |
| `event_ids` | string[] | Events tied to this room (must exist in `events`). |
| `exits` | Exit[] | Connections to other rooms/zones. |
| `encounters` | Encounter[] | Combats/challenges staged here. |
| `treasure` | string[] | |
| `features` | Feature[] | Traps, puzzles, ability checks. |

**`Exit`**: `{ "to": "<room-or-zone-id>", "direction": "north", "description": "...", "locked": false }`
**`Feature`**: `{ "name", "description", "skill", "dc", "success", "failure" }`
**`Encounter`**: `{ "name", "description", "creatures": [...], "difficulty", "tactics" }`

### `NPC`

Both **mechanics** and **roleplay** live here.

| Field | Type | Notes |
|-------|------|-------|
| `id` | string | Required, unique. |
| `name` | string | |
| `role` | string | e.g. `"quest giver"`, `"villain"`. |
| `appearance` | string | |
| `personality` | string | Roleplay guidance. |
| `motivations` | string | What they want (drives improvisation). |
| `secrets` | string | Hidden truths the DM can reveal. |
| `voice` | string | How to portray them (accent, cadence). |
| `knowledge` | string[] | Facts they can share. |
| `sample_dialogue` | string[] | Example lines for inspiration. |
| `disposition` | string | Starting attitude. |
| `stat_block` | StatBlock | Combat mechanics (optional). |
| `image` | string | Relative path to a portrait. |
| `default_location` | string | Room ID where they start (must exist). |

**`StatBlock`**: `{ "ac", "max_hp", "speed", "cr", "abilities": {str,dex,con,int,wis,cha},
"skills": [...], "traits": [...], "actions": [ { "name","description","to_hit","damage" } ] }`

### `Event`

| Field | Type | Notes |
|-------|------|-------|
| `id` | string | Required, unique. |
| `name` | string | |
| `trigger` | string | Player action/condition that fires it. |
| `description` | string | |
| `read_aloud` | string | Boxed text. |
| `dm_notes` | string | How to run it. |
| `consequences` | string | |
| `outcomes` | Outcome[] | Branches: `{ "condition", "result" }`. |

### `Item`, `Faction`, `LoreEntry`, `ImageRef`

- **Item**: `{ "id", "name", "description", "rarity", "mechanics", "image" }`
- **Faction**: `{ "id", "name", "description", "goals" }`
- **LoreEntry**: `{ "title", "content" }`
- **ImageRef** (optional catalog): `{ "id", "path", "kind": "map"|"art", "description" }`

## Validation rules

Import fails (with a listed reason) if any of these do not hold:

- `id` and `title` are present; at least one zone exists.
- Zone, room, NPC, and event IDs are non-empty and unique.
- Every `room.npc_ids` / `room.event_ids` / `npc.default_location` / `exit.to`
  reference points at something that exists.
- Every referenced image (zone `map_image`, room/NPC/item `image`, and `images[]`
  paths) exists in the archive.

## How the module reaches the LLM

The oracle always receives: the adventure `summary` + `background`, the **current room**
in full (read-aloud + DM notes), the **NPCs present** (dossier + stat block), tracked
session state, and the recent timeline. Everything else (other rooms, NPCs, events,
items, lore) is pulled on demand through retrieval tools (`get_room`, `get_npc`,
`get_event`, `get_item`, `search_module`). This keeps context bounded for large modules.

See **[authoring-guide.md](authoring-guide.md)** for a step-by-step walkthrough.
