# Built-in reference rulesets

These packages demonstrate the host/package protocol with small, interoperable
resolution primitives. They are deliberately **not** electronic copies of any
published ruleset or compendium.

| Package | Primitive included |
| --- | --- |
| `pf2e` | d20 versus DC, four degrees, natural-face adjustment |
| `runequest` | d100 roll-under with critical, special, failure, and fumble |
| `coc7e` | d100 difficulty levels and bonus/penalty tens dice |
| `vtm5e` | d10 pool, critical pairs, hunger complications |
| `shadowrun6e` | d6 hits, thresholds, glitches |
| `pbta` | 2d6 bands and an optional authored 7-9 choice |
| `gurps4e` | 3d6 roll-under, margin, critical results |
| `fatecore` | 4dF, opposition, shifts, explicit +2 invokes |
| `savageworlds` | exploding trait/Wild dice, target number, raises, snake eyes |

Campaign state, catalogs, character creation, combat engines, spells, powers,
gear, settings, scenarios, prose, and other game-specific content are outside
this reference layer. A distributable rules package can add those through the
same protocol while remaining separately licensed and versioned.

All entropy is requested from the host as `NeedRandom`; continuations and
responses are bounded JSON and are validated again when resumed. PbtA choices
use `NeedDecision`. No package reads a clock, process-global RNG, filesystem,
network, or other ambient capability.
