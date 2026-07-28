# Authoring & Adding Adventures

This guide walks you through creating a new adventure module and loading it into
thAImaturgy. For the exact field reference, see
**[adventure-schema.md](adventure-schema.md)**.

## What an adventure module is

A `.tar.gz` containing one `adventure.json` (the whole adventure) plus the images it
references. thAImaturgy loads it and acts as a grounded **oracle for the DM**: it
answers your questions using the module's canon, offers read-aloud text and NPC
roleplay, handles quick mechanics, and records what happens at the table.

## Quick start: use the example

```bash
make example-module          # packages examples/adventures/the-sunken-crypt
                             #   → dist/modules/the-sunken-crypt.tar.gz
make run                     # launch the TUI
```

In the app: choose **Import module…**, enter the path
`dist/modules/the-sunken-crypt.tar.gz`, then pick the adventure to start a session.
(Or, inside a session, type `/import dist/modules/the-sunken-crypt.tar.gz`.)

## The easy way: the visual editor

Instead of hand-writing JSON you can use the bundled **module editor**:

```bash
make run-edit
```

- **New / Open folder / Open .tar.gz** — start fresh, edit an unpacked module, or
  edit an existing package.
- A navigation tree on the left (Adventure · Zones → Rooms · NPCs · Events · Items) with
  **+Zone / +Room / +NPC / +Event / +Item / Delete** buttons; forms on the right cover
  every field, including stat blocks, exits, features, encounters, and event outcomes.
- **Import…** next to any image field copies the file into the module's `assets/` and
  fills in the relative path for you.
- **Validate** runs the same checks the player uses; **Save** writes `adventure.json`;
  **Package .tar.gz…** produces an importable module.

### AI-build a module from source material

Two toolbar buttons hand the raw material to an **AI model**, which interprets the whole
document — text *and* images — and returns a complete adventure (zones, rooms, NPCs with
motivations and stat blocks, events, items), referencing the extracted maps and art back
into the zones/rooms/NPCs:

- **Import PDF…** — pick a PDF. Its text and embedded images are extracted, then the
  model designs the module from them (map-like images become zone maps, character/scene
  art is attached to rooms and NPCs).
- **Import images…** — pick a folder of images; the model interprets them visually to
  build the adventure.

Requires an API key (`THAIM_OPENAI_API_KEY` or `THAIM_ANTHROPIC_API_KEY`, same as the
player) — set it and restart the editor. Extraction is pure-Go; interpretation runs
through the configured provider using vision when the model supports it.

Extracted images are **curated with vision** first: each is classified
(map / portrait / scene / item / decorative) and captioned, decorative junk (borders,
logos, textures) is discarded, and the classifications guide how maps and art are
attached to zones, NPCs and rooms. Very large modules are generated across multiple
requests (continuation) so the JSON isn't cut off. References the model invents that
don't resolve (dangling IDs, missing images) are stripped automatically. Treat the
result as a strong first draft: review and refine in the forms, then **Validate**,
**Save**, and **Package .tar.gz**.

If you'd rather write JSON by hand, follow the steps below.

## Create your own — step by step

1. **Make the folder layout:**

   ```
   my-adventure/
   ├── adventure.json
   └── assets/
       ├── maps/
       └── art/
   ```

2. **Write `adventure.json`.** Start from the example
   (`examples/adventures/the-sunken-crypt/adventure.json`) and adapt it. Minimum viable:

   ```json
   {
     "schema_version": "1.0",
     "id": "my-adventure",
     "title": "My Adventure",
     "system": "D&D 5e",
     "summary": "One-line pitch.",
     "background": "The hidden truth for the DM.",
     "introduction": "How it starts.",
     "conclusion": "How it can end.",
     "zones": [
       {
         "id": "zone1", "name": "Starting Area", "map_image": "assets/maps/zone1.png",
         "rooms": [
           { "id": "room1", "name": "Entrance",
             "read_aloud": "Text you read to the players.",
             "dm_notes": "What actually happens here.",
             "npc_ids": ["innkeeper"],
             "exits": [ { "to": "room2", "direction": "north" } ] },
           { "id": "room2", "name": "Back Room" }
         ]
       }
     ],
     "npcs": [
       { "id": "innkeeper", "name": "Bram", "role": "quest giver",
         "personality": "Gruff but kind.", "motivations": "Protect his village.",
         "voice": "Slow, northern drawl.", "default_location": "room1",
         "image": "assets/art/bram.png",
         "stat_block": { "ac": 12, "max_hp": 9, "cr": "0",
           "abilities": {"str":11,"dex":10,"con":11,"int":10,"wis":12,"cha":11} } }
     ],
     "events": [
       { "id": "brawl", "name": "Tavern Brawl", "trigger": "A PC insults a patron.",
         "read_aloud": "Chairs scrape back…", "dm_notes": "Run as a nonlethal skirmish." }
     ]
   }
   ```

3. **Add the images** you referenced (`assets/maps/zone1.png`, `assets/art/bram.png`,
   …). Any common format works; the DM opens them in the system image viewer.

4. **Package it:**

   ```bash
   tar -czf my-adventure.tar.gz -C my-adventure .
   ```

   Or drop the folder in `examples/adventures/` and run `make modules` to package every
   adventure into `dist/modules/`.

5. **Import & play** (see Quick start above).

## Authoring tips

- **Separate voices.** Put player-facing prose in `read_aloud` and hidden guidance in
  `dm_notes`. The oracle keeps them distinct in its answers.
- **Give NPCs motivations, not just stats.** `motivations`, `secrets`, and `voice` are
  what make the oracle useful for improvisation when players go off-script.
- **Use IDs everywhere.** Link rooms↔NPCs↔events by ID. The oracle cites IDs so you can
  `/npc <id>` or `/event <id>` to pull the full entry.
- **Document the ending.** A clear `conclusion` (with branches) lets the oracle steer
  toward a satisfying resolution.
- **Keep image paths relative** and under the archive root.

## Running a session (DM cheat-sheet)

Type a question to the oracle, or use a command:

| Command | Purpose |
|---------|---------|
| `/goto <room_id>` | Move the party (marks the room visited). |
| `/room`, `/look` | Show the current room. |
| `/npc <id>`, `/npcs` | NPC dossier / who's here. |
| `/event <id>`, `/item <id>`, `/zone [id]` | Look up authored content. |
| `/search <query>` | Search the whole module. |
| `/map [zone]`, `/art <id\|path>` | Open a map/art image in your OS viewer. |
| `/note <text>`, `/flag key=true` | Feed the running session state. |
| `/roll 2d6+3`, `/quests`, `/party`, `/status` | Utilities. |
| `/save [name]`, `/load [name]`, `/quit` | Session management. |

The oracle also updates session state itself (location, NPCs met, events triggered,
flags, notes) as you describe what the players do — so its later answers stay
contextual. Sessions are stored in `~/.thaimaturgy/sessions/`; imported modules live in
`~/.thaimaturgy/adventures/`.

## Validation

Import refuses malformed modules and tells you why (missing `id`/`title`, no zones,
duplicate or dangling IDs, or a referenced image that isn't in the archive). Fix the
reported items and re-package. The shipped example is covered by an automated test
(`make test`), so it always stays importable.
