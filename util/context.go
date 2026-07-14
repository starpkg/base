package util

import (
	"context"
	"time"

	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
)

// OpContext returns a context (and its cancel func) for a single host operation
// triggered from a script. It starts from the thread's run context — so the
// Machine's own cancellation/deadline (e.g. RunWithTimeout) propagates to the
// operation — and layers an additional host timeout when timeout > 0. The
// returned context is always cancellable and derived; a timeout <= 0 adds no
// extra deadline. The caller MUST call the returned cancel func (typically via
// defer) to release resources.
//
// This closes the recurring gap where a blocking remote call (a DB query, an
// API request, a queue send) took no context and could hang the host until the
// vendor SDK's or the OS's TCP timeout.
func OpContext(thread *starlark.Thread, timeout time.Duration) (context.Context, context.CancelFunc) {
	parent := dataconv.GetThreadContext(thread) // never nil: falls back to Background
	if timeout > 0 {
		return context.WithTimeout(parent, timeout)
	}
	return context.WithCancel(parent)
}
