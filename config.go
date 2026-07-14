package base

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
)

// ConfigGetter is a function that returns a configuration value of type T.
type ConfigGetter[T any] func() T

// ConfigValidator is a function that validates a configuration value of type T.
type ConfigValidator[T any] func(value T) error

// ConfigOption represents a configuration option with a specific type.
// It supports default values, validation, dynamic getters, and environment variable overrides.
// The internal mutex protects all mutable fields.
//
// Configuration values are resolved in the following priority order (from highest to lowest):
// 1. Immediate value (set via WithValue/SetValue)
// 2. Returned value from the getter function (set via WithGetter)
// 3. Environment variable value (set via WithEnvVar)
// 4. Default value (set via WithDefault or NewConfigOption)
//
// Secret configuration values are accessible in Go code but will not be exposed
// to Starlark runtime or in the GetInfo() results to protect sensitive data.
type ConfigOption[T any] struct {
	// Configuration metadata
	Name        string // Unique identifier for this configuration.
	Description string // Human-readable description of the configuration.
	EnvVar      string // Environment variable name for overriding the configuration.

	// Internal fields
	mu         sync.RWMutex
	defaultVal T
	value      T
	hasValue   bool
	getter     ConfigGetter[T]
	validator  ConfigValidator[T]
	isRequired bool
	isSecret   bool
	isHostOnly bool
}

// NewConfigOption creates a new configuration option with the given default value.
func NewConfigOption[T any](defaultValue T) *ConfigOption[T] {
	return &ConfigOption[T]{
		defaultVal: defaultValue,
	}
}

// NewNamedConfigOption creates a configuration option with its name, description,
// and the conventional `<MODULE>_<NAME>` environment variable (both uppercased)
// filled in. It collapses the per-module `genConfigOption` helper every domain
// module hand-rolls — e.g. NewNamedConfigOption("yaml", "max_depth", "...", 64)
// derives the env var `YAML_MAX_DEPTH`.
func NewNamedConfigOption[T any](module, name, description string, defaultValue T) *ConfigOption[T] {
	return NewConfigOption(defaultValue).
		WithName(name).
		WithDescription(description).
		WithEnvVar(strings.ToUpper(module) + "_" + strings.ToUpper(name))
}

//////////////////////////////////////////////////////////////////////////
// Builder Methods
//////////////////////////////////////////////////////////////////////////

// WithName sets the name of the configuration option.
func (o *ConfigOption[T]) WithName(name string) *ConfigOption[T] {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.Name = name
	return o
}

// WithDescription adds a description to the configuration option.
func (o *ConfigOption[T]) WithDescription(desc string) *ConfigOption[T] {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.Description = desc
	return o
}

// WithValue sets the value of the configuration option.
// This is useful for chain calls when building a configuration option.
// Unlike SetValue, this method ignores any validators since it's part of a builder chain.
// Validation will occur during module initialization when Initialize() is called.
// This has the highest priority in the resolution order.
func (o *ConfigOption[T]) WithValue(value T) *ConfigOption[T] {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.value = value
	o.hasValue = true
	return o
}

// WithGetter adds a custom getter to the configuration option.
// This has the second highest priority in the resolution order.
func (o *ConfigOption[T]) WithGetter(getter ConfigGetter[T]) *ConfigOption[T] {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.getter = getter
	return o
}

// WithEnvVar specifies an environment variable name to check for this configuration.
// This has the third highest priority in the resolution order.
func (o *ConfigOption[T]) WithEnvVar(envVar string) *ConfigOption[T] {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.EnvVar = envVar
	return o
}

// WithDefault sets the default value of the configuration option.
// This has the lowest priority in the resolution order.
func (o *ConfigOption[T]) WithDefault(defaultValue T) *ConfigOption[T] {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.defaultVal = defaultValue
	return o
}

// WithValidator adds a validator to the configuration option.
func (o *ConfigOption[T]) WithValidator(validator ConfigValidator[T]) *ConfigOption[T] {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.validator = validator
	return o
}

// SetRequired sets whether the configuration option is required.
func (o *ConfigOption[T]) SetRequired(required bool) *ConfigOption[T] {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.isRequired = required
	return o
}

// SetSecret sets whether the configuration option is secret.
func (o *ConfigOption[T]) SetSecret(secret bool) *ConfigOption[T] {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.isSecret = secret
	return o
}

