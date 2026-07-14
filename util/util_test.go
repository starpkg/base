package util

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func TestDurationFromSeconds(t *testing.T) {
	cases := []struct {
		name string
		sec  float64
		want time.Duration
	}{
		{"subsecond", 0.5, 500 * time.Millisecond},       // the truncation bug: 0.5 must NOT become 0
		{"fractional", 1.5, 1500 * time.Millisecond},     // must NOT become 1s
		{"whole", 3, 3 * time.Second},                    // ordinary case
		{"tiny", 0.001, time.Millisecond},                // 1ms
		{"zero", 0, 0},                                   //
		{"negative", -0.5, -500 * time.Millisecond},      // caller-owned sign, preserved
		{"nan", math.NaN(), 0},                           // NaN -> 0, not garbage
		{"overflow", 1e30, time.Duration(math.MaxInt64)}, // clamp, don't wrap
		{"neg_overflow", -1e30, time.Duration(math.MinInt64)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DurationFromSeconds(c.sec); got != c.want {
				t.Errorf("DurationFromSeconds(%v) = %v, want %v", c.sec, got, c.want)
			}
		})
	}
}

func TestRecover(t *testing.T) {
	// panic is captured into a nil *err
	got := func() (err error) {
		defer Recover(&err, "op")
		panic("boom")
	}()
	if got == nil || !strings.Contains(got.Error(), "op: panic: boom") {
		t.Errorf("expected wrapped panic, got %v", got)
	}

	// no panic leaves err untouched
	got = func() (err error) {
		defer Recover(&err, "op")
		return nil
	}()
	if got != nil {
		t.Errorf("no panic should leave err nil, got %v", got)
	}

	// an already-set error is not overwritten by a panic-free path
	sentinel := errors.New("original")
	got = func() (err error) {
		defer Recover(&err, "op")
		return sentinel
	}()
	if got != sentinel {
		t.Errorf("existing error must be preserved, got %v", got)
	}
}

func TestSafeCall(t *testing.T) {
	// normal: returns fn's result and error
	v, err := SafeCall("op", func() (int, error) { return 42, nil })
	if v != 42 || err != nil {
		t.Errorf("SafeCall normal = (%d,%v), want (42,nil)", v, err)
	}
	e := errors.New("fn error")
	_, err = SafeCall("op", func() (int, error) { return 0, e })
	if err != e {
		t.Errorf("SafeCall should pass fn error through, got %v", err)
	}
	// panic: converted to error, zero value returned
	v, err = SafeCall("op", func() (int, error) { panic("kaboom") })
	if v != 0 || err == nil || !strings.Contains(err.Error(), "op: panic: kaboom") {
		t.Errorf("SafeCall panic = (%d,%v), want (0, wrapped panic)", v, err)
	}
}

func TestReadAllLimited(t *testing.T) {
	// under the limit
	b, err := ReadAllLimited(strings.NewReader("hello"), 10)
	if err != nil || string(b) != "hello" {
		t.Errorf("under-limit read = (%q,%v)", b, err)
	}
	// exactly at the limit is accepted
	b, err = ReadAllLimited(strings.NewReader("hello"), 5)
	if err != nil || string(b) != "hello" {
		t.Errorf("at-limit read = (%q,%v)", b, err)
	}
	// over the limit errors
	if _, err := ReadAllLimited(strings.NewReader("hello world"), 5); err == nil {
		t.Error("over-limit read should error")
	}
	// unlimited (max <= 0)
	b, err = ReadAllLimited(strings.NewReader("anything"), 0)
	if err != nil || string(b) != "anything" {
		t.Errorf("unlimited read = (%q,%v)", b, err)
	}
	// max == MaxInt64: the +1 would overflow, so it is treated as unlimited (must
	// NOT read empty with a nil error)
	b, err = ReadAllLimited(strings.NewReader("data"), math.MaxInt64)
	if err != nil || string(b) != "data" {
		t.Errorf("MaxInt64 read = (%q,%v), want data", b, err)
	}
	// underlying read error propagates
	if _, err := ReadAllLimited(errReader{}, 100); err == nil {
		t.Error("underlying read error should propagate")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read fail") }

func TestCheckInputSize(t *testing.T) {
	if err := CheckInputSize("body", 5, 10); err != nil {
		t.Errorf("under limit should pass: %v", err)
	}
	if err := CheckInputSize("body", 10, 10); err != nil {
		t.Errorf("at limit should pass: %v", err)
	}
	if err := CheckInputSize("body", 11, 10); err == nil {
		t.Error("over limit should error")
	}
	if err := CheckInputSize("body", 1<<20, 0); err != nil {
		t.Errorf("unlimited should pass: %v", err)
	}
}

func TestCappedWriter(t *testing.T) {
	var buf bytes.Buffer
	cw := NewCappedWriter(&buf, 10)
	if n, err := cw.Write([]byte("hello")); n != 5 || err != nil {
		t.Fatalf("first write = (%d,%v)", n, err)
	}
	if cw.Written() != 5 {
		t.Errorf("Written = %d, want 5", cw.Written())
	}
	// a write that would exceed the limit errors and writes nothing
	if _, err := cw.Write([]byte("world!")); err == nil {
		t.Error("over-limit write should error")
	}
	if buf.String() != "hello" || cw.Written() != 5 {
		t.Errorf("rejected write must not change output: %q / %d", buf.String(), cw.Written())
	}
	// a write that fits is accepted
	if _, err := cw.Write([]byte("world")); err != nil {
		t.Errorf("fitting write should succeed: %v", err)
	}
	if buf.String() != "helloworld" {
		t.Errorf("got %q", buf.String())
	}
	// unlimited
	var buf2 bytes.Buffer
	un := NewCappedWriter(&buf2, 0)
	if _, err := un.Write(bytes.Repeat([]byte("x"), 1000)); err != nil {
		t.Errorf("unlimited write should succeed: %v", err)
	}
}

func TestSecretEqual(t *testing.T) {
	if !SecretEqual([]byte("abc"), []byte("abc")) {
		t.Error("equal bytes should be equal")
	}
	if SecretEqual([]byte("abc"), []byte("abd")) {
		t.Error("different bytes should not be equal")
	}
	if SecretEqual([]byte("abc"), []byte("abcd")) {
		t.Error("different-length should not be equal")
	}
	if !SecretEqualString("s3cr3t", "s3cr3t") {
		t.Error("equal strings should be equal")
	}
	if SecretEqualString("s3cr3t", "wrong") {
		t.Error("different strings should not be equal")
	}
}
