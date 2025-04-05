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
	Default T

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

// WithDescription adds a description to the configuration option.
// The description is used for documentation and displayed when listing configurations.
func (o *ConfigOption[T]) WithDescription(desc string) *ConfigOption[T] {
	o.Description = desc
	return o
}

// WithValidator adds a validator to the configuration option.
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

// PreferGetter configures the option to prefer dynamic values from the getter
// even when a direct value has been set.
func (o *ConfigOption[T]) PreferGetter() *ConfigOption[T] {
	o.valuePriority = PriorityGetter
	return o
}

// PreferSetValue configures the option to prefer explicitly set values
// over dynamic values from the getter (this is the default behavior).
func (o *ConfigOption[T]) PreferSetValue() *ConfigOption[T] {
	o.valuePriority = PrioritySetValue
	return o
}

// getValue returns the current value of the configuration option.
func (o *ConfigOption[T]) getValue() (T, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// If the configuration is marked as secret, don't allow retrieval
	if o.isSecret {
		var zero T
		return zero, ErrSecretConfigNotRetrievable
	}

	// Handle based on priority setting
	switch o.valuePriority {
	case PriorityGetter:
		// Check getter first, then fall back to set value, then default
		if o.getter != nil {
			return o.getter(), nil
		}
		if o.hasValue {
			return o.value, nil
		}
		return o.Default, nil

	case PrioritySetValue:
		// Check set value first, then fall back to getter, then default
		if o.hasValue {
			return o.value, nil
		}
		if o.getter != nil {
			return o.getter(), nil
		}
		return o.Default, nil

	default:
		// Should never happen, but fallback to default case
		if o.hasValue {
			return o.value, nil
		}
		if o.getter != nil {
			return o.getter(), nil
		}
		return o.Default, nil
	}
}

// getSecretValue returns the value even if it's secret (for internal use only).
func (o *ConfigOption[T]) getSecretValue() T {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// Handle based on priority setting
	switch o.valuePriority {
	case PriorityGetter:
		// Check getter first, then fall back to set value, then default
		if o.getter != nil {
			return o.getter()
		}
		if o.hasValue {
			return o.value
		}
		return o.Default

	case PrioritySetValue:
		// Check set value first, then fall back to getter, then default
		if o.hasValue {
			return o.value
		}
		if o.getter != nil {
			return o.getter()
		}
		return o.Default

	default:
		// Should never happen, but fallback to default case
		if o.hasValue {
			return o.value
		}
		if o.getter != nil {
			return o.getter()
		}
		return o.Default
	}
}

// setValue sets the value of the configuration option.
func (o *ConfigOption[T]) setValue(value T) error {
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
	return o.getter != nil
}