// SetHostOnly sets whether the configuration option is host-only. A host-only
// option is NOT exposed to Starlark as a set_<name> builtin, so a script cannot
// change it; the host still configures it in Go and (unless it is also secret)
// a get_<name> builtin still lets a script read it. This is for safety- or
// resource-sensitive limits — e.g. a maximum input size — that a module must be
// able to enforce against untrusted scripts, which a script-settable option
// cannot do (it could just raise or zero the limit). Defaults to false, so
// existing options remain script-settable exactly as before.
func (o *ConfigOption[T]) SetHostOnly(hostOnly bool) *ConfigOption[T] {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.isHostOnly = hostOnly
	return o
}

//////////////////////////////////////////////////////////////////////////
// Accessors and Mutators
//////////////////////////////////////////////////////////////////////////

// GetValue returns the current value of the configuration option.
// The value is resolved according to the following priority order (from highest to lowest):
// 1. Immediate value (set via WithValue/SetValue)
// 2. Returned value from the getter function (set via WithGetter)
// 3. Environment variable value (set via WithEnvVar)
// 4. Default value (set via WithDefault or NewConfigOption)
//
// Secret values can be accessed via this method in Go code.
func (o *ConfigOption[T]) GetValue() (T, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.resolveValue()
}

// SetValue sets the value of the configuration option.
// It validates the value if a validator is set.
func (o *ConfigOption[T]) SetValue(value T) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.validator != nil {
		if err := o.validator(value); err != nil {
			return fmt.Errorf("%w: %v", ErrConfigInvalidValue, err)
		}
	}

	o.value = value
	o.hasValue = true
	return nil
}

// Validate validates the current value if a validator is set.
// This method ONLY validates the explicitly set value (via SetValue or WithValue),
// and does NOT validate values returned from a getter function or environment, or default values.
// Returns nil if no validator is set, no value has been set, or the validation passes.
func (o *ConfigOption[T]) Validate() error {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.validator == nil || !o.hasValue {
		return nil
	}

	if err := o.validator(o.value); err != nil {
		return fmt.Errorf("%w: %v", ErrConfigInvalidValue, err)
	}

	return nil
}

// ConfigOptionInterface implementation

// GetName returns the name of the configuration option.
func (o *ConfigOption[T]) GetName() string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.Name
}

// SetName sets the name of the configuration option.
func (o *ConfigOption[T]) SetName(name string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.Name = name
}

// IsRequired returns whether the configuration option is required.
func (o *ConfigOption[T]) IsRequired() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.isRequired
}

// IsSecret returns whether the configuration option is secret.
func (o *ConfigOption[T]) IsSecret() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.isSecret
}

// IsHostOnly returns whether the configuration option is host-only (not exposed
// to Starlark as a set_<name> builtin). See SetHostOnly.
func (o *ConfigOption[T]) IsHostOnly() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.isHostOnly
}

// HasValue returns whether the configuration option has a value set.
func (o *ConfigOption[T]) HasValue() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.hasValue
}

// HasGetter returns whether the configuration option has a getter.
func (o *ConfigOption[T]) HasGetter() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.getter != nil
}

// HasDefault returns true if the default value is not the zero value.
func (o *ConfigOption[T]) HasDefault() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	var zero T
	return !reflect.DeepEqual(o.defaultVal, zero)
}

// HasEnvVar returns whether the configuration option has an environment variable specified.
func (o *ConfigOption[T]) HasEnvVar() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.EnvVar != ""
}

// GetInfo returns information about the configuration option.
func (o *ConfigOption[T]) GetInfo() map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	info := map[string]interface{}{
		"name":        o.Name,
		"description": o.Description,
		"env_var":     o.EnvVar,
		"required":    o.isRequired,
		"secret":      o.isSecret,
		"host_only":   o.isHostOnly,
		"has_value":   o.hasValue,
		"has_getter":  o.getter != nil,
		"has_env_var": o.EnvVar != "",
	}

	// Only include values for non-secret configs in the info map
	// This protects secrets from being accidentally logged or displayed
	if !o.isSecret {
		if val, err := o.resolveValue(); err == nil {
			info["value"] = val
		}
	}

	return info
}

//////////////////////////////////////////////////////////////////////////
// Starlark Integration
//////////////////////////////////////////////////////////////////////////

