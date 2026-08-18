package starlarkruntime

import "errors"

var (
	// ErrInvalidBundle means an archive does not satisfy the bundle format or
	// confinement rules.
	ErrInvalidBundle = errors.New("starlark rules: invalid bundle")
	// ErrBundleTooLarge means compressed or expanded bundle data exceeds a
	// configured resource limit.
	ErrBundleTooLarge = errors.New("starlark rules: bundle too large")
	// ErrContract means the program does not implement the Starlark ruleset
	// contract or returned a value outside its JSON-neutral boundary.
	ErrContract = errors.New("starlark rules: contract violation")
	// ErrExecutionLimit means Starlark evaluation consumed its step quota.
	ErrExecutionLimit = errors.New("starlark rules: execution step limit exceeded")
)
