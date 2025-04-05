// Package base provides a generic base module that can be extended with different configurations.
package base

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
)

// ConfigurableModule provides a generic base module that can be extended with different configurations.
type ConfigurableModule[T any] struct {
	mu          sync.RWMutex
	initialized bool
	configs     map[string]*ConfigOption[T]
}

// NewConfigurableModule creates a new instance of ConfigurableModule.
func NewConfigurableModule[T any]() *ConfigurableModule[T] {
	return &ConfigurableModule[T]{
		configs: make(map[string]*ConfigOption[T]),
	}
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
			if !option.hasSetValue() && !option.hasGetter() {
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

	return option.getValue()
}

// InternalGetConfig retrieves a configuration value, including secrets, for internal use only.
func (m *ConfigurableModule[T]) InternalGetConfig(name string) (T, error) {
	var zero T

	m.mu.RLock()
	option, exists := m.configs[name]
	if !exists {
		m.mu.RUnlock()
		return zero, fmt.Errorf("%w: %s", ErrConfigNotSet, name)
	}
	m.mu.RUnlock()

	return option.getSecretValue(), nil
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
			"has_value":   option.hasSetValue(),
			"has_getter":  option.hasGetter(),
		}

		// Only include values for non-secret configs
		if !option.IsSecret {
			val, err := option.getValue()
			if err == nil {
				info["value"] = val
			}
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

	// Add setter functions for all configs and getter functions only for non-secret configs
	for name, option := range m.configs {
		sd["set_"+name] = m.genSetConfig(name)

		// Only add getter functions for non-secret configs
		if !option.IsSecret {
			sd["get_"+name] = m.genGetConfig(name)
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
