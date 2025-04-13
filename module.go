package base

import (
	"fmt"
	"sync"

	"github.com/1set/starlet"
	"go.starlark.net/starlark"
)

// ConfigOptionInterface defines the common interface for configuration options.
// It is used by ConfigurableModule to manage configuration settings.
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

// ConfigurableModule provides a generic module that can be extended with different configurations.
// Once initialized, the module becomes immutable.
//
// Configuration values are resolved in the following priority order (from highest to lowest):
// 1. Immediate value (set via WithValue/SetValue)
// 2. Returned value from the getter function (set via WithGetter)
// 3. Environment variable value (set via WithEnvVar)
// 4. Default value (set via WithDefault or NewConfigOption)
type ConfigurableModule struct {
	mu          sync.RWMutex
	initialized bool
	configs     map[string]ConfigOptionInterface
}

// ModuleOption applies a configuration to the module.
type ModuleOption func(*ConfigurableModule) error

// WithConfigOption registers a configuration option for the module.
func WithConfigOption(name string, option ConfigOptionInterface) ModuleOption {
	return func(m *ConfigurableModule) error {
		return m.SetConfigOption(name, option)
	}
}

// WithTypedConfigOption registers a strongly-typed configuration option for the module.
func WithTypedConfigOption[T any](name string, option *ConfigOption[T]) ModuleOption {
	return func(m *ConfigurableModule) error {
		return SetTypedConfigOption(m, name, option)
	}
}

// WithConfigValue sets a configuration value directly.
// This has the highest priority in the resolution order.
func WithConfigValue[T any](name string, value T) ModuleOption {
	return func(m *ConfigurableModule) error {
		return SetConfigValue(m, name, value)
	}
}

// WithConfigGetter registers a dynamic getter for the configuration.
// This has the second highest priority in the resolution order.
func WithConfigGetter[T any](name string, getter ConfigGetter[T]) ModuleOption {
	return func(m *ConfigurableModule) error {
		return SetConfigGetter(m, name, getter)
	}
}

// WithConfigEnvVar associates an environment variable with the configuration.
// This has the third highest priority in the resolution order.
func WithConfigEnvVar[T any](name string, envVar string) ModuleOption {
	return func(m *ConfigurableModule) error {
		return SetConfigEnvVar[T](m, name, envVar)
	}
}

// WithConfigDefault sets a default value for the configuration.
// This has the lowest priority in the resolution order.
func WithConfigDefault[T any](name string, defaultValue T) ModuleOption {
	return func(m *ConfigurableModule) error {
		return SetConfigDefault(m, name, defaultValue)
	}
}

// NewConfigurableModule returns a new instance of ConfigurableModule.
func NewConfigurableModule() *ConfigurableModule {
	return &ConfigurableModule{
		configs: make(map[string]ConfigOptionInterface),
	}
}

// NewConfigurableModuleWithOptions returns a new instance of ConfigurableModule with the provided options applied.
func NewConfigurableModuleWithOptions(options ...ModuleOption) (*ConfigurableModule, error) {
	m := NewConfigurableModule()
	for _, opt := range options {
		if err := opt(m); err != nil {
			return nil, fmt.Errorf("failed to apply module option: %w", err)
		}
	}
	return m, nil
}

// SetConfigOption registers a configuration option for the module.
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

// Initialize finalizes the module configuration and prevents further modifications.
func (m *ConfigurableModule) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, option := range m.configs {
		if option.GetName() == "" {
			option.SetName(name)
		}
		if option.IsRequired() && !option.HasValue() && !option.HasGetter() && !option.HasEnvVar() && !option.HasDefault() {
			return fmt.Errorf("%w: %s", ErrConfigRequired, option.GetName())
		}
		if err := option.Validate(); err != nil {
			return fmt.Errorf("validation failed for option '%s': %w", name, err)
		}
	}
	m.initialized = true
	return nil
}

// LoadModule returns a Starlark module loader with registered built-in functions.
func (m *ConfigurableModule) LoadModule(moduleName string, additionalFuncs starlark.StringDict) starlet.ModuleLoader {
	if err := m.Initialize(); err != nil {
		panic(err)
	}
	return func() (starlark.StringDict, error) {
		sd := starlark.StringDict{}
		for name, option := range m.configs {
			sd["set_"+name] = m.generateSetBuiltin(name, option)
			if !option.IsSecret() {
				sd["get_"+name] = m.generateGetBuiltin(name, option)
			}
		}
		for k, v := range additionalFuncs {
			sd[k] = v
		}
		return sd, nil
	}
}

