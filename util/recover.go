package util

import "fmt"

// Recover, used in a deferred call, converts a panic into an error stored in
// *errp (only when *errp is still nil), prefixed with label. It lets a wrapper
// around a panicky third-party call return an error instead of crashing the
// host — the recurring "defer/recover around a 3rd-party parser/renderer"
// pattern. Use as: defer util.Recover(&err, "yaml decode").
func Recover(errp *error, label string) {
	if r := recover(); r != nil && errp != nil && *errp == nil {
		*errp = fmt.Errorf("%s: panic: %v", label, r)
	}
}

// SafeCall runs fn and converts any panic into an error prefixed with label, so
// a panicky third-party call cannot crash the host. It returns fn's own result
// and error unchanged when fn does not panic.
func SafeCall[T any](label string, fn func() (T, error)) (result T, err error) {
	defer Recover(&err, label)
	return fn()
}
