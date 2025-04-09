package base

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
)

// ConfigGetter is a function type that returns a value of type T.
type ConfigGetter[T any] func() T

// ConfigValidator is a function type that validates a configuration value of type T.
type ConfigValidator[T any] func(value T) error

// ConfigOption represents an option for a configuration value.
// Use NewConfigOption to create a new instance and the provided builder methods
// to configure it (WithDescription, WithValidator, WithGetter, Required, Secret).
type ConfigOption[T any] struct {
	// Name is the unique identifier for this configuration option.
	// This is used when registering the option with a ConfigurableModule.
	Name string

	// Description is a human-readable description of the configuration.
	// This is used for documentation and displayed when listing configurations.
	// It's recommended to provide a clear, concise explanation of what the config does.
	Description string

	// EnvVar is the name of an environment variable to look up for this configuration.
	// If specified and the environment variable exists, its value will be used
	// according to the priority order: immediate value, getter, environment variable, default.
	EnvVar string

	// Default is the default value for the configuration.
	// Validator will not be called on the default value.
	Default T

	// Private fields that should be accessed only through methods
	getter     ConfigGetter[T]    // Use WithGetter() to set
	validator  ConfigValidator[T] // Use WithValidator() to set
	isRequired bool               // Use Required() to set
	isSecret   bool               // Use Secret() to set

	// Internal state fields
	mu       sync.RWMutex
	value    T
	hasValue bool
}

// NewConfigOption creates a new configuration option.
func NewConfigOption[T any](defaultValue T) *ConfigOption[T] {
	return &ConfigOption[T]{
		Default: defaultValue,
	}
}

// Builder methods

// WithName sets the name of the configuration option.
func (o *ConfigOption[T]) WithName(name string) *ConfigOption[T] {
	o.Name = name
	return o
}

// WithDescription adds a description to the configuration option.
func (o *ConfigOption[T]) WithDescription(desc string) *ConfigOption[T] {
	o.Description = desc
	return o
}

// WithEnvVar specifies an environment variable name to check for this configuration.
func (o *ConfigOption[T]) WithEnvVar(envVar string) *ConfigOption[T] {
	o.EnvVar = envVar
	return o
}

// WithValue sets the value of the configuration option.
// This is useful for chain calls when building a configuration option.
// Unlike SetValue, this method ignores any validators since it's part of a builder chain.
// Validation will occur during module initialization when Initialize() is called.
// If you need immediate validation, use SetValue instead.
func (o *ConfigOption[T]) WithValue(value T) *ConfigOption[T] {
	// Skip validator checks in the builder pattern
	// Validation will happen during Initialize() when the module is finalized
	o.value = value
	o.hasValue = true
	return o
}

// WithGetter adds a custom getter to the configuration option.
func (o *ConfigOption[T]) WithGetter(getter ConfigGetter[T]) *ConfigOption[T] {
	o.getter = getter
	return o
}

// WithValidator adds a validator to the configuration option, which is called when the value is set.
func (o *ConfigOption[T]) WithValidator(validator ConfigValidator[T]) *ConfigOption[T] {
	o.validator = validator
	return o
}

// SetRequired sets whether the configuration option is required.
// By default, an option is not required.
func (o *ConfigOption[T]) SetRequired(required bool) *ConfigOption[T] {
	o.isRequired = required
	return o
}

// SetSecret sets whether the configuration option is secret.
// Secret options cannot be retrieved through GetValue.
// By default, an option is not secret.
func (o *ConfigOption[T]) SetSecret(secret bool) *ConfigOption[T] {
	o.isSecret = secret
	return o
}

// Public methods

// GetValue returns the current value of the configuration option.
func (o *ConfigOption[T]) GetValue() (T, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// If the configuration is marked as secret, don't allow retrieval
	if o.isSecret {
		var zero T
		return zero, ErrSecretConfigNotRetrievable
	}

	return o.resolveValue(), nil
}

