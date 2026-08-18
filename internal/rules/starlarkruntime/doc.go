// Package starlarkruntime loads immutable ZIP rules bundles and adapts their
// deterministic Starlark entrypoints to the rules.Ruleset protocol.
//
// The runtime deliberately exposes no application builtins. Starlark programs
// receive only JSON-shaped request values and the deterministic standard
// language universe; filesystem, network, clock, process, and random access are
// not available. All nondeterministic work must be requested through rules.Step.
package starlarkruntime
