// Package base provides a generic base module that can be extended with different configurations.
package base

import (
	"errors"
	"fmt"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv"
	"go.starlark.net/starlark"
)

// ConfigGetter is a function type that returns a value of type T.
type ConfigGetter[T any] func() T

// ConfigurableModule provides a generic base module that can be extended with different configurations.
type ConfigurableModule[T any] struct {
	configs map[string]ConfigGetter[T]
}

// NewConfigurableModule creates a new instance of ConfigurableModule.
func NewConfigurableModule[T any]() *ConfigurableModule[T] {
	return &ConfigurableModule[T]{configs: make(map[string]ConfigGetter[T])}
}

// SetConfig sets a configuration getter for a given name.
func (m *ConfigurableModule[T]) SetConfig(name string, getter ConfigGetter[T]) {
	m.configs[name] = getter
}

// SetConfigValue sets a configuration value for a given name.
func (m *ConfigurableModule[T]) SetConfigValue(name string, value T) {
	m.configs[name] = func() T { return value }
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
		m.configs[name] = func() T { return vt }
		return starlark.None, nil
	})
}

var (
	// ErrConfigNotSet is the error when the configuration is not set.
	ErrConfigNotSet = errors.New("config not set")
)

// GetConfig retrieves the configuration value for a given name.
func (m *ConfigurableModule[T]) GetConfig(name string) (T, error) {
	getter, exists := m.configs[name]
	if !exists || getter == nil {
		var zero T
		return zero, fmt.Errorf("%w: %s", ErrConfigNotSet, name)
	}
	return getter(), nil
}

// LoadModule returns a Starlark module loader with the given configurations and additional functions.
func (m *ConfigurableModule[T]) LoadModule(moduleName string, additionalFuncs starlark.StringDict) starlet.ModuleLoader {
	sd := starlark.StringDict{}
	for name := range m.configs {
		sd["set_"+name] = m.genSetConfig(name)
	}
	for k, v := range additionalFuncs {
		sd[k] = v
	}
	return dataconv.WrapModuleData(moduleName, sd)
}
