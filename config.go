package base

import (
	"encoding/json"
	"fmt"
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
}

// NewConfigOption creates a new configuration option with the given default value.
func NewConfigOption[T any](defaultValue T) *ConfigOption[T] {
	return &ConfigOption[T]{
		defaultVal: defaultValue,
	}
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
func (o *ConfigOption[T]) SetValueFromStarlark(v starlark.Value) error {
	gv, err := dataconv.Unmarshal(v)
	if err != nil {
		return err
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

			// Try to convert numeric types
			if isNumericType(elemType) && isNumericType(srcElemType) {
				srcVal := reflect.ValueOf(srcElem)
				if srcVal.Type().ConvertibleTo(elemType) {
					destSlice.Index(i).Set(srcVal.Convert(elemType))
					continue
				}
			}

			// Try direct conversion
			if srcElemType.ConvertibleTo(elemType) {
				destSlice.Index(i).Set(reflect.ValueOf(srcElem).Convert(elemType))
			} else {
				return fmt.Errorf("element at index %d cannot be converted from %v to %v", i, srcElemType, elemType)
			}
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

			// Try to convert numeric types for keys
			var destKey reflect.Value
			if isNumericType(keyType) && isNumericType(srcKeyType) {
				srcKeyVal := reflect.ValueOf(srcKey)
				if srcKeyVal.Type().ConvertibleTo(keyType) {
					destKey = srcKeyVal.Convert(keyType)
				} else {
					return fmt.Errorf("map key cannot be converted from %v to %v", srcKeyType, keyType)
				}
			} else if srcKeyType.ConvertibleTo(keyType) {
				destKey = reflect.ValueOf(srcKey).Convert(keyType)
			} else {
				return fmt.Errorf("map key cannot be converted from %v to %v", srcKeyType, keyType)
			}

			// Try to convert numeric types for values
			var destValue reflect.Value
			if isNumericType(valueType) && isNumericType(srcValueType) {
				srcValueVal := reflect.ValueOf(srcValue)
				if srcValueVal.Type().ConvertibleTo(valueType) {
					destValue = srcValueVal.Convert(valueType)
				} else {
					return fmt.Errorf("map value cannot be converted from %v to %v", srcValueType, valueType)
				}
			} else if srcValueType.ConvertibleTo(valueType) {
				destValue = reflect.ValueOf(srcValue).Convert(valueType)
			} else {
				return fmt.Errorf("map value cannot be converted from %v to %v", srcValueType, valueType)
			}

			destMap.SetMapIndex(destKey, destValue)
		}

		return o.SetValue(destMap.Interface().(T))
	}

	// Handle simple types
	if isNumericType(targetType) && isNumericType(sourceType) {
		if sourceValue.Type().ConvertibleTo(targetType) {
			return o.SetValue(sourceValue.Convert(targetType).Interface().(T))
		}
		return fmt.Errorf("value cannot be converted from %v to %v", sourceType, targetType)
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
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return true
	}
	return false
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
			return reflect.ValueOf(intVal).Convert(targetType).Interface().(T), true

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			uintVal, err := strconv.ParseUint(envValue, 10, 64)
			if err != nil {
				return zero, false
			}
			return reflect.ValueOf(uintVal).Convert(targetType).Interface().(T), true

		case reflect.Float32, reflect.Float64:
			floatVal, err := strconv.ParseFloat(envValue, 64)
			if err != nil {
				return zero, false
			}
			return reflect.ValueOf(floatVal).Convert(targetType).Interface().(T), true
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
