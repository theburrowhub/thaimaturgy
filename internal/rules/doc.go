// Package rules defines the provider-neutral boundary between the game host and
// a loaded rules package.
//
// A Ruleset receives immutable descriptions of the current state and returns a
// Step. It does not roll dice, draw cards, read clocks, perform I/O, or mutate
// host state. Anything requiring entropy, human input, adjudication, child
// resolution, or state changes is represented explicitly by a Step and carried
// out by the host.
//
// This package deliberately contains no concepts from a particular role-playing
// game. System-specific state, action parameters, random specifications, events,
// and outcomes travel as bounded JSON Payload values.
package rules
