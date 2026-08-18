# Rules packages

thAImaturgy keeps adventure content, executable mechanics, and the language
model in separate trust domains:

```text
adventure.json --requires--> package id + version constraint
                                  |
host catalog --resolves--------> exact id/version/SHA-256/protocol lock
                                  |
session state <----events---- transactional rules host ----game_* tools----> LLM
```

The LLM discovers legal actions through `game_list_actions`, submits typed JSON
with `game_submit_intent`, and narrates the confirmed result. It does not choose
the package, generate random results, edit mechanical state directly, or apply
its own interpretation instead of the loaded package.

## Built-in packages

The repository includes compact resolution primitives for ten systems. They
exercise the complete package protocol but deliberately do not reproduce
compendiums, setting text, character builders, equipment, spells, playbooks, or
other separately distributed game content.

| Package | Included primitive |
| --- | --- |
| `dnd5e@0.1.0` | arbitrary dice expressions and a d20 ability check |
| `pf2e@0.1.0` | d20 check with four degrees and natural-face adjustment |
| `runequest@0.1.0` | d100 roll-under with critical, special and fumble bands |
| `coc7e@0.1.0` | d100 difficulty plus bonus/penalty tens dice |
| `vtm5e@0.1.0` | d10 pools, critical pairs and hunger complications |
| `shadowrun6e@0.1.0` | d6 hits, thresholds and glitches |
| `pbta@0.1.0` | 2d6 result bands and a persistible 7-9 choice |
| `gurps4e@0.1.0` | 3d6 roll-under, margin and critical results |
| `fatecore@0.1.0` | four Fate dice, opposition, shifts and explicit invokes |
| `savageworlds@0.1.0` | exploding trait/Wild dice, raises and snake eyes |

All randomness is requested from the host as `dice.roll`; packages only receive
the audited response. PbtA demonstrates a suspended `need_decision` step. Savage
Worlds demonstrates several sequential random requests in one resolution.

### Immutable built-in releases

A built-in artifact digest covers the package's explicitly declared executable
sources and any behavior-affecting shared helpers (`ruleskit`, `diceexpr`, and
`jsonstrict` where used). It deliberately excludes the host kernel. The
separate `protocol_version` lock identifies the host/package contract, so a
compatible host refactor does not strand existing sessions; an incompatible
contract change requires a new `ProtocolVersion`.

[`builtins.lock.json`](../../internal/rules/runtimecatalog/builtins.lock.json)
is the central append-only release ledger. Startup checks every implemented
built-in against it before publishing the catalog, and tests open every ledger
entry by its exact lock. Never edit or remove a published entry or its matching
implementation. A mechanics or included-helper change requires a new SemVer
package version, a retained implementation for the old release, and a new entry
appended to the ledger.

## Selecting a package

An adventure declares a dependency, not an executable path:

```json
{
  "system": "Fate Core",
  "ruleset": {"id": "fatecore", "version": "^0.1.0"}
}
```

When a new session starts, the host chooses the highest installed compatible
version and persists its complete lock. Resumed sessions use that exact digest;
installing a newer release never upgrades them silently. If the exact artifact
is missing or invalid, mechanical tools remain unavailable until it is restored
or an explicit migration is performed.

The legacy `system` field remains a display label. A closed compatibility map
recognizes the ten historical names when `ruleset` is absent. Unknown labels are
never guessed and an invalid explicit requirement never falls back to the label.

## Installing external packages

External packages are immutable `.rules.zip` files executed by the constrained
Starlark runtime described in [starlark-bundles.md](starlark-bundles.md). Build
the manager, pack a source directory, and install the validated bundle
explicitly:

```bash
make build-rules
./bin/thaimaturgy-rules pack ./my-system ./my-system.rules.zip
./bin/thaimaturgy-rules path
./bin/thaimaturgy-rules install ./my-system.rules.zip
./bin/thaimaturgy-rules list
```

The desktop app, server, bot, and MCP subprocess load an immutable catalog when
their process starts. Restart long-running thAImaturgy processes after an
install so new sessions can resolve the package. Sessions that are already
bound keep their exact ID, version, digest, and protocol lock.

`pack` refuses symlinks, non-regular files, non-portable paths, output inside
the source tree, and inputs beyond the runtime limits. It normalizes ZIP order,
timestamps, modes, and compression, executes the completed bundle through the
production loader, validates its initial state, and only then publishes it
atomically. Repacking unchanged bytes is idempotent; an existing different
output is preserved and reported as a conflict. Run `make example-rules` for a
complete source package at `examples/rules/simple-d6`.

Use `--data-dir PATH` to target a non-default thAImaturgy data directory. The
manager computes SHA-256 over the exact ZIP bytes and stores the verified bundle
under:

```text
<data-dir>/rulesets/<id>/<version>/<sha256>.rules.zip
```

Adventures remain under their existing content store and cannot install or
carry executable rules. Installing a second digest for the same `id@version` is
rejected as release equivocation. Discovery reports malformed packages without
hiding healthy ones.

## Stable tool flow

Every package is exposed through the same seven tools:

1. `game_observe` returns the principal's authorized projection and pending
   responses.
2. `game_list_actions` returns applicable actions and their JSON schemas.
3. `game_get_action_schema` selects one advertised schema.
4. `game_preview` validates only the first step and never draws or commits.
5. `game_submit_intent` starts a transaction and drives host-owned automatic
   steps.
6. `game_respond` resumes an authorized decision or adjudication by its
   persisted resolution ID.
7. `game_explain` resolves a visible package rule reference.

Mutating calls use durable idempotency receipts. Random exchanges, event
batches, pending continuations, the exact lock, principal and request identity
are persisted together with the resulting state. A retry returns the stored
result instead of drawing or reducing again.

## Trust boundary and limits

Built-in Go packages are trusted application code. External Starlark packages
receive JSON values only and have no application, filesystem, process, network,
environment, clock or random APIs. ZIP paths, imports, payloads, collections,
execution steps and compiled-program cache size are bounded. Every response is
decoded strictly and validated again by the kernel.

The Starlark runtime executes in the thAImaturgy process. Its instruction and
data limits substantially reduce accidental or adversarial resource use, but it
is not an operating-system memory boundary. Deployments accepting completely
hostile authors should run packages in a resource-limited worker (or a future
WASM tier) instead of treating in-process Starlark as a complete sandbox.

See [ADR-0001](../adr/0001-ruleset-kernel.md) for the protocol decisions and
`internal/rules/reference` for cross-system conformance fixtures.
