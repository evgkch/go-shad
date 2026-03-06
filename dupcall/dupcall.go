//go:build !solution

package dupcall

import (
	"context"
	"sync"
)

type Call struct {
	mu     sync.Mutex
	flight *flight
}

type flight struct {
	done    chan struct{}
	res     interface{}
	err     error
	waiters int
	cancel  context.CancelFunc
}

func (o *Call) Do(ctx context.Context, cb func(context.Context) (interface{}, error)) (interface{}, error) {
	o.mu.Lock()
	if o.flight == nil {
		cbCtx, cancel := context.WithCancel(context.Background())
		f := &flight{done: make(chan struct{}), cancel: cancel, waiters: 1}
		o.flight = f
		o.mu.Unlock()

		go func() {
			f.res, f.err = cb(cbCtx)
			close(f.done)
		}()

		return o.wait(ctx, f)
	}

	f := o.flight
	f.waiters++
	o.mu.Unlock()

	return o.wait(ctx, f)
}

func (o *Call) wait(ctx context.Context, f *flight) (interface{}, error) {
	select {
	case <-f.done:
		o.mu.Lock()
		if o.flight == f {
			o.flight = nil
		}
		o.mu.Unlock()
		return f.res, f.err
	case <-ctx.Done():
		o.mu.Lock()
		f.waiters--
		if f.waiters == 0 {
			f.cancel()
			o.flight = nil
		}
		o.mu.Unlock()
		return nil, ctx.Err()
	}
}
