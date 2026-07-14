package util

import (
	"context"
	"time"
)

// Retry runs fn up to attempts times, stopping early on success (nil error), on
// a non-retryable error (retryable returns false), or when ctx ends. It returns
// fn's last error, or ctx.Err() if the context ends first.
//
// attempts < 1 is treated as 1: fn is ALWAYS invoked at least once. This is the
// point of the primitive — a retry count of 0 must not silently skip the call
// and report a fake success (the bug found where retry=0 returned an empty
// result with no error and no request made).
//
// retryable may be nil (every error is retried). backoff may be nil (no delay);
// otherwise backoff(attempt) is waited between attempts, interruptible by ctx.
func Retry(ctx context.Context, attempts int, retryable func(error) bool, backoff func(attempt int) time.Duration, fn func() error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if attempts < 1 {
		attempts = 1
	}
	var err error
	for i := 0; i < attempts; i++ {
		if e := ctx.Err(); e != nil {
			return e
		}
		if err = fn(); err == nil {
			return nil
		}
		if retryable != nil && !retryable(err) {
			return err
		}
		if i < attempts-1 && backoff != nil {
			if d := backoff(i); d > 0 {
				t := time.NewTimer(d)
				select {
				case <-ctx.Done():
					t.Stop()
					return ctx.Err()
				case <-t.C:
				}
			}
		}
	}
	return err
}