// SetValueFromStarlark sets the configuration option from a Starlark value.
//
// It never lets a reflection/conversion panic escape to the host: a recover
// guard (mirroring resolveValue) turns any panic into an error, and None/nil
// input is rejected up front (dataconv.Unmarshal(None) yields a nil interface,
// whose reflect.TypeOf is a nil Type that would panic on Kind()).
func (o *ConfigOption[T]) SetValueFromStarlark(v starlark.Value) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v", ErrConfigInvalidValue, r)
		}
	}()

	gv, err := dataconv.Unmarshal(v)
	if err != nil {
		return err
	}
	if gv == nil {
		var zero T
		return fmt.Errorf("%w: cannot set %q (%T) from None/nil", ErrConfigInvalidValue, o.Name, zero)
	}

	targetType := reflect.TypeOf((*T)(nil)).Elem()
	sourceValue := reflect.ValueOf(gv)
	sourceType := reflect.TypeOf(gv)

	// Handle slices/arrays
	if targetType.Kind() == reflect.Slice && sourceType.Kind() == reflect.Slice {
		destSlice := reflect.MakeSlice(targetType, sourceValue.Len(), sourceValue.Len())
		elemType := targetType.Elem()

		for i := 0; i < sourceValue.Len(); i++ {
			srcElem := sourceValue.Index(i).Interface()
			srcElemType := reflect.TypeOf(srcElem)

			// Try to convert numeric types (checked: reject silent narrowing/overflow)
			if isNumericType(elemType) && isNumericType(srcElemType) {
				cv, cerr := checkedConvert(reflect.ValueOf(srcElem), elemType)
				if cerr != nil {
					return fmt.Errorf("element at index %d: %v", i, cerr)
				}
				destSlice.Index(i).Set(cv)
				continue
			}

			// Try direct conversion (checked: reject int->string code-point casts)
			cv, cerr := checkedElemConvert(reflect.ValueOf(srcElem), elemType)
			if cerr != nil {
				return fmt.Errorf("element at index %d cannot be converted from %v to %v: %v", i, srcElemType, elemType, cerr)
			}
			destSlice.Index(i).Set(cv)
		}

		return o.SetValue(destSlice.Interface().(T))
	}

	// Handle maps
	if targetType.Kind() == reflect.Map && sourceType.Kind() == reflect.Map {
		destMap := reflect.MakeMap(targetType)
		keyType := targetType.Key()
		valueType := targetType.Elem()

		iter := sourceValue.MapRange()
		for iter.Next() {
			srcKey := iter.Key().Interface()
			srcValue := iter.Value().Interface()
			srcKeyType := reflect.TypeOf(srcKey)
			srcValueType := reflect.TypeOf(srcValue)

			// dataconv.Unmarshal renders every dict key as its decimal string
			// (e.g. "1", "1.5"), so a numeric target key arrives as a string:
			// parse it to a float64, then convert CHECKED — the same bar as map
			// values and slice elements. A fractional key into an integer target
			// (1.5 -> int) or an out-of-range key (256 -> uint8) is rejected, not
			// silently truncated/wrapped. (Float precision loss into a float
			// target is accepted, matching the ecosystem's checked-conversion
			// invariant, which errors on overflow/narrowing/code-point only.)
			var destKey reflect.Value
			if isNumericType(keyType) {
				kv, ok := numericKeyValue(srcKey, keyType)
				if !ok || !kv.Type().ConvertibleTo(keyType) {
					// non-numeric key, or a numeric key whose type cannot reach the
					// target at all (e.g. a real key into a complex target)
					return fmt.Errorf("map key cannot be converted from %v to %v", srcKeyType, keyType)
				}
				ck, cerr := checkedConvert(kv, keyType)
				if cerr != nil {
					return fmt.Errorf("map key %v cannot be converted to %v: %v", kv.Interface(), keyType, cerr)
				}
				destKey = ck
			} else {
				// checked: reject int->string code-point casts (keys arrive as
				// decimal strings from dataconv, so this normally converts
				// string->string, but guard the general case)
				ck, cerr := checkedElemConvert(reflect.ValueOf(srcKey), keyType)
				if cerr != nil {
					return fmt.Errorf("map key cannot be converted from %v to %v: %v", srcKeyType, keyType, cerr)
				}
				destKey = ck
			}

			// Try to convert numeric types for values (checked: reject silent narrowing/overflow)
			var destValue reflect.Value
			if isNumericType(valueType) && isNumericType(srcValueType) {
				cv, cerr := checkedConvert(reflect.ValueOf(srcValue), valueType)
				if cerr != nil {
					return fmt.Errorf("map value: %v", cerr)
				}
				destValue = cv
			} else {
				// checked: reject int->string code-point casts
				cvv, cerr := checkedElemConvert(reflect.ValueOf(srcValue), valueType)
				if cerr != nil {
					return fmt.Errorf("map value cannot be converted from %v to %v: %v", srcValueType, valueType, cerr)
				}
				destValue = cvv
			}

			// Two distinct source keys can convert to the same destination key
			// (e.g. int 1 and float 1.0, or "1" and "1.0"); silently overwriting
			// would drop an entry, so reject the collision instead.
			if destMap.MapIndex(destKey).IsValid() {
				return fmt.Errorf("map key %v collides with an earlier key after conversion to %v", destKey, keyType)
			}
			destMap.SetMapIndex(destKey, destValue)
		}

		return o.SetValue(destMap.Interface().(T))
	}

	// Handle simple types (checked: reject silent narrowing/overflow/NaN/Inf)
	if isNumericType(targetType) && isNumericType(sourceType) {
		cv, cerr := checkedConvert(sourceValue, targetType)
		if cerr != nil {
			return fmt.Errorf("%w: cannot set %q: %v", ErrConfigInvalidValue, o.Name, cerr)
		}
		return o.SetValue(cv.Interface().(T))
	}

	// Try direct type assertion
	vt, ok := gv.(T)
	if !ok {
		var zero T
		return fmt.Errorf("value type mismatch, expected %T, got %T", zero, gv)
	}

	return o.SetValue(vt)
}

