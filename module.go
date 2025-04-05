// Package base provides a generic base module that can be extended with different configurations.
package base

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
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

// ConfigurableModule provides a generic base module that can be extended with different configurations.
type ConfigurableModule[T any] struct {
	configs     map[string]*ConfigOption[T]
	initialized bool
	mu          sync.RWMutex
}

// Errors related to configuration operations
var (
	// ErrConfigNotSet is the error when the configuration is not set.
	ErrConfigNotSet = errors.New("config not set")
	// ErrConfigRequired is the error when a required configuration is not set.
	ErrConfigRequired = errors.New("required config not set")
	// ErrConfigInvalidValue is the error when a configuration value is invalid.
	ErrConfigInvalidValue = errors.New("invalid config value")
	// ErrModuleAlreadyInitialized is the error when trying to modify a module after it's initialized.
	ErrModuleAlreadyInitialized = errors.New("module already initialized")
)

// NewConfigurableModule creates a new instance of ConfigurableModule.
func NewConfigurableModule[T any]() *ConfigurableModule[T] {
	return &ConfigurableModule[T]{
		configs: make(map[string]*ConfigOption[T]),
	}
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
func (o *ConfigOption[T]) getValue() T {
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

// SetConfigOption sets a configuration option for a given name.
func (m *ConfigurableModule[T]) SetConfigOption(name string, option *ConfigOption[T]) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return ErrModuleAlreadyInitialized
	}

	m.configs[name] = option
	return nil
}

// SetConfig sets a configuration getter for a given name.
func (m *ConfigurableModule[T]) SetConfig(name string, getter ConfigGetter[T]) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return ErrModuleAlreadyInitialized
	}

	option, exists := m.configs[name]
	if !exists {
		var zero T
		option = NewConfigOption(zero).WithGetter(getter)
		m.configs[name] = option
		return nil
	}

	option.Getter = getter
	return nil
}

// SetConfigValue sets a configuration value for a given name.
func (m *ConfigurableModule[T]) SetConfigValue(name string, value T) error {
	option, exists := m.configs[name]
	if !exists {
		m.mu.Lock()
		if m.initialized {
			m.mu.Unlock()
			return ErrModuleAlreadyInitialized
		}

		option = NewConfigOption(value)
		m.configs[name] = option
		m.mu.Unlock()
		return nil
	}

	return option.setValue(value)
}

// Initialize finalizes the module configuration and makes it immutable.
func (m *ConfigurableModule[T]) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if all required configs are set
	for name, option := range m.configs {
		if option.IsRequired {
			// For required options, ensure they have a value or a getter
			if !option.hasValue && option.Getter == nil {
				var zero T
				if reflect.DeepEqual(option.Default, zero) {
					return fmt.Errorf("%w: %s", ErrConfigRequired, name)
				}
			}
		}
	}

	m.initialized = true
	return nil
}

// GetConfig retrieves the configuration value for a given name.
func (m *ConfigurableModule[T]) GetConfig(name string) (T, error) {
	var zero T

	m.mu.RLock()
	option, exists := m.configs[name]
	if !exists {
		m.mu.RUnlock()
		return zero, fmt.Errorf("%w: %s", ErrConfigNotSet, name)
	}
	m.mu.RUnlock()

	return option.getValue(), nil
}

// genSetConfig generates a Starlark callable function to set a configuration value.
func (m *ConfigurableModule[T]) genSetConfig(name string) starlark.Callable {
	return starlark.NewBuiltin(name, func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var v starlark.Value
		if err := starlark.UnpackArgs(b.Name(), args, kwargs, name, &v); err != nil {
			return nil, err
		}

		// Convert to Go value
		gv, err := dataconv.Unmarshal(v)
		if err != nil {
			return nil, err
		}

		// Check type
		vt, ok := gv.(T)
		if !ok {
			return nil, fmt.Errorf("value type mismatch, expected %T, got %T", *new(T), gv)
		}

		// Set config
		if err := m.SetConfigValue(name, vt); err != nil {
			return nil, err
		}
		return starlark.None, nil
	})
}

// genGetConfig generates a Starlark callable function to get a configuration value.
func (m *ConfigurableModule[T]) genGetConfig(name string) starlark.Callable {
	return starlark.NewBuiltin("get_"+name, func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// Get config value
		value, err := m.GetConfig(name)
		if err != nil {
			return nil, err
		}

		// Convert to Starlark value
		sv, err := dataconv.Marshal(value)
		if err != nil {
			return nil, err
		}

		return sv, nil
	})
}

// ListConfigs returns information about all configuration options.
func (m *ConfigurableModule[T]) ListConfigs() map[string]map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]map[string]interface{})

	for name, option := range m.configs {
		info := map[string]interface{}{
			"description": option.Description,
			"required":    option.IsRequired,
			"secret":      option.IsSecret,
			"has_value":   option.hasValue,
			"has_getter":  option.Getter != nil,
		}

		// Don't include actual values for secrets
		if !option.IsSecret && option.hasValue {
			info["value"] = option.value
		}
		result[name] = info
	}

	return result
}

// LoadModule returns a Starlark module loader with the given configurations and additional functions.
func (m *ConfigurableModule[T]) LoadModule(moduleName string, additionalFuncs starlark.StringDict) starlet.ModuleLoader {
	// Ensure all required configs are set
	if err := m.Initialize(); err != nil {
		panic(err)
	}

	sd := starlark.StringDict{}

	// Add setter functions
	for name := range m.configs {
		sd["set_"+name] = m.genSetConfig(name)
		sd["get_"+name] = m.genGetConfig(name)
	}

	// Add helper functions for listing configs
	sd["list_configs"] = starlark.NewBuiltin("list_configs", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		configs := m.ListConfigs()
		return dataconv.Marshal(configs)
	})

	// Add additional functions
	for k, v := range additionalFuncs {
		sd[k] = v
	}

	return dataconv.WrapModuleData(moduleName, sd)
}
