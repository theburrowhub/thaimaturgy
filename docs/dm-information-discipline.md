# DM information discipline — diagnosis & design (issue #28)

How the virtual DM must handle **DM-only information** so it never leaks as a
spoiler to the players unless there is an in-fiction reason (the right moment and
event). This documents the current state, the gaps, and the proposed design.

## 1. Is the content well segmented today?

The authored module (`internal/domain/adventure.go`) already separates
player-facing text from DM-only text reasonably well:

| Entity | Player-facing (revealable) | DM-only (secret by default) |
|--------|----------------------------|-----------------------------|
| Adventure | `summary`, `introduction`, `hooks` | `background` ("the truth"), `context` (running guidance) |
| Zone | `name`, `description`, `map_image` | `overview` (DM summary) |
| Room | `read_aloud` (boxed text), `image` | `dm_notes`, `features` (traps/DCs), `encounters`, `treasure` |
| NPC | `appearance`, `image` | `secrets`, `motivations`, `personality`, `voice`, `stat_block` |
| Event | `read_aloud` | `trigger`, `dm_notes`, `consequences`, `outcomes` |
| Item | `name`, `description` | `mechanics` (sometimes) |

**Verdict:** the split exists field-by-field, but it is *implicit* (a reader has
to know which fields are secret) and *coarse* (a field is wholly player-facing or
wholly DM-only; there is no "reveal this room's secret door once someone rolls
Investigation ≥ 15"). There is no explicit per-item visibility marker.

## 2. How the DM is protected today

1. **System-prompt discipline.** `DefaultGMPrompt{EN,ES}` opens with an
   "INFORMATION DISCIPLINE (NO SPOILERS)" section instructing the DM to use
   DM-only fields only to adjudicate, never to reveal them, and to answer only
   with what the party could perceive or already knows — revealing hidden content
   only through play (exploration, successful checks, in-fiction discovery).
2. **Grounding is bounded.** The oracle prompt injects the *current* room + present
   NPCs + recent timeline, not a full dump of every zone/room, so the model isn't
   handed the whole map at once (`engine/oracle.go buildSystemPrompt`).
3. **Player commands can't pull DM fields.** Over Telegram, only a player-safe
   command allow-list is delegated to the shared handler (#20); the DM-facing
   retrieval commands (`/room`, `/npc`, `/zone`, `/event`, `/item`, `/search`)
   are **not** exposed to players. `/map` shows only the current zone (#24) and
   `/portrait` only a **met** NPC (#27). Access itself is gated by chat id and an
   immutable-user-id allow-list (#34).
4. **Retrieval tools are the DM's, not the players'.** `get_room` / `get_npc` etc.
   are invoked by the AI DM to ground itself, never directly by a player.

## 3. Gaps and risks

- **Implicit segmentation.** Nothing marks a field as secret, so the protection
  relies entirely on the prompt "knowing" which fields are DM-only. A new field
  added later could be leaked by default.
- **No conditional revelation.** There is no structured "reveal X when Y"
  (a check succeeds, an event triggers); it's left to the model's judgment.
- **All-or-nothing fields.** A room's `dm_notes` mixes "what the players can find"
  with "the twist"; the model must self-censor within a single blob.
- **No automated verification** that a given narration didn't quote a secret.

## 4. Proposed design

### 4a. Explicit visibility markers (schema, opt-in, backward-compatible)
Add optional, additive fields so authors can be explicit and the model has a
machine-checkable signal:

- `secret: true` on a `Feature`, `Exit`, `Room`, `NPC`, `Event`, or `Item` marks
  it hidden until revealed.
- `reveal_when: "<condition>"` (free text or a small DSL, e.g. `check:Investigation>=15`,
  `event:bell-rung`, `flag:door-opened`) documents the trigger.
- Absent markers preserve today's behavior (the existing implicit split stands).

### 4b. Grounding uses the markers
`buildSystemPrompt` / `FormatRoom` / `FormatNPC` should:
- Clearly label DM-only blocks (e.g. wrap them in a `=== DM-ONLY (never reveal
  verbatim) ===` fence) so the boundary is explicit to the model, not inferred.
- Omit or defer `secret: true` items whose `reveal_when` is not yet satisfied by
  session state (visited rooms, triggered events, flags, recent check results),
  re-including them once the condition holds. This turns "reveal only through
  play" into something the grounding enforces, not just requests.

### 4c. Safeguards
- Keep DM-only retrieval tools off the player surface (already true) and add a
  regression test asserting the Telegram player allow-list never includes
  `room/zone/npc/event/item/search`.
- Prompt: keep and reinforce the INFORMATION DISCIPLINE section; reference the
  explicit DM-ONLY fences.
- Optional post-generation check: flag (not block) a narration that contains a
  long verbatim substring of a `secret`/`background` field, surfaced as a
  `🛠 DEBUG` note to the tester, never to players.

### 4d. Authoring guidance
Document in `authoring-guide.md` which fields are player-facing vs DM-only and how
to use `secret` / `reveal_when`, so modules are authored with the split in mind.

## 5. Implementation plan (phased)
1. **Schema**: add optional `secret` / `reveal_when` to the relevant structs
   (+ validation, + migration no-op) and document them.
2. **Grounding**: DM-ONLY fences in `format.go`; gate `secret` items on
   `reveal_when` against session state in `buildSystemPrompt`.
3. **Safeguards**: allow-list regression test; optional verbatim-leak debug check.
4. **Authoring**: guide updates + example.

Phases 1–2 deliver the core (explicit, enforced segmentation); 3–4 harden and
document. None of it changes behavior for existing modules that don't set the new
markers.
