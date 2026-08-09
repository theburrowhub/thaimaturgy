# Full creature stat blocks & the embedded SRD (issue #26)

`domain.StatBlock` is a full 5e-style stat block: beyond the core (AC, HP, speed,
abilities, CR, skills, traits, actions) it carries hit dice, XP, proficiency
bonus, size/type/alignment, saving throws, senses, languages, damage
resistances/immunities/vulnerabilities, condition immunities, reactions, and
legendary actions. Every added field is optional, so modules and sessions
authored before #26 load unchanged.

## Auto-fill from the SRD

`internal/srd` embeds a curated subset of common low-CR **SRD 5.1** creatures.

- The DM tool **`lookup_creature`** returns a complete stat block for a standard
  creature by name (`goblin`, `orc`, `skeleton`, `ogre`, …) — the "standard
  creatures get a full sheet automatically" path, and what combat (#22) will lean
  on.
- **`get_npc`** auto-fills: when an NPC has no authored `stat_block` but its name
  matches an SRD creature, the retrieved dossier appends the SRD block, marked as
  auto-filled.

## Custom / non-SRD creatures & overrides

An authored `stat_block` on an NPC is always authoritative — the SRD is only a
fallback, never an override. For a creature not in the SRD subset (or a
customized one), author or improvise the full block on the NPC; every field is
supported by the schema.

## Extending the dataset

The embedded set is intentionally a curated subset (common monsters). Add more by
extending the `creatures` map in `internal/srd/creatures.go`; `Lookup` is
case-insensitive and resolves simple plurals.

## Attribution

The embedded creature statistics are from the **System Reference Document 5.1
("SRD 5.1")** by Wizards of the Coast LLC, available under the
**Creative Commons Attribution 4.0 International License (CC-BY-4.0)**
(https://creativecommons.org/licenses/by/4.0/legalcode). Each embedded block
records `Source: "SRD 5.1 (CC-BY-4.0)"`.
