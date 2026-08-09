# Mutable world — per-session consequences (issue #21)

The authored module (`adventure.json`) is **immutable**. To make the party's
actions stick — so the DM never re-describes something a character already
changed — the session keeps an **overlay** of DM-recorded consequences layered
on top of the authored text.

## How it works

- `SessionState.WorldEdits` maps an entity target `"<kind>:<id>"`
  (kind ∈ `room|zone|npc|item|event`) to an append-only list of
  `WorldChange{Change, Timestamp}`. It is part of the saved session, so
  consequences survive save/resume, and is empty for sessions that never edited
  the world (backward compatible).
- The AI DM records changes with the **`record_world_change`** tool and can
  review them with **`list_world_changes`**. Both validate that the target names
  a real authored entity, so a typo can't create a dangling entry.
- Reads layer the overlay automatically:
  - the retrieval tools (`get_room`, `get_zone`, `get_npc`, `get_item`,
    `get_event`) append a `*** CURRENT STATE — changes since authored ***`
    block after the authored text, and
  - the always-on grounding (`buildSystemPrompt`) does the same for the
    **current room** and the **NPCs present** every turn.
- Each change is also written to the timeline as a `world` log entry
  (`LogWorld`) for traceability.

We layer consequences on top of the authored text rather than rewriting it:
the authored description is preserved, the change is auditable, and token cost
stays bounded (a few short notes, not a full re-serialization of the module).

## The armor example, end to end

1. The room's authored `read_aloud`: *"A suit of armor stands beside the altar."*
2. A PC drags the armor into the hallway. The DM calls
   `record_world_change{kind:"room", id:"altar",
   change:"The suit of armor has been dragged into the hallway and no longer
   stands by the altar."}`.
3. Later the party returns. The grounding for the altar room now shows the
   authored text **and** the current-state block, so the DM narrates the empty
   altar and does **not** describe the armor back in its original place.
