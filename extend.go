package base

// ConfigurableModuleExt provides extension methods for ConfigurableModule.
// It offers convenient access to configuration values with built-in fallback handling,
// reducing boilerplate code in modules that use ConfigurableModule.
type ConfigurableModuleExt struct {
	*ConfigurableModule
}

// Extend returns an extension wrapper for ConfigurableModule that provides additional convenience methods.
// This helps reduce boilerplate code when working with configuration values that need fallback handling.
func (m *ConfigurableModule) Extend() *ConfigurableModuleExt {
	return &ConfigurableModuleExt{m}
}

// GetString retrieves a string configuration value with an optional fallback value.
// If the configuration doesn't exist or there's an error retrieving it, the fallback value is returned.
// This is a convenience method replacing the common pattern of checking for errors when retrieving config values.
func (e *ConfigurableModuleExt) GetString(key string, fallbackVal ...string) string {
	var fallback string
	if len(fallbackVal) > 0 {
		fallback = fallbackVal[0]
	}
	return GetConfigValueWithFallback(e.ConfigurableModule, key, fallback)
}

// GetInt retrieves an int configuration value with an optional fallback value.
// If the configuration doesn't exist or there's an error retrieving it, the fallback value is returned.
// This provides the same convenience as GetString but for int values.
func (e *ConfigurableModuleExt) GetInt(key string, fallbackVal ...int) int {
	var fallback int
	if len(fallbackVal) > 0 {
		fallback = fallbackVal[0]
	}
	return GetConfigValueWithFallback(e.ConfigurableModule, key, fallback)
}

// GetUint retrieves an uint configuration value with an optional fallback value.
// If the configuration doesn't exist or there's an error retrieving it, the fallback value is returned.
// This provides the same convenience as GetString but for unsigned integer values.
func (e *ConfigurableModuleExt) GetUint(key string, fallbackVal ...uint) uint {
	var fallback uint
	if len(fallbackVal) > 0 {
		fallback = fallbackVal[0]
	}
	return GetConfigValueWithFallback(e.ConfigurableModule, key, fallback)
}

// GetBool retrieves a bool configuration value with an optional fallback value.
// If the configuration doesn't exist or there's an error retrieving it, the fallback value is returned.
// This provides the same convenience as GetString but for boolean values.
func (e *ConfigurableModuleExt) GetBool(key string, fallbackVal ...bool) bool {
	var fallback bool
	if len(fallbackVal) > 0 {
		fallback = fallbackVal[0]
	}
	return GetConfigValueWithFallback(e.ConfigurableModule, key, fallback)
}

// GetFloat retrieves a float64 configuration value with an optional fallback value.
// If the configuration doesn't exist or there's an error retrieving it, the fallback value is returned.
// This provides the same convenience as GetString but for float64 values.
func (e *ConfigurableModuleExt) GetFloat(key string, fallbackVal ...float64) float64 {
	var fallback float64
	if len(fallbackVal) > 0 {
		fallback = fallbackVal[0]
	}
	return GetConfigValueWithFallback(e.ConfigurableModule, key, fallback)
}
