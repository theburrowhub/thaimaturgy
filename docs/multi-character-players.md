# One player, several characters (issue #29)

A single player can control **more than one** party character and choose which
one acts or speaks — useful in small groups where one person runs several PCs.

## Model

`PlayerSlot` (in `SessionState.Players`, keyed by player id) holds:

- `Characters []string` — all party members the player controls;
- `Active string` — the default one used when an action/line doesn't name a
  character;
- `DisplayName`.

The legacy single-character field (`CharacterName`) is migrated into
`Characters`/`Active` on load (`migratePlayerSlots`), so sessions saved before
#29 — and the 1-player-1-character case — keep working unchanged.

Round actions are keyed by **player + character**, so a player with several PCs
can declare one action per character in a round; resubmitting replaces that
character's action. `PendingPlayers` lists the controlled *characters* still to
act (as "Name (player)"), so a round is complete only when every controlled
character has acted.

## Telegram

- `/pick <name>` — repeatable; each claim adds a character and makes it active.
- `/as <name>` (alias `/switch`) — choose the active character.
- `/do [name:] <action>` — act; an optional `name:` prefix targets a specific
  controlled character (only treated as a selector when it names one of yours, so
  a mid-sentence colon isn't misread), otherwise the active one acts.
- `/chat [name:] <line>` — same targeting for in-character dialogue.
- `/me [name]` — show the active character's sheet, or a named one; when you
  control several it lists them with the active one starred.
- `/party` — shows every character and who plays it (a player appears for each of
  their characters).

## App

The desktop app is a solo DM console: the human directs the whole party through
the party panel and the oracle, so it isn't affected by per-player assignment.
It shares the same `SessionState`, so multi-character player slots created over
Telegram load and display correctly. Assignment/ownership is a Telegram concern.

## Compatibility

`Characters`/`Active` are additive; `CharacterName` remains for loading old saves
and is migrated on first load. The 1:1 flow (`/pick` once, `/do <action>`)
behaves exactly as before.
