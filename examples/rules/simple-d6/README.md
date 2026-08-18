# Simple d6 rules package

This deliberately small Starlark package demonstrates the complete external
rules contract with durable state:

```text
Start -> NeedRandom(dice.roll) -> Resume -> Emit -> Reduce/commit
      -> Resume -> Complete
```

It has one action, `simple_d6.check`: roll 1d6, add an optional modifier, and
compare the total with a target. Each committed check increments `attempts` and
stores its result in `last`. The reducer is pure, so the same event log can
reconstruct the state after a restart.

From the repository root:

```bash
make build-rules
./bin/thaimaturgy-rules pack examples/rules/simple-d6 dist/rules/simple-d6.rules.zip
./bin/thaimaturgy-rules install dist/rules/simple-d6.rules.zip
```

The host, rather than the script, supplies and audits the `dice.roll` result.
