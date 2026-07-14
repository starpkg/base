package util

import (
	"strings"
	"testing"
	"time"

	"go.starlark.net/starlark"
)

type stringerType struct{ s string }

func (s stringerType) String() string { return s.s }

func TestToStarlark_Scalars(t *testing.T) {
	no := DecodeLimits{}
	cases := []struct {
		name string
		in   interface{}
		want string // Starlark String() form
	}{
		{"nil", nil, "None"},
		{"bool", true, "True"},
		{"int", 7, "7"},
		{"int64", int64(9223372036854775807), "9223372036854775807"},
		{"uint64", uint64(18446744073709551615), "18446744073709551615"},
		{"float", 1.5, "1.5"},
		{"float32", float32(2.5), "2.5"},
		{"string", "hi", `"hi"`},
		{"int8", int8(-3), "-3"},
		{"uint32", uint32(42), "42"},
		{"stringer", stringerType{"tamed"}, `"tamed"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, err := ToStarlark(c.in, no)
			if err != nil {
				t.Fatalf("ToStarlark(%v): %v", c.in, err)
			}
			if v.String() != c.want {
				t.Errorf("ToStarlark(%v) = %s, want %s", c.in, v.String(), c.want)
			}
		})
	}
}

func TestToStarlark_BytesAndTime(t *testing.T) {
	v, err := ToStarlark([]byte("abc"), DecodeLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(starlark.Bytes); !ok {
		t.Errorf("[]byte should become starlark.Bytes, got %T", v)
	}
	// time.Time is tamed to an RFC 3339 string
	tm := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	v, err = ToStarlark(tm, DecodeLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := starlark.AsString(v); !ok || s != "2020-01-02T03:04:05Z" {
		t.Errorf("time should tame to RFC3339 string, got %s", v.String())
	}
}

func TestToStarlark_Containers(t *testing.T) {
	// nested list + string map, sorted keys
	in := []interface{}{
		map[string]interface{}{"b": 2, "a": 1},
		[]interface{}{true, "x"},
	}
	v, err := ToStarlark(in, DecodeLimits{})
	if err != nil {
		t.Fatal(err)
	}
	l, ok := v.(*starlark.List)
	if !ok || l.Len() != 2 {
		t.Fatalf("expected a list of 2, got %s", v.String())
	}
	if got := l.Index(0).String(); got != `{"a": 1, "b": 2}` {
		t.Errorf("string map should be sorted, got %s", got)
	}

	// TOML-style array-of-tables ([]map[string]interface{})
	aot := []map[string]interface{}{{"k": 1}, {"k": 2}}
	v, err = ToStarlark(aot, DecodeLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if l, ok := v.(*starlark.List); !ok || l.Len() != 2 {
		t.Errorf("array-of-tables should become a 2-list, got %s", v.String())
	}
}

func TestToStarlark_NonStringKeyCollision(t *testing.T) {
	// the real bug: int 1 and string "1" both stringify to "1"; the second must
	// NOT silently overwrite the first — it must error.
	m := map[interface{}]interface{}{1: "int-one", "1": "str-one"}
	if _, err := ToStarlark(m, DecodeLimits{}); err == nil {
		t.Error("colliding non-string map keys should be rejected")
	} else if !strings.Contains(err.Error(), "collide") {
		t.Errorf("unexpected error: %v", err)
	}

	// distinct, non-colliding non-string keys are fine and sorted
	ok := map[interface{}]interface{}{1: "a", 2: "b"}
	v, err := ToStarlark(ok, DecodeLimits{})
	if err != nil {
		t.Fatalf("non-colliding keys should work: %v", err)
	}
	if got := v.String(); got != `{"1": "a", "2": "b"}` {
		t.Errorf("got %s", got)
	}
}

func TestToStarlark_Limits(t *testing.T) {
	// depth limit
	deep := interface{}([]interface{}{[]interface{}{[]interface{}{1}}})
	if _, err := ToStarlark(deep, DecodeLimits{MaxDepth: 2}); err == nil {
		t.Error("exceeding max depth should error")
	}
	if _, err := ToStarlark(deep, DecodeLimits{MaxDepth: 10}); err != nil {
		t.Errorf("within max depth should pass: %v", err)
	}
	// depth counts LEVELS with the root at 1 (matches yaml/toml): a list-in-list
	// is depth 2, so MaxDepth 1 rejects it but admits a bare scalar root.
	if _, err := ToStarlark([]interface{}{[]interface{}{}}, DecodeLimits{MaxDepth: 1}); err == nil {
		t.Error("a nested list at depth 2 should be rejected by MaxDepth 1")
	}
	if _, err := ToStarlark(42, DecodeLimits{MaxDepth: 1}); err != nil {
		t.Errorf("a scalar root should pass MaxDepth 1: %v", err)
	}
	// node limit
	wide := []interface{}{1, 2, 3, 4, 5}
	if _, err := ToStarlark(wide, DecodeLimits{MaxNodes: 3}); err == nil {
		t.Error("exceeding max nodes should error")
	}
	if _, err := ToStarlark(wide, DecodeLimits{MaxNodes: 100}); err != nil {
		t.Errorf("within max nodes should pass: %v", err)
	}
}

func TestToStarlark_TypedNilStringer(t *testing.T) {
	// a typed-nil fmt.Stringer must not panic on String(); it errors instead.
	var tm *time.Time // *time.Time is a fmt.Stringer with a nil receiver
	if _, err := ToStarlark(tm, DecodeLimits{}); err == nil {
		t.Error("a typed-nil Stringer should error, not panic")
	}
}

func TestToStarlark_TimeNanoDropped(t *testing.T) {
	tm := time.Date(2020, 1, 2, 3, 4, 5, 123456789, time.UTC)
	v, err := ToStarlark(tm, DecodeLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := starlark.AsString(v); s != "2020-01-02T03:04:05Z" {
		t.Errorf("sub-second precision should be dropped, got %q", s)
	}
}

func TestToStarlark_CollisionDeterministic(t *testing.T) {
	// A collision must be reported the SAME way regardless of map iteration
	// order, even when the other colliding value would itself hit a cap: detect
	// the collision before converting any value.
	for i := 0; i < 50; i++ {
		m := map[interface{}]interface{}{1: []interface{}{0}, "1": 0}
		_, err := ToStarlark(m, DecodeLimits{MaxNodes: 2})
		if err == nil || !strings.Contains(err.Error(), "collide") {
			t.Fatalf("iteration %d: want a collision error, got %v", i, err)
		}
	}
}

func TestToStarlark_Unsupported(t *testing.T) {
	if _, err := ToStarlark(make(chan int), DecodeLimits{}); err == nil {
		t.Error("an unsupported type should error")
	}
	// an unsupported value nested inside a container propagates the error out
	if _, err := ToStarlark([]interface{}{make(chan int)}, DecodeLimits{}); err == nil {
		t.Error("unsupported value in a list should error")
	}
	if _, err := ToStarlark(map[string]interface{}{"k": make(chan int)}, DecodeLimits{}); err == nil {
		t.Error("unsupported value in a string map should error")
	}
	if _, err := ToStarlark(map[interface{}]interface{}{1: make(chan int)}, DecodeLimits{}); err == nil {
		t.Error("unsupported value in a non-string map should error")
	}
}
