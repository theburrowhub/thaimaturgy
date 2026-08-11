# Hiding suggested actions on Telegram (issue #63)

The virtual DM ends each narration with a short list of the players' possible
next actions. On Telegram that list can be a mild spoiler — it surfaces options
or hints players might not have discovered yet. We hide it behind a **Telegram
spoiler** (tap to reveal), keeping the narrative itself readable.

## How it works

- The virtual-DM prompt (`DefaultGMPrompt{EN,ES}`) instructs the model to put the
  suggested actions **last**, on their own final line, under the exact heading
  `Possible actions:` / `Posibles acciones:` and nothing after.
- `domain.SplitActions` detects that heading (at the start of a line, either
  language, last occurrence) and splits the reply into the narrative and the
  actions list.
- On Telegram (`internal/tgbot`), the DM narration is sent via `sendNarration`:
  - the **narrative** goes out as plain text (no parse mode → nothing to escape
    or break), then
  - the **actions** follow in a separate **HTML** message: a bold heading plus
    the list wrapped in `<tg-spoiler>…</tg-spoiler>` (HTML-escaped). Long lists
    are chunked so a spoiler tag is never split across Telegram's message limit.

Only the actions list is hidden; the narration is normal. As agreed on the issue,
the odd hint may still surface in the narrative body or the `/log` — the goal is
to prevent the *involuntary* reading of the most obvious spoiler (the suggested
actions).

## App / web

The desktop app and web UI are unaffected: they render the reply (heading +
list) as normal text — there's no involuntary-reading problem in their own
layout, so no spoiler wrapping is applied there.
