package starlarkruntime

import (
	"context"
	"fmt"

	star "go.starlark.net/starlark"
)

func initializePrograms(ctx context.Context, programs map[string]*star.Program, entrypoint string, limits Limits) (star.StringDict, error) {
	var entryGlobals star.StringDict
	err := runStarlark(ctx, "ruleset initialization", limits, func(thread *star.Thread) error {
		initialized := make(map[string]star.StringDict)
		initializing := make(map[string]bool)
		var initialize func(string) (star.StringDict, error)
		initialize = func(module string) (star.StringDict, error) {
			if globals, ok := initialized[module]; ok {
				return globals, nil
			}
			if initializing[module] {
				return nil, fmt.Errorf("%w: load cycle includes %q", ErrInvalidBundle, module)
			}
			program, ok := programs[module]
			if !ok {
				return nil, fmt.Errorf("%w: module %q does not exist", ErrInvalidBundle, module)
			}
			initializing[module] = true
			globals, err := program.Init(thread, nil)
			delete(initializing, module)
			if err != nil {
				return nil, fmt.Errorf("initialize %q: %w", module, err)
			}
			globals.Freeze()
			initialized[module] = globals
			return globals, nil
		}
		thread.Load = func(_ *star.Thread, module string) (star.StringDict, error) {
			if err := validateBundlePath(module); err != nil {
				return nil, fmt.Errorf("%w: module %q: %v", ErrInvalidBundle, module, err)
			}
			return initialize(module)
		}
		var err error
		entryGlobals, err = initialize(entrypoint)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("starlark rules: initialize: %w", err)
	}
	return entryGlobals, nil
}
