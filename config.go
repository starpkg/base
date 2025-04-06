// Package base provides a generic base module that can be extended with different configurations.
package base

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
)

// ConfigGetter is a function type that returns a value of type T.
type ConfigGetter[T any] func() T

// ConfigValidator is a function type that validates a configuration value of type T.
type ConfigValidator[T any] func(value T) error

// ValuePriority defines which value source takes precedence when multiple are available.
type ValuePriority int

const (
	// PrioritySetValue means explicitly set values take precedence over getters.
	PrioritySetValue ValuePriority = iota
	// PriorityGetter means getter functions take precedence over set values.
	PriorityGetter
)

// ConfigOption represents an option for a configuration value.
// Use NewConfigOption to create a new instance and the provided builder methods
// to configure it (WithDescription, WithValidator, WithGetter, Required, Secret).
type ConfigOption[T any] struct {
	// Default is the default value for the configuration.
	// Validator will not be called on the default value.
	Default T

	// Name is the unique identifier for this configuration option.
	// This is used when registering the option with a ConfigurableModule.
	Name string

	// Description is a human-readable description of the configuration.
	// This is used for documentation and displayed when listing configurations.
	// It's recommended to provide a clear, concise explanation of what the config does.
	Description string

	// Private fields that should be accessed only through methods
	getter        ConfigGetter[T]    // Use WithGetter() to set
	validator     ConfigValidator[T] // Use WithValidator() to set
	isRequired    bool               // Use Required() to set
	isSecret      bool               // Use Secret() to set
	valuePriority ValuePriority      // Determines which value takes precedence

	// Internal state fields
	mu       sync.RWMutex
	value    T
	hasValue bool
}

// NewConfigOption creates a new configuration option.
func NewConfigOption[T any](defaultValue T) *ConfigOption[T] {
	return &ConfigOption[T]{
		Default:       defaultValue,
		valuePriority: PrioritySetValue, // Default to prioritizing explicitly set values
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

// WithValidator adds a validator to the configuration option, which is called when the value is set.
func (o *ConfigOption[T]) WithValidator(validator ConfigValidator[T]) *ConfigOption[T] {
	o.validator = validator
	return o
}

// WithGetter adds a custom getter to the configuration option.
func (o *ConfigOption[T]) WithGetter(getter ConfigGetter[T]) *ConfigOption[T] {
	o.getter = getter
	return o
}

// Required marks the configuration option as required.
func (o *ConfigOption[T]) Required() *ConfigOption[T] {
	o.isRequired = true
	return o
}

// Secret marks the configuration option as a secret.
func (o *ConfigOption[T]) Secret() *ConfigOption[T] {
	o.isSecret = true
	return o
}

// PreferGetter configures the option to prefer dynamic values from the getter.
func (o *ConfigOption[T]) PreferGetter() *ConfigOption[T] {
	o.valuePriority = PriorityGetter
	return o
}

// PreferSetValue configures the option to prefer explicitly set values (this is the default behavior).
func (o *ConfigOption[T]) PreferSetValue() *ConfigOption[T] {
	o.valuePriority = PrioritySetValue
	return o
}

// Internal methods

// resolveValue returns the current value based on priority settings.
func (o *ConfigOption[T]) resolveValue() T {
	if o.valuePriority == PriorityGetter {
		if o.getter != nil {
			return o.getter()
		}
		if o.hasValue {
			return o.value
		}
		return o.Default
	}

	// Default is PrioritySetValue
	if o.hasValue {
		return o.value
	}
	if o.getter != nil {
		return o.getter()
	}
	return o.Default
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

// IsDefault returns whether the configuration option has the default value.
func (o *ConfigOption[T]) IsDefault() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	var zero T
	return reflect.DeepEqual(o.Default, zero)
}

// GetInfo returns information about the configuration option.
func (o *ConfigOption[T]) GetInfo() map[string]interface{} {
	o.mu.RLock()

	info := map[string]interface{}{
		"name":        o.Name,
		"description": o.Description,
		"required":    o.isRequired,
		"secret":      o.isSecret,
		"has_value":   o.hasValue,
		"has_getter":  o.getter != nil,
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

	// Check type
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

// SetSecret sets whether the configuration option is secret.
func (o *ConfigOption[T]) SetSecret(secret bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.isSecret = secret
}