// isNumericType returns true if the type is a numeric type.
func isNumericType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return true
	}
	return false
}

// isIntegerKind reports whether k is a Go integer kind. Go permits a bare
// reflect.Convert of an integer to a string, which it performs as a Unicode
// code-point cast (int(65) -> "A"), not a decimal rendering. Uintptr is an
// integer kind and converts the same way, so it must be listed too.
func isIntegerKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	}
	return false
}

// checkedElemConvert converts src to destType for a slice element or a map
// key/value, but — unlike a bare reflect.Convert — REJECTS an integer→string
// conversion. Go performs that as a silent Unicode code-point cast (int(65) ->
// "A"), so a script that set, say, a []string allowlist from a list of integers
// would get code-point strings instead of a loud error. This is the collection
// analog of the ecosystem "int->string must be explicit" invariant (the numeric
// narrowing case is handled by checkedConvert). Other convertible pairs pass
// through unchanged.
func checkedElemConvert(src reflect.Value, destType reflect.Type) (reflect.Value, error) {
	if destType.Kind() == reflect.String && isIntegerKind(src.Kind()) {
		return reflect.Value{}, fmt.Errorf("refusing to convert integer %v to string as a Unicode code point; provide a string instead", src.Interface())
	}
	if !src.Type().ConvertibleTo(destType) {
		return reflect.Value{}, fmt.Errorf("cannot convert %v to %v", src.Type(), destType)
	}
	return src.Convert(destType), nil
}

// checkedConvert converts val to type t, but — unlike a bare reflect.Convert —
// REJECTS a numeric conversion that would silently corrupt the value: integer
// narrowing that overflows the target, a negative value into an unsigned
// target, NaN/Inf or a non-integral float into an integer, and a float that
// overflows the target float. This is the base analog of starlight's
// convert.checkedConvert (the ecosystem "checked conversions" invariant), so a
// config value set from a script (or env) can never be silently truncated or
// wrapped. Non-numeric conversions pass through unchanged.
//
// Sources here always come from dataconv.Unmarshal, which yields a signed Go
// int / int64 or a float64 for a Starlark number (never an unsigned Go int), so
// only signed-int and float source kinds are range-checked.
func checkedConvert(val reflect.Value, t reflect.Type) (reflect.Value, error) {
	if val.Type().AssignableTo(t) {
		return val, nil
	}
	if !val.Type().ConvertibleTo(t) {
		return reflect.Value{}, fmt.Errorf("value of type %s cannot be converted to type %s", val.Type(), t)
	}
	if err := checkNumericRange(val, t); err != nil {
		return reflect.Value{}, err
	}
	return val.Convert(t), nil
}

