# Persistent campaign roster (issue #33)

Player characters can outlive a single session. The **roster** is a pool of saved
characters stored under `~/.thaimaturgy/characters/` (one JSON per character,
using the full 5e sheet from #23), so the same characters can be selected into
new adventures and carry their progression forward across a campaign — no longer
limited to the fixed default party of four.

## Model & storage

- `domain.Character.ID` links a party member to a roster entry (empty for an
  ad-hoc member). It is set when a character is saved to / loaded from the roster.
- `storage` CRUD: `SaveCharacter` (assigns a **non-reusable** id — a readable
  name slug plus a random token — on first save, updates in place otherwise),
  `LoadCharacter`, `ListCharacters` (sorted by name), `DeleteCharacter`. Ids are
  validated against path traversal. Because ids are non-reusable, deleting a
  character and creating another with the same name yields a different id, so a
  stale session link can never silently overwrite the new entry.
- All roster operations are serialized under a mutex, so concurrent autosaves
  and a roster edit can't race each other's writes (and a delete can't be raced
  into resurrecting an entry between the existence check and the write).
- `ListCharacters` returns the successfully decoded characters **alongside** a
  non-nil error naming any unreadable/corrupt files, so an incomplete roster is
  surfaced (in the app dialog and `/roster`) rather than silently dropping
  entries.

## Using it

- **App** — *Edit party… → Roster…* opens the roster:
  - **Save current party → roster** persists every party member (assigning ids
    and linking the live members).
  - each saved character has **Add to party** and a **delete** button.
- **Progression write-back** — on autosave, the progression of party members
  that are **linked to an existing roster entry** (non-empty id) is written back
  to the roster. Ad-hoc members and members whose roster entry was deleted are
  left untouched, so a session never silently creates or resurrects roster
  entries. Autosaves are serialized; if a write-back fails (e.g. disk full) the
  failure is recorded to the session timeline rather than silently ignored.
  Adding a character whose name collides with a party member renames the newcomer
  (e.g. "Bob 2") so party members stay uniquely addressable by name.
- **Telegram** — `/roster` lists the saved characters (read-only "consultar");
  creating and choosing characters is done host-side in the app.

## Compatibility

`Character.ID` is additive (`omitempty`), so sessions and default parties from
before #33 load unchanged — they simply have no roster link until saved.
