package starlarkruntime

import (
	"context"
	"fmt"
	"sync/atomic"

	star "go.starlark.net/starlark"
)

const stepLimitReason = "host execution step limit exceeded"

func runStarlark(ctx context.Context, name string, limits Limits, execute func(*star.Thread) error) error {
	if ctx == nil {
		return fmt.Errorf("%w: nil context", ErrContract)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var stepLimit atomic.Bool
	thread := &star.Thread{
		Name: name,
		// Suppress print: scripts get no output channel or host side effect.
		Print: func(*star.Thread, string) {},
		OnMaxSteps: func(thread *star.Thread) {
			stepLimit.Store(true)
			thread.Cancel(stepLimitReason)
		},
	}
	thread.SetMaxExecutionSteps(limits.MaxExecutionSteps)

	finished := make(chan struct{})
	if ctx.Done() != nil {
		go func() {
			select {
			case <-ctx.Done():
				thread.Cancel(ctx.Err().Error())
			case <-finished:
			}
		}()
	}
	err := execute(thread)
	close(finished)
	if contextErr := ctx.Err(); contextErr != nil {
		return contextErr
	}
	if stepLimit.Load() {
		return fmt.Errorf("%w: %s used at least %d steps", ErrExecutionLimit, name, limits.MaxExecutionSteps)
	}
	return err
}
