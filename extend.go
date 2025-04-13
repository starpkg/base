package base

// ConfigurableModuleExt provides extension methods for ConfigurableModule.
// It offers convenient access to configuration values with built-in default handling,
// reducing boilerplate code in modules that use ConfigurableModule.
type ConfigurableModuleExt struct {
	*ConfigurableModule
}

// Extend returns an extension wrapper for ConfigurableModule that provides additional convenience methods.
// This helps reduce boilerplate code when working with configuration values that need default handling.
func (m *ConfigurableModule) Extend() *ConfigurableModuleExt {
	return &ConfigurableModuleExt{m}
}

// GetString retrieves a string configuration value with an optional default value.
// If the configuration doesn't exist or there's an error retrieving it, the default value is returned.
// This is a convenience method replacing the common pattern of checking for errors when retrieving config values.
func (e *ConfigurableModuleExt) GetString(key string, defaultVal ...string) string {
	var defVal string
	if len(defaultVal) > 0 {
		defVal = defaultVal[0]
	}
	return GetConfigValueWithFallback(e.ConfigurableModule, key, defVal)
}

// GetInt retrieves an int configuration value with an optional default value.
// If the configuration doesn't exist or there's an error retrieving it, the default value is returned.
// This provides the same convenience as GetString but for int values.
func (e *ConfigurableModuleExt) GetInt(key string, defaultVal ...int) int {
	var defVal int
	if len(defaultVal) > 0 {
		defVal = defaultVal[0]
	}
	return GetConfigValueWithFallback(e.ConfigurableModule, key, defVal)
}

// GetUint retrieves an uint configuration value with an optional default value.
// If the configuration doesn't exist or there's an error retrieving it, the default value is returned.
// This provides the same convenience as GetString but for unsigned integer values.
func (e *ConfigurableModuleExt) GetUint(key string, defaultVal ...uint) uint {
	var defVal uint
	if len(defaultVal) > 0 {
		defVal = defaultVal[0]
	}
	return GetConfigValueWithFallback(e.ConfigurableModule, key, defVal)
}

// GetBool retrieves a bool configuration value with an optional default value.
// If the configuration doesn't exist or there's an error retrieving it, the default value is returned.
// This provides the same convenience as GetString but for boolean values.
func (e *ConfigurableModuleExt) GetBool(key string, defaultVal ...bool) bool {
	var defVal bool
	if len(defaultVal) > 0 {
		defVal = defaultVal[0]
	}
	return GetConfigValueWithFallback(e.ConfigurableModule, key, defVal)
}

// GetFloat retrieves a float64 configuration value with an optional default value.
// If the configuration doesn't exist or there's an error retrieving it, the default value is returned.
// This provides the same convenience as GetString but for float64 values.
func (e *ConfigurableModuleExt) GetFloat(key string, defaultVal ...float64) float64 {
	var defVal float64
	if len(defaultVal) > 0 {
		defVal = defaultVal[0]
	}
	return GetConfigValueWithFallback(e.ConfigurableModule, key, defVal)
}
