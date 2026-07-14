package util

import (
	"fmt"
	"reflect"
	"sort"
	"time"

	"go.starlark.net/starlark"
)

// DecodeLimits bounds the work a ToStarlark conversion may do, fencing a huge or
// malicious decoded document. A zero field means that dimension is unbounded.
type DecodeLimits struct {
	MaxDepth int // maximum nesting depth
	MaxNodes int // maximum total number of values materialized
}

// ToStarlark converts a Go value — typically the output of a decoder such as
// yaml/toml/json — into a Starlark value, bounded by lim. It centralizes the
// hardened "capwalk" every starpkg codec re-implemented (each subtly different):
//
//   - caps nesting depth and total node count from lim;
//   - materializes maps in deterministic (sorted-key) order;
//   - tames a time.Time (RFC 3339, so sub-second precision is discarded) and any
//     other fmt.Stringer (e.g. a TOML local date) to a string, so an opaque Go
//     type never leaks into a script;
//   - REJECTS a map whose distinct keys collide after stringification (e.g. int
//     1 and string "1") instead of silently dropping one — a real bug the
//     hand-rolled decoders shared.
func ToStarlark(v interface{}, lim DecodeLimits) (starlark.Value, error) {
	nodes := 0
	// The root value is at depth 1, matching the yaml/toml decoders this
	// generalizes, so MaxDepth counts nesting LEVELS (a MaxDepth of 1 admits only
	// a scalar root).
	return toStarlark(v, 1, &nodes, lim)
}

func toStarlark(v interface{}, depth int, nodes *int, lim DecodeLimits) (starlark.Value, error) {
	if lim.MaxDepth > 0 && depth > lim.MaxDepth {
		return nil, fmt.Errorf("decode: nesting exceeds max depth (%d)", lim.MaxDepth)
	}
	*nodes++
	if lim.MaxNodes > 0 && *nodes > lim.MaxNodes {
		return nil, fmt.Errorf("decode: node count exceeds max nodes (%d)", lim.MaxNodes)
	}
	if sv, ok := simpleToStarlark(v); ok {
		return sv, nil
	}
	// Tame any fmt.Stringer (e.g. a TOML local date/time) to a string — but not a
	// typed-nil one, whose String() would dereference a nil pointer and panic;
	// let that fall through to the unsupported-type error instead.
	if s, ok := v.(fmt.Stringer); ok && !isNilValue(v) {
		return starlark.String(s.String()), nil
	}
	return containerToStarlark(v, depth, nodes, lim)
}

// isNilValue reports whether v holds a nil pointer/map/slice/etc. — a value whose
// methods would panic on a nil receiver.
func isNilValue(v interface{}) bool {
	switch rv := reflect.ValueOf(v); rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		return rv.IsNil()
	}
	return false
}

// simpleToStarlark converts a scalar (or a numeric of any kind); ok is false for
// a container or an unsupported type.
func simpleToStarlark(v interface{}) (starlark.Value, bool) {
	switch x := v.(type) {
	case nil:
		return starlark.None, true
	case bool:
		return starlark.Bool(x), true
	case string:
		return starlark.String(x), true
	case []byte:
		return starlark.Bytes(x), true
	case time.Time:
		return starlark.String(x.Format(time.RFC3339)), true
	}
	return numberToStarlark(v)
}

// numberToStarlark handles every Go numeric kind a decoder might emit (int,
// int8..int64, uint..uint64/uintptr, float32/float64) via reflect.
func numberToStarlark(v interface{}) (starlark.Value, bool) {
	switch rv := reflect.ValueOf(v); rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return starlark.MakeInt64(rv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return starlark.MakeUint64(rv.Uint()), true
	case reflect.Float32, reflect.Float64:
		return starlark.Float(rv.Float()), true
	}
	return nil, false
}

// containerToStarlark converts a list/map, or returns an unsupported-type error.
func containerToStarlark(v interface{}, depth int, nodes *int, lim DecodeLimits) (starlark.Value, error) {
	switch x := v.(type) {
	case []interface{}:
		return listToStarlark(x, depth, nodes, lim)
	case []map[string]interface{}: // e.g. a TOML array-of-tables
		elems := make([]interface{}, len(x))
		for i := range x {
			elems[i] = x[i]
		}
		return listToStarlark(elems, depth, nodes, lim)
	case map[string]interface{}:
		return stringMapToStarlark(x, depth, nodes, lim)
	case map[interface{}]interface{}:
		return anyMapToStarlark(x, depth, nodes, lim)
	}
	return nil, fmt.Errorf("decode: unsupported value of type %T", v)
}

func listToStarlark(x []interface{}, depth int, nodes *int, lim DecodeLimits) (starlark.Value, error) {
	elems := make([]starlark.Value, 0, len(x))
	for _, e := range x {
		sv, err := toStarlark(e, depth+1, nodes, lim)
		if err != nil {
			return nil, err
		}
		elems = append(elems, sv)
	}
	return starlark.NewList(elems), nil
}

func stringMapToStarlark(x map[string]interface{}, depth int, nodes *int, lim DecodeLimits) (starlark.Value, error) {
	keys := make([]string, 0, len(x))
	for k := range x {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	d := starlark.NewDict(len(keys))
	for _, k := range keys {
		sv, err := toStarlark(x[k], depth+1, nodes, lim)
		if err != nil {
			return nil, err
		}
		if err := d.SetKey(starlark.String(k), sv); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func anyMapToStarlark(x map[interface{}]interface{}, depth int, nodes *int, lim DecodeLimits) (starlark.Value, error) {
	type kv struct {
		key string
		val interface{}
	}
	items := make([]kv, 0, len(x))
	for k, val := range x {
		items = append(items, kv{fmt.Sprintf("%v", k), val})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].key < items[j].key })
	// Detect key collisions FIRST, before converting any value, so the collision
	// error is returned deterministically rather than racing a value conversion
	// that might itself hit the depth/node cap.
	seen := make(map[string]struct{}, len(items))
	for _, it := range items {
		if _, dup := seen[it.key]; dup {
			return nil, fmt.Errorf("decode: distinct map keys collide as %q after stringification", it.key)
		}
		seen[it.key] = struct{}{}
	}
	d := starlark.NewDict(len(items))
	for _, it := range items {
		sv, err := toStarlark(it.val, depth+1, nodes, lim)
		if err != nil {
			return nil, err
		}
		if err := d.SetKey(starlark.String(it.key), sv); err != nil {
			return nil, err
		}
	}
	return d, nil
}
