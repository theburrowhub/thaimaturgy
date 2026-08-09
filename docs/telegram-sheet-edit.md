# Editing your character sheet from Telegram (issue #31)

Players can adjust their own character directly from the chat. Every command edits
**only the sender's claimed character** (pick one first with `/pick`), and every
change is recorded in the session timeline as a `party` entry, so the DM sees it
in context and it shows up in `/log` and in `/me`.

| Command | Effect |
|---------|--------|
| `/hp -5` · `/hp +3` · `/hp =10` | Take damage, heal, or set current HP |
| `/condition <name>` | Apply a 5e condition (e.g. `poisoned`) |
| `/uncondition <name>` | Remove a condition |
| `/gold +50` · `/gold -10` · `/gold =100` | Adjust or set gold |
| `/xp <n>` | Award experience |
| `/item add <name> [xN]` | Add to inventory |
| `/item remove <name> [xN]` | Remove from inventory |
| `/setnote <text>` | Set your character's notes |

## Authorization

A player can only ever edit the character they control (`PlayerCharacterName` of
the sender). There is no way to target another player's sheet from the chat. The
**host** edits any sheet from the desktop app (the full sheet editor, #23) and the
DM adjusts sheets through the AI's tools; those paths are unchanged.

## Safety

- Edits are serialized against an in-flight `/dm` resolution (a change made while
  the DM's turn snapshot is open could otherwise be lost on merge), mirroring
  `/rest`.
- The underlying `Character` mutators clamp to valid ranges (HP within
  `[0, MaxHP]`, gold ≥ 0, XP only increases), so a command can't drive the sheet
  into an invalid state.
- Access to the bot itself is still gated by the chat-id / immutable-user-id
  allow-list (#34).
