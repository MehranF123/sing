package task

import (
	"context"
	"sync"

	E "github.com/MehranF123/sing/common/exceptions"
)

func Run(ctx context.Context, tasks ...func() error) error {
	runtimeCtx, cancel := context.WithCancel(ctx)
	wg := &sync.WaitGroup{}
	wg.Add(len(tasks))
	var retErr []error
	for _, task := range tasks {
		currentTask := task
		go func() {
			if err := currentTask(); err != nil {
				retErr = append(retErr, err)
			}
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		cancel()
	}()
	select {
	case <-ctx.Done():
	case <-runtimeCtx.Done():
	}
	retErr = append(retErr, ctx.Err())
	return E.Errors(retErr...)
}

func Any(ctx context.Context, tasks ...func(ctx context.Context) error) error {
	runtimeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var retErr error
	for _, task := range tasks {
		currentTask := task
		go func() {
			if err := currentTask(runtimeCtx); err != nil {
				retErr = err
			}
			cancel()
		}()
	}
	select {
	case <-ctx.Done():
	case <-runtimeCtx.Done():
	}
	return E.Errors(retErr, ctx.Err())
}
