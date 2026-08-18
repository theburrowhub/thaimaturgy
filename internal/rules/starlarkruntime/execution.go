package starlarkruntime

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"

	star "go.starlark.net/starlark"
)

const (
	stepLimitReason  = "host execution step limit exceeded"
	contextPollSteps = uint64(1024)
)

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
	}
	thread.OnMaxSteps = func(thread *star.Thread) {
		if ctx.Done() != nil {
			// Yield at bounded interpreter intervals. Besides making cancellation
			// responsive on a single busy scheduler, this lets an already-due timer
			// publish ctx.Err before the hard quota wins the same race.
			runtime.Gosched()
			if err := ctx.Err(); err != nil {
				thread.Cancel(err.Error())
				return
			}
		}
		steps := thread.ExecutionSteps()
		if steps >= limits.MaxExecutionSteps {
			stepLimit.Store(true)
			thread.Cancel(stepLimitReason)
			return
		}
		next := steps + contextPollSteps
		if next > limits.MaxExecutionSteps {
			next = limits.MaxExecutionSteps
		}
		thread.SetMaxExecutionSteps(next)
	}
	firstCheckpoint := limits.MaxExecutionSteps
	if ctx.Done() != nil && firstCheckpoint > contextPollSteps {
		firstCheckpoint = contextPollSteps
	}
	thread.SetMaxExecutionSteps(firstCheckpoint)

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
