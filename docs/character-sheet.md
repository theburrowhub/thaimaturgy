# Character sheet — full D&D 5e (issue #23)

`domain.Character` is a complete, editable 5e sheet. It is used by the virtual-DM
party, the app's tabletop sheet panel, and (indirectly) the DM, who reads the
party's authoritative sheets every turn.

## Model

Beyond the core fields (identity, ability scores, HP/temp HP, AC, initiative,
speed, proficiency bonus, hit dice, skills, conditions, gold, XP, notes), the
sheet carries:

- **Saving throws** — `SavingThrows []Ability` marks the proficient saves;
  `SaveBonus(ability)` returns the modifier plus proficiency when proficient.
- **Inspiration** — `Inspiration bool`.
- **Languages** and **other proficiencies** (armor / weapons / tools).
- **Features & traits** — `[]Trait{Name, Description, Source}` for racial / class
  / background features.
- **Spellcasting** — a `*Spellcasting` (nil for non-casters, so martial sheets
  and pre-#23 saved sessions stay compact and load unchanged):
  - `Ability` (INT/WIS/CHA), derived `SaveDC` and `AttackBonus`.
  - `Slots` — max/used per spell level 1..9; `UseSpellSlot` / `RestoreSpellSlot`
    / `SpellSlotsRemaining`, and a **long rest restores all slots**.
  - `Spells` — the spellbook (known / prepared); `AddSpell`, `RemoveSpell`,
    `SetSpellPrepared`.

`Normalize()` clamps a hand-edited sheet to a self-consistent state (HP within
`[0, MaxHP]`, non-negative temp HP / gold / XP, hit dice and spell slots used
within their maxima, item quantities ≥ 1), so a stray value typed into the editor
can never persist invalid domain state.

## Generation

`GenerateCharacter` now fills saving-throw proficiencies (the class's two),
starting languages (from the race), and, for caster classes, the spellcasting
block with the correct slot progression: full casters
(wizard/cleric/druid/bard/sorcerer), half casters (paladin/ranger, from level 2),
and warlock Pact Magic. The default party and the AI party planner inherit this.

## Editing

- **App** — the party panel shows every section and an **“Edit sheet…”** button
  per character opens a full editor (identity, abilities, combat & resources,
  saving-throw / skill proficiencies, languages, inventory, spellcasting and
  spellbook, notes). Saving normalizes the sheet, applies it under the session
  lock, logs the edit to the timeline for the DM, and autosaves.
- **DM** — reads the party's current sheets every turn (authoritative for
  HP/conditions/etc.) and adjusts them with the existing tools (`update_hp`,
  `set_condition`, `add_item`, …).
- **Telegram** — editing a sheet from the chat (and the private-DM management
  flow) is delivered by its dedicated issue **#31**, reusing the same tools.

## Compatibility

All new fields are additive (`omitempty` / pointer), so sessions and rosters
saved before #23 load unchanged; a character with no spellcasting simply omits
the block.
