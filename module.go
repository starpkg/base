// Package base provides a generic base module that can be extended with different configurations.
package base

import (
	"fmt"
	"sync"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
)

// ConfigOptionInterface defines the common interface for configuration options.
type ConfigOptionInterface interface {
	// GetName returns the name of the configuration option.
	GetName() string
	// SetName sets the name of the configuration option.
	SetName(name string)
	// IsRequired returns whether the configuration option is required.
	IsRequired() bool
	// IsSecret returns whether the configuration option is secret.
	IsSecret() bool
	// HasSetValue returns whether the configuration option has a value set.
	HasSetValue() bool
	// HasGetter returns whether the configuration option has a getter.
	HasGetter() bool
	// IsDefault returns whether the configuration option has the default value.
	IsDefault() bool
	// GetDescription returns the description of the configuration option.
	GetDescription() string
	// GetInfo returns information about the configuration option.
	GetInfo() map[string]interface{}
	// ValidValue validates whether a starlark value can be properly set to this option.
	ValidValue(v starlark.Value) error
	// SetValueFromStarlark sets the configuration option from a starlark value.
	SetValueFromStarlark(v starlark.Value) error
	// GetStarlarkValue returns the configuration value as a starlark value.
	GetStarlarkValue() (starlark.Value, error)
}

// ConfigurableModule provides a generic base module that can be extended with different configurations.
type ConfigurableModule struct {
	mu          sync.RWMutex
	initialized bool
	configs     map[string]ConfigOptionInterface
}

// NewConfigurableModule creates a new instance of ConfigurableModule.
func NewConfigurableModule() *ConfigurableModule {
	return &ConfigurableModule{
		configs: make(map[string]ConfigOptionInterface),
	}
}

// checkInitialized returns an error if the module is already initialized.
// It must be called with the mutex already locked.
func (m *ConfigurableModule) checkInitialized() error {
	if m.initialized {
		return ErrModuleAlreadyInitialized
	}
	return nil
}

// getOption retrieves a configuration option by name.
// Returns the option and a boolean indicating if it exists.
func (m *ConfigurableModule) getOption(name string) (ConfigOptionInterface, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	option, exists := m.configs[name]
	return option, exists
}

// SetConfigOption sets a configuration option for a given name.
func (m *ConfigurableModule) SetConfigOption(name string, option ConfigOptionInterface) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkInitialized(); err != nil {
		return err
	}

	// Set the Name field if it's not already set
	if option.GetName() == "" {
		option.SetName(name)
	}

	m.configs[name] = option
	return nil
}

// SetTypedConfigOption sets a strongly-typed configuration option for a given name.
// This is a convenience helper for working with generic ConfigOption[T].
func SetTypedConfigOption[T any](m *ConfigurableModule, name string, option *ConfigOption[T]) error {
	return m.SetConfigOption(name, option)
}

// GetTypedConfigOption retrieves a strongly-typed configuration option by name.
// This is a convenience helper for working with generic ConfigOption[T].
// Returns the option and an error if it doesn't exist or has wrong type.
func GetTypedConfigOption[T any](m *ConfigurableModule, name string) (*ConfigOption[T], error) {
	option, exists := m.getOption(name)
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrConfigNotSet, name)
	}

	// Type assertion
	typedOption, ok := option.(*ConfigOption[T])
	if !ok {
		return nil, fmt.Errorf("config '%s' is not of expected type", name)
	}

	return typedOption, nil
}

// SetConfigValue sets a configuration value for a given name.
// This is a convenience helper for working with strongly-typed values.
func SetConfigValue[T any](m *ConfigurableModule, name string, value T) error {
	// Check if option already exists
	option, exists := m.getOption(name)
	if exists {
		// Try to cast to correct type
		typedOption, ok := option.(*ConfigOption[T])
		if !ok {
			return fmt.Errorf("cannot set value of different type for config '%s'", name)
		}
		return typedOption.setValue(value)
	}

	// Create new option
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkInitialized(); err != nil {
		return err
	}

	newOption := NewConfigOption(value).WithName(name)
	m.configs[name] = newOption
	return nil
}

