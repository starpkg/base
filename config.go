// Package base provides a generic base module that can be extended with different configurations.
package base

import (
	"fmt"
	"sync"
)

// ConfigGetter is a function type that returns a value of type T.
type ConfigGetter[T any] func() T

// ConfigValidator is a function type that validates a configuration value of type T.
type ConfigValidator[T any] func(value T) error

// ConfigOption represents an option for a configuration value.
type ConfigOption[T any] struct {
	// Default is the default value for the configuration.
	Default T
	// Getter is a function that returns the configuration value.
	Getter ConfigGetter[T]
	// Validator is a function that validates the configuration value.
	Validator ConfigValidator[T]
	// Description is a human-readable description of the configuration.
	Description string
	// IsRequired indicates whether the configuration is required.
	IsRequired bool
	// IsSecret indicates whether the configuration is a secret (like API keys), secret values can't be retrieved from the module.
	IsSecret bool
	// Value holds the current value of the configuration.
	value    T
	hasValue bool
	mu       sync.RWMutex
}

// NewConfigOption creates a new configuration option.
func NewConfigOption[T any](defaultValue T) *ConfigOption[T] {
	return &ConfigOption[T]{
		Default: defaultValue,
	}
}

// WithDescription adds a description to the configuration option.
func (o *ConfigOption[T]) WithDescription(desc string) *ConfigOption[T] {
	o.Description = desc
	return o
}

// WithValidator adds a validator to the configuration option.
func (o *ConfigOption[T]) WithValidator(validator ConfigValidator[T]) *ConfigOption[T] {
	o.Validator = validator
	return o
}

// WithGetter adds a custom getter to the configuration option.
func (o *ConfigOption[T]) WithGetter(getter ConfigGetter[T]) *ConfigOption[T] {
	o.Getter = getter
	return o
}

// Required marks the configuration option as required.
func (o *ConfigOption[T]) Required() *ConfigOption[T] {
	o.IsRequired = true
	return o
}

// Secret marks the configuration option as a secret.
func (o *ConfigOption[T]) Secret() *ConfigOption[T] {
	o.IsSecret = true
	return o
}

// getValue returns the current value of the configuration option.
func (o *ConfigOption[T]) getValue() (T, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// If the configuration is marked as secret, don't allow retrieval
	if o.IsSecret {
		var zero T
		return zero, ErrSecretConfigNotRetrievable
	}

	if o.hasValue {
		return o.value, nil
	}

	if o.Getter != nil {
		return o.Getter(), nil
	}

	return o.Default, nil
}

// getSecretValue returns the value even if it's secret (for internal use only).
func (o *ConfigOption[T]) getSecretValue() T {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.hasValue {
		return o.value
	}

	if o.Getter != nil {
		return o.Getter()
	}

	return o.Default
}

// setValue sets the value of the configuration option.
func (o *ConfigOption[T]) setValue(value T) error {
	if o.Validator != nil {
		if err := o.Validator(value); err != nil {
			return fmt.Errorf("%w: %v", ErrConfigInvalidValue, err)
		}
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	o.value = value
	o.hasValue = true
	return nil
}

// hasSetValue returns whether the configuration option has a value set.
func (o *ConfigOption[T]) hasSetValue() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.hasValue
}

// hasGetter returns whether the configuration option has a getter.
func (o *ConfigOption[T]) hasGetter() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.Getter != nil
}
