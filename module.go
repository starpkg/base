package base

import (
	"fmt"
	"sync"

	"github.com/1set/starlet"
	"go.starlark.net/starlark"
)

// ConfigOptionInterface defines the common interface for configuration options.
type ConfigOptionInterface interface {
	// Core methods
	GetName() string
	SetName(name string)
	IsRequired() bool
	IsSecret() bool
	HasValue() bool
	HasGetter() bool
	HasDefault() bool
	HasEnvVar() bool
	Validate() error

	// Starlark integration
	SetValueFromStarlark(v starlark.Value) error
	GetStarlarkValue() (starlark.Value, error)

	// Go inspection
	GetInfo() map[string]interface{}
}

// ConfigurableModule provides a generic base module that can be extended with different configurations.
type ConfigurableModule struct {
	mu          sync.RWMutex
	initialized bool
	configs     map[string]ConfigOptionInterface
}

// ModuleOption is a function that applies configuration to a ConfigurableModule.
type ModuleOption func(*ConfigurableModule) error

// WithConfigOption sets a configuration option for the module.
func WithConfigOption(name string, option ConfigOptionInterface) ModuleOption {
	return func(m *ConfigurableModule) error {
		return m.SetConfigOption(name, option)
	}
}

// WithTypedConfigOption sets a strongly-typed configuration option for the module.
func WithTypedConfigOption[T any](name string, option *ConfigOption[T]) ModuleOption {
	return func(m *ConfigurableModule) error {
		return SetTypedConfigOption(m, name, option)
	}
}

// WithConfigValue sets a configuration value directly.
func WithConfigValue[T any](name string, value T) ModuleOption {
	return func(m *ConfigurableModule) error {
		return SetConfigValue(m, name, value)
	}
}

// WithConfigGetter sets a dynamic getter function for the module.
func WithConfigGetter[T any](name string, getter ConfigGetter[T]) ModuleOption {
	return func(m *ConfigurableModule) error {
		return SetConfigGetter(m, name, getter)
	}
}

// WithConfigEnvVar sets an environment variable for the configuration option.
func WithConfigEnvVar[T any](name string, envVar string) ModuleOption {
	return func(m *ConfigurableModule) error {
		return SetConfigEnvVar[T](m, name, envVar)
	}
}

// NewConfigurableModule creates a new instance of ConfigurableModule.
func NewConfigurableModule() *ConfigurableModule {
	return &ConfigurableModule{
		configs: make(map[string]ConfigOptionInterface),
	}
}

// NewConfigurableModuleWithOptions creates a new instance of ConfigurableModule with the given options.
// This allows for a more fluent API when creating and configuring modules.
func NewConfigurableModuleWithOptions(options ...ModuleOption) (*ConfigurableModule, error) {
	module := NewConfigurableModule()

	for _, option := range options {
		if err := option(module); err != nil {
			return nil, fmt.Errorf("failed to apply module option: %w", err)
		}
	}

	return module, nil
}

// SetConfigOption sets a configuration option for a given name.
func (m *ConfigurableModule) SetConfigOption(name string, option ConfigOptionInterface) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return ErrModuleAlreadyInitialized
	}

	if option.GetName() == "" {
		option.SetName(name)
	}

	m.configs[name] = option
	return nil
}

// Initialize finalizes the module configuration and makes it immutable.
// During initialization, it:
// 1. Checks that all required options either have a value, a getter, or a default
// 2. Validates all explicitly set values (via SetValue or WithValue)
//    Note: Values from getter functions are NOT validated during initialization
// Returns an error if any required configs are missing or if any value fails validation.
func (m *ConfigurableModule) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if all required configs are set
	for name, option := range m.configs {
		if option.GetName() == "" {
			option.SetName(name)
		}

		if option.IsRequired() && !option.HasValue() && !option.HasGetter() && !option.HasEnvVar() && !option.HasDefault() {
			return fmt.Errorf("%w: %s", ErrConfigRequired, option.GetName())
		}

		// Validate all options with explicitly set values (including those set with WithValue)
		// Note: This only validates the explicitly set values, not values from getters
		if err := option.Validate(); err != nil {
			return fmt.Errorf("validation failed for option '%s': %w", name, err)
		}
	}

	m.initialized = true
	return nil
}

// LoadModule returns a Starlark module loader with the given configurations and additional functions.
func (m *ConfigurableModule) LoadModule(moduleName string, additionalFuncs starlark.StringDict) starlet.ModuleLoader {
	if err := m.Initialize(); err != nil {
		panic(err)
	}

	return func() (starlark.StringDict, error) {
		sd := starlark.StringDict{}

		// Add setter and getter functions for all configs
		for name, option := range m.configs {
			sd["set_"+name] = m.genSetFunction(name, option)

			// Only add getter functions for non-secret configs
			if !option.IsSecret() {
				sd["get_"+name] = m.genGetFunction(name, option)
			}
		}

		// Add additional functions
		for k, v := range additionalFuncs {
			sd[k] = v
		}

		return sd, nil
	}
}

