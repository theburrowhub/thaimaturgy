# Tactical combat — design & phased plan (issue #22)

Research deliverable: *how* to improve tactical combat in Virtual-DM mode before
implementing it. It surveys the options, recommends an approach that fits an
AI DM driven over chat/GUI, and lays out a phased implementation with the
concrete `domain` / `engine` / frontend changes each phase needs.

## 1. Where combat stands today

Virtual-DM play is a **round loop** with no explicit combat structure:

- players declare actions with `/do` → buffered in `SessionState.Round`
  (`RoundAction`), resolved together when someone calls `/dm`
  (`Oracle.RunGroupTurn` → `composeRoundInput` → `Oracle.Ask`);
- the oracle has mechanical tools already: `roll_dice`, `ability_check`,
  `update_hp`, `set_condition` / `remove_condition`, `update_gold`, `award_xp`,
  and (since #26) `lookup_creature` plus full `StatBlock`s on NPCs;
- characters carry a full 5e sheet (#23) including HP, AC, conditions, and
  (for casters) spell slots.

What's missing is *tactical structure*: there is no initiative/turn order, no
action economy, no notion of position or range, and no combat-specific view of
"who's in the fight, at what HP, whose turn is it". The DM improvises all of it
turn to turn, which drifts (forgotten turns, contradicted positions, lost
reactions).

## 2. Positioning: grid vs. zones vs. range bands

| Approach | Fit for an AI-DM over chat/GUI | Cost |
|----------|--------------------------------|------|
| **Square/hex grid** (exact coordinates) | Poor over text/Telegram; the model must track and re-emit a coordinate map every turn (token-heavy, error-prone); hard to render in a chat | High (grid state, movement validation, rendering) |
| **Theatre of the mind, pure prose** | What we do now; natural for an LLM but *untracked* — positions contradict themselves | Low but imprecise |
| **Zones + abstract range bands** (recommended) | Strong: a small, enumerable state the model reads/writes via tools; renders as a short list in both frontends; matches the module's existing zone model | Medium |

### Recommendation: **abstract range bands over "theatre of the mind"**, not a grid

Track each combatant's position as a coarse **range relative to the party /
to a named anchor**, not coordinates:

- range bands: `engaged` (melee, ~5 ft) · `near` (~within 30 ft, a move away) ·
  `far` (ranged only) · `distant` (needs a dash/΄multiple moves).
- optional free-text **position tag** per combatant ("behind the altar", "on the
  gallery") for flavor and cover, which the DM narrates and may use for
  advantage/cover adjudication.

This keeps positional state to one enum + one short string per combatant — cheap
to store, cheap to put in the prompt, trivial to render as a list in the app and
on Telegram, and it degrades gracefully to pure narration. A grid can be added
later as an *optional* per-encounter mode without changing the core model, but is
explicitly out of scope for the recommended design.

## 3. Initiative & turn order

Track initiative **explicitly** in a new `domain.CombatState`:

- at combat start, roll `1d20 + DEX mod` for each combatant (party members from
  their sheet, monsters from their `StatBlock`), store a sorted **initiative
  order**, a `Round` counter, and a `TurnIndex`.
- `CombatState` holds a slice of `Combatant`: `{ID, Name, Side (party/foe/ally),
  Initiative, CurrentHP/MaxHP, AC, Conditions, Range, PositionTag, CharacterID
  (link to a party sheet) or StatBlock (a monster's, copied from SRD/authored),
  Defeated bool}`.
- turn advancement is a tool/command: the current combatant acts, then
  `end_turn` advances `TurnIndex` (wrapping and incrementing `Round`), skipping
  defeated combatants.

The round loop still works: in combat, `/do` records the acting player's
declared action; the DM resolves it **in initiative order** and calls `end_turn`.
Monsters' turns are taken by the AI DM. Out of combat, the loop is unchanged.

## 4. Action economy (5e)

Per combatant per turn, track what's been spent:

- `Action`, `BonusAction`, `Reaction` (reaction persists across the round until
  the combatant's next turn), and `MovementUsed` vs a `Speed` budget.
- reset Action/BonusAction/Movement at the start of the combatant's turn;
  reset Reaction at the start of their turn too (it recharges each round).
- expose it in grounding so the DM stops a player from taking two actions, and as
  helper tools (`use_action`, `use_bonus_action`, `use_reaction`, `spend_move`)
  the DM calls as it adjudicates. Enforcement is *advisory* (the DM is the
  authority) but tracked so the model has ground truth.

## 5. Conditions & their mechanical effect

Conditions already exist on the sheet (#23) and on combatants. Add a small,
**data-driven effect table** (`domain`) mapping each 5e condition to its
mechanical hints — e.g. *Prone*: melee attacks against have advantage, the
creature's attacks have disadvantage; *Restrained*: attacks against have
advantage, its attacks disadvantage, DEX saves disadvantage, speed 0;
*Poisoned*: disadvantage on attacks and ability checks; *Incapacitated*: no
actions/reactions; etc. The table is injected into the combat grounding for any
combatant that has the condition, so the DM applies the modifier consistently
instead of remembering it. This is a lookup table, not an enforcement engine.

## 6. Attacks, AC, rolls, damage, AoE, cover, advantage

Provide **combat-aware tools** that wrap the dice engine so results are grounded
and logged, rather than free-form `roll_dice`:

- `attack_roll{attacker, target, bonus, advantage|disadvantage, damage}` — rolls
  `d20 (+/- adv)`, compares to the target's AC, on hit rolls damage and applies
  it via the existing HP path, logs the exchange.
- `saving_throw{target, ability, dc, advantage|disadvantage}` — for AoE/effects.
- `apply_damage{target, amount, type}` / `apply_healing` — already largely
  covered by `update_hp`; add damage *type* so resistances/immunities from the
  `StatBlock` (#26) can halve/zero it automatically.
- **Advantage/disadvantage** is a first-class parameter on the roll tools; the DM
  decides it from cover, conditions, and positioning (range bands + position
  tags), guided by the condition-effects table.
- **Cover** and **AoE** stay narrative + a save/attack modifier (half cover +2
  AC, three-quarters +5); AoE is resolved as a `saving_throw` per affected
  combatant. No grid templates in the recommended scope.

## 7. Representing combat to players

A single, compact **combat status** view, derived from `CombatState`, rendered in
both frontends and refreshed each turn:

- **App**: a combat panel (replacing/augmenting the party sheet in DM mode)
  listing combatants in initiative order with HP, conditions, range, and a
  "← current turn" marker; the party's own HP is exact.
- **Telegram**: a `/combat` command and an auto-posted status line after each
  resolved turn: initiative order, whose turn, party HP exact, and — importantly
  — **foe HP as bands** ("Bloodied", "Hurt", "Unharmed"), never exact numbers, to
  preserve the information-discipline rule (#28). Conditions and rough range are
  shown.

Information discipline (#28) applies: players see their own sheets exactly and
foes only as much as they could perceive; the combat view must not leak a
monster's exact HP, immunities, or unrevealed abilities.

## 8. Integration with the round loop and tools

- Combat is bracketed by `start_combat{combatants…}` and `end_combat` tools (and
  DM/host commands). `start_combat` rolls initiative and builds `CombatState`;
  `end_combat` clears it and awards XP.
- While `CombatState` is active, the oracle grounding gains a **COMBAT** section
  (initiative order, current turn, each combatant's HP/AC/conditions/range,
  action-economy remaining, and the condition-effects hints). This reuses the
  bounded-grounding discipline already in `buildSystemPrompt`.
- The multiplayer buffer (`/do` → `/dm`) is unchanged out of combat; in combat,
  the DM resolves strictly by initiative and advances turns. Solo/GUI play uses
  the same tools.
- Persistence: `CombatState` lives in `SessionState` (JSON, `omitempty`) so a
  fight survives save/resume; empty for non-combat sessions (backward
  compatible).

## 9. Phased implementation plan

- **Phase A — structure & tracking (MVP).** `domain.CombatState` + `Combatant`;
  `start_combat` / `end_combat` / `end_turn` tools; initiative roll; a read-only
  `/combat` status (app panel + Telegram). Grounding gains the COMBAT section.
  No enforcement — the DM narrates within the tracked structure. *Delivers the
  biggest correctness win: turn order and a shared combat picture.*
- **Phase B — mechanics.** Combat-aware `attack_roll` / `saving_throw` /
  typed `apply_damage` (with resistance/immunity from `StatBlock`); the
  condition-effects table injected into grounding.
- **Phase C — action economy.** Per-turn action/bonus/reaction/movement tracking
  and tools; range bands + position tags with movement between bands.
- **Phase D — representation & polish.** Rich app combat panel, formatted
  Telegram status with foe HP bands, advantage/cover helpers, AoE save sweeps;
  optional opt-in grid mode as a separate, non-default encounter setting.

Each phase is independently shippable and reviewable, and none breaks existing
sessions (all new state is additive and `omitempty`).

## 10. Summary of required changes

- **domain**: `CombatState`, `Combatant`, range-band enum, condition-effects
  table; `SessionState.Combat *CombatState` (additive).
- **engine**: combat tools (`start_combat`, `end_combat`, `end_turn`,
  `attack_roll`, `saving_throw`, action-economy tools); a COMBAT grounding block
  in `buildSystemPrompt`; XP award on `end_combat`.
- **frontends**: app combat panel + `/combat` (and auto status) on Telegram, with
  foe-HP banding to respect information discipline.

Recommended first step to implement: **Phase A**, as its own issue/PR.