// checkNumericRange returns an error if converting val to the numeric type t
// would lose information; it is a no-op for non-numeric targets.
func checkNumericRange(val reflect.Value, t reflect.Type) error {
	zt := reflect.Zero(t)
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return checkToInt(val, zt, t)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return checkToUint(val, zt, t)
	case reflect.Float32, reflect.Float64:
		if isFloatKind(val.Kind()) && zt.OverflowFloat(val.Float()) {
			return fmt.Errorf("value %v out of range for type %s", val.Float(), t)
		}
	}
	return nil
}

func checkToInt(val, zt reflect.Value, t reflect.Type) error {
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if zt.OverflowInt(val.Int()) {
			return fmt.Errorf("value %d out of range for type %s", val.Int(), t)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if u := val.Uint(); u > math.MaxInt64 || zt.OverflowInt(int64(u)) {
			return fmt.Errorf("value %d out of range for type %s", u, t)
		}
	case reflect.Float32, reflect.Float64:
		return checkFloatToInt(val.Float(), zt, t)
	}
	return nil
}

func checkToUint(val, zt reflect.Value, t reflect.Type) error {
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i := val.Int()
		if i < 0 || zt.OverflowUint(uint64(i)) {
			return fmt.Errorf("value %d out of range for type %s", i, t)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if u := val.Uint(); zt.OverflowUint(u) {
			return fmt.Errorf("value %d out of range for type %s", u, t)
		}
	case reflect.Float32, reflect.Float64:
		return checkFloatToUint(val.Float(), zt, t)
	}
	return nil
}

func checkFloatToInt(f float64, zt reflect.Value, t reflect.Type) error {
	if err := floatIntegral(f, t); err != nil {
		return err
	}
	if f < math.MinInt64 || f >= math.MaxInt64 || zt.OverflowInt(int64(f)) {
		return fmt.Errorf("value %v out of range for type %s", f, t)
	}
	return nil
}

func checkFloatToUint(f float64, zt reflect.Value, t reflect.Type) error {
	if err := floatIntegral(f, t); err != nil {
		return err
	}
	if f < 0 || f >= math.MaxUint64 || zt.OverflowUint(uint64(f)) {
		return fmt.Errorf("value %v out of range for type %s", f, t)
	}
	return nil
}

// floatIntegral rejects a float that cannot become an integer without loss
// (NaN, Inf, or a fractional value).
func floatIntegral(f float64, t reflect.Type) error {
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) {
		return fmt.Errorf("value %v would be truncated when converted to type %s", f, t)
	}
	return nil
}

func isFloatKind(k reflect.Kind) bool {
	return k == reflect.Float32 || k == reflect.Float64
}

// numericKeyValue parses a map key into the most precise Go numeric value for a
// numeric target key type, WITHOUT a float64 round-trip. dataconv.Unmarshal
// renders dict keys as decimal strings, so the common case is a string parse: an
// integer string parses to int64/uint64 EXACTLY (a float64 bridge would corrupt
// a key beyond float64's 53-bit exact-integer range, e.g. 2^53+1), and only a
// genuinely fractional key falls back to float64. Already-numeric keys (from a
// host-wrapped Go map) are used as-is. The caller then range/integrality-checks
// via checkedConvert.
func numericKeyValue(k interface{}, keyType reflect.Type) (reflect.Value, bool) {
	if s, ok := k.(string); ok {
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return reflect.ValueOf(i), true
		}
		if u, err := strconv.ParseUint(s, 10, 64); err == nil {
			return reflect.ValueOf(u), true
		}
		// An integer-formatted key outside [MinInt64, MaxUint64] cannot be
		// represented exactly by any 64-bit integer target; float-parsing it
		// would ROUND it and silently change the key's identity (e.g. MinInt64-1
		// -> MinInt64). Reject it for an integer target — only a float target
		// accepts it, as ordinary float precision loss, and only a genuinely
		// fractional/exponent key floats.
		if !strings.ContainsAny(s, ".eE") && !isFloatKind(keyType.Kind()) {
			return reflect.Value{}, false
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return reflect.ValueOf(f), true
		}
		return reflect.Value{}, false
	}
	rv := reflect.ValueOf(k)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return rv, true
	}
	return reflect.Value{}, false
}

// GetStarlarkValue returns the configuration value as a starlark value.
// This method provides the underlying mechanism to access configuration values in Starlark scripts.
// Note that while this method itself doesn't block secret values, secret configuration options
// are not exposed as get_* methods in Starlark runtime by the LoadModule method.
func (o *ConfigOption[T]) GetStarlarkValue() (starlark.Value, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	value, err := o.resolveValue()
	if err != nil {
		return nil, err
	}
	return dataconv.Marshal(value)
}