// genSetFunction generates a Starlark callable function to set a configuration value.
func (m *ConfigurableModule) genSetFunction(name string, option ConfigOptionInterface) starlark.Callable {
	return starlark.NewBuiltin("set_"+name, func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var v starlark.Value
		if err := starlark.UnpackArgs(b.Name(), args, kwargs, "value", &v); err != nil {
			return nil, err
		}

		// Set value directly, validation happens inside SetValueFromStarlark
		if err := option.SetValueFromStarlark(v); err != nil {
			return nil, err
		}

		return starlark.None, nil
	})
}

// genGetFunction generates a Starlark callable function to get a configuration value.
func (m *ConfigurableModule) genGetFunction(name string, option ConfigOptionInterface) starlark.Callable {
	return starlark.NewBuiltin("get_"+name, func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return option.GetStarlarkValue()
	})
}

// Helper functions

// SetTypedConfigOption sets a strongly-typed configuration option.
func SetTypedConfigOption[T any](m *ConfigurableModule, name string, option *ConfigOption[T]) error {
	return m.SetConfigOption(name, option)
}

// GetConfigValue retrieves a configuration value.
func GetConfigValue[T any](m *ConfigurableModule, name string) (T, error) {
	var zero T

	m.mu.RLock()
	option, exists := m.configs[name]
	m.mu.RUnlock()

	if !exists {
		return zero, fmt.Errorf("%w: %s", ErrConfigNotSet, name)
	}

	typedOption, ok := option.(*ConfigOption[T])
	if !ok {
		return zero, fmt.Errorf("config '%s' is not of expected type", name)
	}

	return typedOption.GetValue()
}

// SetConfigValue sets a configuration value.
func SetConfigValue[T any](m *ConfigurableModule, name string, value T) error {
	m.mu.RLock()
	option, exists := m.configs[name]
	m.mu.RUnlock()

	if exists {
		typedOption, ok := option.(*ConfigOption[T])
		if !ok {
			return fmt.Errorf("cannot set value of different type for config '%s'", name)
		}
		return typedOption.SetValue(value)
	}

	// Create new option if it doesn't exist
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return ErrModuleAlreadyInitialized
	}

	newOption := NewConfigOption(value).WithName(name)
	m.configs[name] = newOption
	return nil
}

// SetConfigGetter sets a configuration getter.
func SetConfigGetter[T any](m *ConfigurableModule, name string, getter ConfigGetter[T]) error {
	m.mu.RLock()
	option, exists := m.configs[name]
	m.mu.RUnlock()

	if exists {
		typedOption, ok := option.(*ConfigOption[T])
		if !ok {
			return fmt.Errorf("cannot set getter of different type for config '%s'", name)
		}

		m.mu.Lock()
		if m.initialized {
			m.mu.Unlock()
			return ErrModuleAlreadyInitialized
		}

		typedOption.WithGetter(getter)
		m.mu.Unlock()
		return nil
	}

	// Create new option if it doesn't exist
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return ErrModuleAlreadyInitialized
	}

	var zero T
	newOption := NewConfigOption(zero).WithName(name).WithGetter(getter)
	m.configs[name] = newOption
	return nil
}

// SetConfigEnvVar sets an environment variable for a configuration option.
func SetConfigEnvVar[T any](m *ConfigurableModule, name string, envVar string) error {
	m.mu.RLock()
	option, exists := m.configs[name]
	m.mu.RUnlock()

	if exists {
		typedOption, ok := option.(*ConfigOption[T])
		if !ok {
			return fmt.Errorf("cannot set environment variable for config of different type '%s'", name)
		}

		m.mu.Lock()
		if m.initialized {
			m.mu.Unlock()
			return ErrModuleAlreadyInitialized
		}

		typedOption.WithEnvVar(envVar)
		m.mu.Unlock()
		return nil
	}

	// Create new option if it doesn't exist
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return ErrModuleAlreadyInitialized
	}

	var zero T
	newOption := NewConfigOption(zero).WithName(name).WithEnvVar(envVar)
	m.configs[name] = newOption
	return nil
}

// ListConfigs returns a map of configuration information
func (m *ConfigurableModule) ListConfigs() map[string]map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]map[string]interface{})
	for name, option := range m.configs {
		result[name] = option.GetInfo()
	}
	return result
}

// GetConfigOption retrieves a configuration option by name.
func (m *ConfigurableModule) GetConfigOption(name string) (ConfigOptionInterface, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	option, exists := m.configs[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrConfigNotSet, name)
	}

	return option, nil
}