// GetConfigValue retrieves a configuration value for a given name.
// This is a convenience helper for working with strongly-typed values.
func GetConfigValue[T any](m *ConfigurableModule, name string) (T, error) {
	var zero T

	option, err := GetTypedConfigOption[T](m, name)
	if err != nil {
		return zero, err
	}

	return option.getValue()
}

// SetConfigGetter sets a configuration getter for a given name.
// This is a convenience helper for working with strongly-typed getters.
func SetConfigGetter[T any](m *ConfigurableModule, name string, getter ConfigGetter[T]) error {
	// Check if option already exists
	option, exists := m.getOption(name)
	if exists {
		// Try to cast to correct type
		typedOption, ok := option.(*ConfigOption[T])
		if !ok {
			return fmt.Errorf("cannot set getter of different type for config '%s'", name)
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		if err := m.checkInitialized(); err != nil {
			return err
		}

		typedOption.WithGetter(getter)
		return nil
	}

	// Create new option
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkInitialized(); err != nil {
		return err
	}

	var zero T
	newOption := NewConfigOption(zero).WithName(name).WithGetter(getter)
	m.configs[name] = newOption
	return nil
}

// Initialize finalizes the module configuration and makes it immutable.
func (m *ConfigurableModule) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if all required configs are set
	for name, option := range m.configs {
		// Make sure name is set
		if option.GetName() == "" {
			option.SetName(name)
		}

		if option.IsRequired() {
			// For required options, ensure they have a value or a getter
			if !option.HasSetValue() && !option.HasGetter() && option.IsDefault() {
				configName := option.GetName()
				return fmt.Errorf("%w: %s", ErrConfigRequired, configName)
			}
		}
	}

	m.initialized = true
	return nil
}

// findConfig finds a configuration option by name and returns an error if not found.
func (m *ConfigurableModule) findConfig(name string) (ConfigOptionInterface, error) {
	option, exists := m.getOption(name)
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrConfigNotSet, name)
	}

	// Set the Name field if it's not already set
	if option.GetName() == "" {
		m.mu.Lock()
		option.SetName(name)
		m.mu.Unlock()
	}

	return option, nil
}

// genSetConfig generates a Starlark callable function to set a configuration value.
func (m *ConfigurableModule) genSetConfig(name string, option ConfigOptionInterface) starlark.Callable {
	return starlark.NewBuiltin("set_"+name, func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var v starlark.Value
		if err := starlark.UnpackArgs(b.Name(), args, kwargs, "value", &v); err != nil {
			return nil, err
		}

		// Validate value type
		if err := option.ValidValue(v); err != nil {
			return nil, err
		}

		// Set config
		if err := option.SetValueFromStarlark(v); err != nil {
			return nil, err
		}

		return starlark.None, nil
	})
}

// genGetConfig generates a Starlark callable function to get a configuration value.
func (m *ConfigurableModule) genGetConfig(name string, option ConfigOptionInterface) starlark.Callable {
	return starlark.NewBuiltin("get_"+name, func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		// Get config value
		return option.GetStarlarkValue()
	})
}

// ListConfigs returns information about all configuration options.
func (m *ConfigurableModule) ListConfigs() map[string]map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]map[string]interface{})

	for name, option := range m.configs {
		result[name] = option.GetInfo()
	}

	return result
}

// LoadModule returns a Starlark module loader with the given configurations and additional functions.
func (m *ConfigurableModule) LoadModule(moduleName string, additionalFuncs starlark.StringDict) starlet.ModuleLoader {
	// Ensure all required configs are set
	if err := m.Initialize(); err != nil {
		panic(err)
	}

	sd := starlark.StringDict{}

	// Add setter functions for all configs and getter functions only for non-secret configs
	for name, option := range m.configs {
		sd["set_"+name] = m.genSetConfig(name, option)

		// Only add getter functions for non-secret configs
		if !option.IsSecret() {
			sd["get_"+name] = m.genGetConfig(name, option)
		}
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