//////////////////////////////////////////////////////////////////////////
// Internal Methods
//////////////////////////////////////////////////////////////////////////

// resolveValue returns the current value based on the priority order:
// PRIORITY ORDER (from highest to lowest):
// 1. Immediate value (set via WithValue/SetValue)
// 2. Returned value from the getter function (set via WithGetter)
// 3. Environment variable value (set via WithEnvVar)
// 4. Default value (set via WithDefault or NewConfigOption)
//
// This function may return an error if:
// - The getter function panics
// - Reflection operations fail
func (o *ConfigOption[T]) resolveValue() (value T, err error) {
	// Use defer/recover to handle panics
	defer func() {
		if r := recover(); r != nil {
			var zero T
			value = zero
			err = fmt.Errorf("%w: %v", ErrConfigGetterPanic, r)
		}
	}()

	// Priority 1 (Highest): Immediate value takes precedence
	if o.hasValue {
		return o.value, nil
	}

	// Priority 2: Getter provides dynamic values and takes precedence over environment variables
	if o.getter != nil {
		return o.getter(), nil
	}

	// Priority 3: Environment variable takes precedence over default value
	if o.EnvVar != "" {
		if envValue, exists := os.LookupEnv(o.EnvVar); exists {
			if converted, ok := o.convertEnvValue(envValue); ok {
				return converted, nil
			}
		}
	}

	// Priority 4 (Lowest): Default value is used as a fallback
	return o.defaultVal, nil
}

// convertEnvValue attempts to convert an environment variable string value
// to the target type T using JSON decoding for complex types.
func (o *ConfigOption[T]) convertEnvValue(envValue string) (T, bool) {
	var zero T
	targetType := reflect.TypeOf(zero)

	// Handle string types directly
	if targetType.Kind() == reflect.String {
		return reflect.ValueOf(envValue).Convert(targetType).Interface().(T), true
	}

	// Handle boolean values with common formats
	if targetType.Kind() == reflect.Bool {
		lowerVal := strings.ToLower(envValue)
		var boolValue bool
		switch lowerVal {
		case "true", "yes", "1", "on":
			boolValue = true
		case "false", "no", "0", "off":
			boolValue = false
		default:
			return zero, false
		}
		return reflect.ValueOf(boolValue).Convert(targetType).Interface().(T), true
	}

	// Handle numeric types
	if isNumericType(targetType) {
		switch targetType.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			intVal, err := strconv.ParseInt(envValue, 10, 64)
			if err != nil {
				return zero, false
			}
			cv, cerr := checkedConvert(reflect.ValueOf(intVal), targetType)
			if cerr != nil {
				return zero, false
			}
			return cv.Interface().(T), true

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			uintVal, err := strconv.ParseUint(envValue, 10, 64)
			if err != nil {
				return zero, false
			}
			cv, cerr := checkedConvert(reflect.ValueOf(uintVal), targetType)
			if cerr != nil {
				return zero, false
			}
			return cv.Interface().(T), true

		case reflect.Float32, reflect.Float64:
			floatVal, err := strconv.ParseFloat(envValue, 64)
			if err != nil {
				return zero, false
			}
			cv, cerr := checkedConvert(reflect.ValueOf(floatVal), targetType)
			if cerr != nil {
				return zero, false
			}
			return cv.Interface().(T), true
		}
	}

	// For complex types, try JSON decoding
	if strings.HasPrefix(envValue, "[") || strings.HasPrefix(envValue, "{") {
		value := reflect.New(targetType).Interface()
		if err := json.Unmarshal([]byte(envValue), value); err == nil {
			return reflect.ValueOf(value).Elem().Interface().(T), true
		}
	}

	return zero, false
}

// GetValueOrFallback returns the current value of the configuration option or the provided fallback value if retrieval fails.
// This is a convenience method to avoid having to handle errors when getting config values.
//
// Example:
//
//	// Instead of:
//	val, err := option.GetValue()
//	if err != nil {
//	    val = fallbackVal
//	}
//
//	// You can use:
//	val := option.GetValueOrFallback(fallbackVal)
func (o *ConfigOption[T]) GetValueOrFallback(fallbackVal T) T {
	val, err := o.GetValue()
	if err != nil {
		return fallbackVal
	}
	return val
}