// SetValue sets the value of the configuration option.
func (o *ConfigOption[T]) SetValue(value T) error {
	if o.validator != nil {
		if err := o.validator(value); err != nil {
			return fmt.Errorf("%w: %v", ErrConfigInvalidValue, err)
		}
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	o.value = value
	o.hasValue = true
	return nil
}

// Validate validates the current value if a validator is set.
// This method ONLY validates the explicitly set value (via SetValue or WithValue),
// and does NOT validate values returned from a getter function.
// Validation only occurs if both a validator is set AND a value has been explicitly set.
// Returns nil if no validator is set, no value has been set, or the validation passes.
func (o *ConfigOption[T]) Validate() error {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// Skip validation if there's no validator or no value has been set
	if o.validator == nil || !o.hasValue {
		return nil
	}

	// Run the validator on the explicitly set value
	// Note: This does NOT validate values from getter functions
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
	return !reflect.DeepEqual(o.Default, zero)
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

	// Only include values for non-secret configs
	if !o.isSecret {
		o.mu.RUnlock()
		val, err := o.GetValue()
		o.mu.RLock()
		if err == nil {
			info["value"] = val
		}
	}

	o.mu.RUnlock()
	return info
}

// Starlark integration

// SetValueFromStarlark sets the configuration option from a starlark value.
func (o *ConfigOption[T]) SetValueFromStarlark(v starlark.Value) error {
	// Convert to Go value
	gv, err := dataconv.Unmarshal(v)
	if err != nil {
		return err
	}

	targetType := reflect.TypeOf((*T)(nil)).Elem()
	sourceValue := reflect.ValueOf(gv)
	sourceType := reflect.TypeOf(gv)

	// Special case for slices/arrays to handle type conversion
	if targetType.Kind() == reflect.Slice && sourceType.Kind() == reflect.Slice {
		// Convert from []interface{} to the target slice type
		destSlice := reflect.MakeSlice(targetType, sourceValue.Len(), sourceValue.Len())

		elemType := targetType.Elem()
		for i := 0; i < sourceValue.Len(); i++ {
			srcElem := sourceValue.Index(i).Interface()
			if reflect.TypeOf(srcElem).ConvertibleTo(elemType) {
				destSlice.Index(i).Set(reflect.ValueOf(srcElem).Convert(elemType))
			} else {
				return fmt.Errorf("element at index %d cannot be converted to %v", i, elemType)
			}
		}

		// Create a typed value from the converted slice
		typedVal := destSlice.Interface().(T)
		return o.SetValue(typedVal)
	}

	// Special case for maps to handle key/value type conversion
	if targetType.Kind() == reflect.Map && sourceType.Kind() == reflect.Map {
		// Create a new map of the target type
		destMap := reflect.MakeMap(targetType)

		// Get the key and value types of the target map
		keyType := targetType.Key()
		valueType := targetType.Elem()

		// Iterate over the source map and convert each key/value pair
		iter := sourceValue.MapRange()
		for iter.Next() {
			srcKey := iter.Key().Interface()
			srcValue := iter.Value().Interface()

			// Convert key
			var destKey reflect.Value
			if reflect.TypeOf(srcKey).ConvertibleTo(keyType) {
				destKey = reflect.ValueOf(srcKey).Convert(keyType)
			} else {
				return fmt.Errorf("map key %v cannot be converted to %v", srcKey, keyType)
			}

			// Convert value
			var destValue reflect.Value
			if reflect.TypeOf(srcValue).ConvertibleTo(valueType) {
				destValue = reflect.ValueOf(srcValue).Convert(valueType)
			} else {
				return fmt.Errorf("map value %v for key %v cannot be converted to %v", srcValue, srcKey, valueType)
			}

			// Set the converted key/value in the destination map
			destMap.SetMapIndex(destKey, destValue)
		}

		// Create a typed value from the converted map
		typedVal := destMap.Interface().(T)
		return o.SetValue(typedVal)
	}

	// Try direct type assertion for simple types
	vt, ok := gv.(T)
	if !ok {
		var zero T
		return fmt.Errorf("value type mismatch, expected %T, got %T", zero, gv)
	}

	// Set value (this will run any custom validators)
	return o.SetValue(vt)
}

// GetStarlarkValue returns the configuration value as a starlark value.
func (o *ConfigOption[T]) GetStarlarkValue() (starlark.Value, error) {
	// Get configuration value
	value, err := o.GetValue()
	if err != nil {
		return nil, err
	}

	// Convert to Starlark value
	return dataconv.Marshal(value)
}

// Internal methods

// resolveValue returns the current value based on the priority order:
// 1. Immediate value (if set)
// 2. Getter method (if available)
// 3. Environment variable (if specified and available)
// 4. Default value
func (o *ConfigOption[T]) resolveValue() T {
	// Priority 1: Immediate value
	if o.hasValue {
		return o.value
	}

	// Priority 2: Getter method
	if o.getter != nil {
		return o.getter()
	}

	// Priority 3: Environment variable
	if o.EnvVar != "" {
		if envValue, exists := os.LookupEnv(o.EnvVar); exists {
			// Try to convert the environment variable value to the right type
			converted, ok := o.convertEnvValue(envValue)
			if ok {
				return converted
			}
			// If conversion fails, continue to the next priority
		}
	}

	// Priority 4: Default value
	return o.Default
}

// convertEnvValue attempts to convert an environment variable string value
// to the target type T.
func (o *ConfigOption[T]) convertEnvValue(envValue string) (T, bool) {
	var zero T
	targetType := reflect.TypeOf(zero)

	// Special case handling for direct string types
	if targetType.Kind() == reflect.String {
		stringValue := reflect.ValueOf(envValue).Convert(targetType).Interface().(T)
		return stringValue, true
	}

	// Special case handling for common scalar types
	switch targetType.Kind() {
	case reflect.Bool:
		// Handle boolean values more directly, accepting various formats
		lowerVal := strings.ToLower(envValue)
		var boolValue bool
		if lowerVal == "true" || lowerVal == "yes" || lowerVal == "1" || lowerVal == "on" {
			boolValue = true
		} else if lowerVal == "false" || lowerVal == "no" || lowerVal == "0" || lowerVal == "off" {
			boolValue = false
		} else {
			return zero, false // Invalid boolean format
		}
		return reflect.ValueOf(boolValue).Convert(targetType).Interface().(T), true

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

	// For complex types (slices, maps, structs) use Starlark parsing
	// Try to parse as a literal first (for lists, maps, etc.)
	if strings.HasPrefix(envValue, "[") || strings.HasPrefix(envValue, "{") {
		// Try to parse as a Starlark value
		starValue, err := starlark.Eval(
			&starlark.Thread{Name: "env-convert"},
			"<env>",
			envValue,
			nil,
		)
		if err == nil {
			goValue, err := dataconv.Unmarshal(starValue)
			if err == nil {
				// Check if we got something that's compatible with our target type
				if reflect.TypeOf(goValue).ConvertibleTo(targetType) {
					return reflect.ValueOf(goValue).Convert(targetType).Interface().(T), true
				}

				// Special handling for slices/arrays
				if targetType.Kind() == reflect.Slice && reflect.TypeOf(goValue).Kind() == reflect.Slice {
					goSlice := reflect.ValueOf(goValue)
					targetSlice := reflect.MakeSlice(targetType, goSlice.Len(), goSlice.Len())

					// Try to convert each element
					elemType := targetType.Elem()
					for i := 0; i < goSlice.Len(); i++ {
						srcElem := goSlice.Index(i).Interface()
						if reflect.TypeOf(srcElem).ConvertibleTo(elemType) {
							targetSlice.Index(i).Set(reflect.ValueOf(srcElem).Convert(elemType))
						} else {
							return zero, false // Element type mismatch
						}
					}

					return targetSlice.Interface().(T), true
				}
			}
		}
	}

	// If we got here, conversion failed
	return zero, false
}
