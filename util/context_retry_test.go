package util

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.starlark.net/starlark"
)

func TestOpContext(t *testing.T) {
	// with a timeout, the returned context has a deadline
	thread := &starlark.Thread{}
	ctx, cancel := OpContext(thread, 50*time.Millisecond)
	if _, ok := ctx.Deadline(); !ok {
		t.Error("expected a deadline with timeout > 0")
	}
	cancel()
	if ctx.Err() == nil {
		t.Error("cancel should end the context")
	}

	// without a timeout, still cancellable, no deadline
	ctx2, cancel2 := OpContext(thread, 0)
	if _, ok := ctx2.Deadline(); ok {
		t.Error("expected no deadline with timeout <= 0")
	}
	cancel2()
	if ctx2.Err() == nil {
		t.Error("cancel should end the context")
	}

	// derives from the thread's run context: cancelling the parent cancels ours
	parent, cancelParent := context.WithCancel(context.Background())
	th := &starlark.Thread{}
	th.SetLocal("context", parent)
	ctx3, cancel3 := OpContext(th, 0)
	defer cancel3()
	cancelParent()
	select {
	case <-ctx3.Done():
	case <-time.After(time.Second):
		t.Error("cancelling the thread context should cancel the op context")
	}
}

func TestRetry(t *testing.T) {
	// attempts <= 0 must still call fn exactly once (no fake success / skip)
	for _, n := range []int{0, -1} {
		calls := 0
		err := Retry(context.Background(), n, nil, nil, func() error { calls++; return nil })
		if err != nil || calls != 1 {
			t.Errorf("attempts=%d: calls=%d err=%v, want 1 call, nil err", n, calls, err)
		}
	}

	// a nil context is tolerated (treated as Background)
	calls := 0
	if err := Retry(nil, 2, nil, nil, func() error { calls++; return nil }); err != nil || calls != 1 {
		t.Errorf("nil ctx: calls=%d err=%v, want 1 call, nil", calls, err)
	}

	// all attempts fail -> fn called `attempts` times, last error returned
	calls = 0
	last := errors.New("boom")
	err := Retry(context.Background(), 3, nil, nil, func() error { calls++; return last })
	if calls != 3 || err != last {
		t.Errorf("all-fail: calls=%d err=%v, want 3 calls, last error", calls, err)
	}

	// success on the 2nd attempt stops the loop
	calls = 0
	err = Retry(context.Background(), 5, nil, nil, func() error {
		calls++
		if calls < 2 {
			return errors.New("transient")
		}
		return nil
	})
	if calls != 2 || err != nil {
		t.Errorf("succeed-2nd: calls=%d err=%v, want 2 calls, nil", calls, err)
	}

	// a non-retryable error stops immediately
	calls = 0
	fatal := errors.New("fatal")
	err = Retry(context.Background(), 5, func(e error) bool { return e != fatal }, nil, func() error {
		calls++
		return fatal
	})
	if calls != 1 || err != fatal {
		t.Errorf("non-retryable: calls=%d err=%v, want 1 call, fatal", calls, err)
	}

	// an already-cancelled context returns its error without calling fn
	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls = 0
	err = Retry(cctx, 3, nil, nil, func() error { calls++; return nil })
	if calls != 0 || err == nil {
		t.Errorf("cancelled ctx: calls=%d err=%v, want 0 calls, ctx error", calls, err)
	}

	// a short backoff elapses normally between attempts (the timer-fires path)
	calls = 0
	err = Retry(context.Background(), 3, nil, func(int) time.Duration { return time.Millisecond }, func() error {
		calls++
		if calls < 2 {
			return errors.New("transient")
		}
		return nil
	})
	if calls != 2 || err != nil {
		t.Errorf("short-backoff: calls=%d err=%v, want 2 calls, nil", calls, err)
	}

	// backoff is waited between attempts and is interruptible by ctx
	bctx, bcancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); bcancel() }()
	start := time.Now()
	err = Retry(bctx, 5, nil, func(int) time.Duration { return time.Hour }, func() error {
		return errors.New("retry me")
	})
	if err == nil || time.Since(start) > 2*time.Second {
		t.Errorf("backoff should be interrupted by ctx cancel: err=%v elapsed=%v", err, time.Since(start))
	}
}