// generateSetBuiltin creates a Starlark builtin for setting a configuration option.
func (m *ConfigurableModule) generateSetBuiltin(name string, option ConfigOptionInterface) starlark.Callable {
	return starlark.NewBuiltin("set_"+name, func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var v starlark.Value
		if err := starlark.UnpackArgs(b.Name(), args, kwargs, "value", &v); err != nil {
			return nil, err
		}
		if err := option.SetValueFromStarlark(v); err != nil {
			return nil, err
		}
		return starlark.None, nil
	})
}

// generateGetBuiltin creates a Starlark builtin for retrieving a configuration option.
func (m *ConfigurableModule) generateGetBuiltin(name string, option ConfigOptionInterface) starlark.Callable {
	return starlark.NewBuiltin("get_"+name, func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return option.GetStarlarkValue()
	})
}

// ListConfigs returns a map with configuration details for each option.
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

// Helper functions

// SetTypedConfigOption registers a strongly-typed configuration option.
func SetTypedConfigOption[T any](m *ConfigurableModule, name string, option *ConfigOption[T]) error {
	return m.SetConfigOption(name, option)
}

// GetConfigValue returns the value of a configuration option.
func GetConfigValue[T any](m *ConfigurableModule, name string) (T, error) {
	m.mu.RLock()
	option, exists := m.configs[name]
	m.mu.RUnlock()
	var zero T
	if !exists {
		return zero, fmt.Errorf("%w: %s", ErrConfigNotSet, name)
	}
	typedOption, ok := option.(*ConfigOption[T])
	if !ok {
		return zero, fmt.Errorf("config '%s' is not of expected type", name)
	}
	return typedOption.GetValue()
}

// GetConfigValueWithDefault returns the value of a configuration option with a default value if not found or if there's an error.
// This is a convenience function to avoid repeating the pattern of checking for errors and using defaults.
//
// Example:
//
//	// Instead of:
//	val, err := base.GetConfigValue[string](m.cfgMod, key)
//	if err != nil {
//	    val = defaultVal
//	}
//
//	// You can use:
//	val := base.GetConfigValueWithDefault(m.cfgMod, key, defaultVal)
//
// For even more convenience, use the ConfigurableModuleExt methods via the Extend() function.
func GetConfigValueWithDefault[T any](m *ConfigurableModule, name string, defaultVal T) T {
	val, err := GetConfigValue[T](m, name)
	if err != nil {
		return defaultVal
	}
	return val
}

// SetConfigValue sets the configuration value.
// This has the highest priority in the resolution order.
func SetConfigValue[T any](m *ConfigurableModule, name string, value T) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.initialized {
		return ErrModuleAlreadyInitialized
	}
	if option, exists := m.configs[name]; exists {
		typedOption, ok := option.(*ConfigOption[T])
		if !ok {
			return fmt.Errorf("cannot set value of different type for config '%s'", name)
		}
		return typedOption.SetValue(value)
	}
	newOption := NewConfigOption(value).WithName(name)
	m.configs[name] = newOption
	return nil
}

// SetConfigGetter registers a dynamic getter for the configuration.
// This has the second highest priority in the resolution order.
func SetConfigGetter[T any](m *ConfigurableModule, name string, getter ConfigGetter[T]) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.initialized {
		return ErrModuleAlreadyInitialized
	}
	if option, exists := m.configs[name]; exists {
		typedOption, ok := option.(*ConfigOption[T])
		if !ok {
			return fmt.Errorf("cannot set getter of different type for config '%s'", name)
		}
		typedOption.WithGetter(getter)
		return nil
	}
	var zero T
	newOption := NewConfigOption(zero).WithName(name).WithGetter(getter)
	m.configs[name] = newOption
	return nil
}

// SetConfigEnvVar associates an environment variable with the configuration.
// This has the third highest priority in the resolution order.
func SetConfigEnvVar[T any](m *ConfigurableModule, name string, envVar string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.initialized {
		return ErrModuleAlreadyInitialized
	}
	if option, exists := m.configs[name]; exists {
		typedOption, ok := option.(*ConfigOption[T])
		if !ok {
			return fmt.Errorf("cannot set environment variable for config of different type '%s'", name)
		}
		typedOption.WithEnvVar(envVar)
		return nil
	}
	var zero T
	newOption := NewConfigOption(zero).WithName(name).WithEnvVar(envVar)
	m.configs[name] = newOption
	return nil
}

// SetConfigDefault sets a default value for the configuration.
// This has the lowest priority in the resolution order.
func SetConfigDefault[T any](m *ConfigurableModule, name string, defaultValue T) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.initialized {
		return ErrModuleAlreadyInitialized
	}
	if option, exists := m.configs[name]; exists {
		typedOption, ok := option.(*ConfigOption[T])
		if !ok {
			return fmt.Errorf("cannot set default value of different type for config '%s'", name)
		}
		typedOption.WithDefault(defaultValue)
		return nil
	}
	newOption := NewConfigOption(defaultValue).WithName(name)
	m.configs[name] = newOption
	return nil
}
