# 🧱 `base` Module for Starlark Extensions

[![godoc](https://pkg.go.dev/badge/github.com/starpkg/base.svg)](https://pkg.go.dev/github.com/starpkg/base)
[![licenese](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

A generic base module that bridges the gap between online services, external libraries, and the [Starlark](https://github.com/google/starlark-go) runtime. This module allows you to create configurable Starlark modules by extending its functionality with different configurations.

## Features

- **Flexible Configuration**: Rich configuration options with validation, dynamic getters, and environment variable support
- **Type Safety**: Fully leverages Go generics for type-safe configurations
- **Comprehensive Validation**: Support for custom validation rules for configuration values
- **Starlark Integration**: Exposes getter/setter functions to Starlark scripts
- **Secret Handling**: Special handling for sensitive configuration values (not exposable to Starlark)
- **Thread Safety**: Concurrency-safe operations for all configuration access
- **Environment Variable Support**: Override configs via environment variables
- **Value Priority**: Clear precedence between values, getters, environment variables, and defaults

## Installation

To install the module, run:

```bash
go get github.com/starpkg/base
```

## Value Resolution Priority

The `ConfigOption` type uses a clear, predictable resolution strategy when multiple sources for a value exist:

1. **Immediate value** (highest priority): Values explicitly set via `SetValue()` or `WithValue()` methods
2. **Getter function**: Dynamic values provided by a function set with `WithGetter()`
3. **Environment variable**: Values from environment variables specified with `WithEnvVar()`
4. **Default value** (lowest priority): The default value provided when creating the option

This priority order means:
- You can override any value by explicitly setting it, regardless of other sources
- Dynamic getters provide values only when no explicit value is set
- Environment variables can override default values but are overridden by explicit values and getters
- Default values are used only as a last resort when no other source provides a value

For example:
```go
// Create an option with default value "production"
option := NewConfigOption("production").
    WithEnvVar("APP_ENV").           // Can be overridden by APP_ENV environment variable
    WithGetter(func() string {       // Will override env var but not explicit values
        return getEnvironmentFromMetadata()
    }).
    WithValue("development")         // Explicitly set - overrides all other sources
```

In this example, the "development" value will be used regardless of the environment variable or getter function.

## Usage

Here's how you can use the `ConfigurableModule` to create custom Starlark modules:

```go
package main

import (
    "fmt"
    "errors"

    "github.com/starpkg/base"
    "github.com/1set/starlet"
    "go.starlark.net/starlark"
)

func main() {
    // Create a new configurable module
    cm := base.NewConfigurableModule()

    // Create a configuration option with validation and description
    apiKeyOption := base.NewConfigOption("").
        WithName("api_key").
        WithDescription("API key for authentication with the service").
        WithValidator(func(value string) error {
            if len(value) < 10 {
                return errors.New("API key must be at least 10 characters")
            }
            return nil
        }).
        SetRequired(true).
        SetSecret(true)  // Secret configs won't be exposable via get_* functions

    // Register the configuration option
    cm.SetConfigOption("api_key", apiKeyOption)

    // Create an endpoint configuration with a default value
    endpointOption := base.NewConfigOption("https://api.example.com").
        WithName("endpoint").
        WithDescription("API endpoint URL for service connection").
        WithEnvVar("API_ENDPOINT")  // Can be overridden by environment variable

    // Register the endpoint configuration
    cm.SetConfigOption("endpoint", endpointOption)
    
    // Create a dynamic configuration option that gets current timestamp
    timestampOption := base.NewConfigOption("").
        WithName("timestamp").
        WithDescription("Current server timestamp").
        WithGetter(func() string {
            return time.Now().Format(time.RFC3339)
        })

    cm.SetConfigOption("timestamp", timestampOption)

    // Set a value for the API key
    base.SetConfigValue(cm, "api_key", "your-api-key-here")

    // Add additional functions if needed
    additionalFuncs := starlark.StringDict{
        "do_something": starlark.NewBuiltin("do_something", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
            // We can access config values in our Go code
            apiKey, err := base.GetConfigValue[string](cm, "api_key")
            if err != nil {
                return starlark.None, err
            }
            endpoint, err := base.GetConfigValue[string](cm, "endpoint")
            if err != nil {
                return starlark.None, err
            }
            fmt.Printf("API Key: %s\n", apiKey)
            fmt.Printf("Endpoint: %s\n", endpoint)
            // Your implementation here
            return starlark.None, nil
        }),
    }

    // Load the module
    loader := cm.LoadModule("mymodule", additionalFuncs)

    // Use the module with Starlet or any Starlark interpreter
    script := `
load("mymodule", "set_api_key", "get_endpoint", "get_timestamp", "do_something")

# Set configuration values
set_api_key("new-api-key-1234567890")

# Get configuration values (note that get_api_key is NOT available as it's secret)
endpoint = get_endpoint()
print("Using endpoint:", endpoint)

# Get dynamic configuration that always returns the latest value
timestamp = get_timestamp()
print("Current timestamp:", timestamp)

# Use the custom function
do_something()
`

    // Run the script with the module loader ...
}
```

## File Structure

The package is organized into separate files for better maintainability:

- `errors.go` - Common error definitions
- `config.go` - Configuration option types and methods
- `module.go` - Main module implementation and Starlark integration

## Documentation

### Core Types

#### `ConfigOption[T]`

A generic configuration option providing validation, metadata, and special behaviors:

```go
type ConfigOption[T any] struct {
    // Configuration metadata
    Name        string // Unique identifier for this configuration
    Description string // Human-readable description of the configuration
    EnvVar      string // Environment variable name for overriding the configuration
    
    // Internal fields
    // defaultVal, value, hasValue, getter, validator, isRequired, isSecret
}
```

#### `ConfigOptionInterface`

A common interface implemented by all configuration options for use by the module:

```go
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
```

#### `ConfigurableModule`

A module that can be extended with various configuration options:

```go
type ConfigurableModule struct {
    // Internal fields - all private
}
```

### Config Option Creation and Configuration

- `NewConfigOption[T](defaultValue T) *ConfigOption[T]`: Creates a new configuration option with a default value.

- `WithName(name string) *ConfigOption[T]`: Sets the name of the configuration option.

- `WithDescription(desc string) *ConfigOption[T]`: Adds a description to the configuration option.

- `WithEnvVar(envVar string) *ConfigOption[T]`: Specifies an environment variable name to check for this configuration.

- `WithValue(value T) *ConfigOption[T]`: Sets the value of the configuration option.

- `WithValidator(validator ConfigValidator[T]) *ConfigOption[T]`: Adds a validator to verify configuration values.

- `WithGetter(getter ConfigGetter[T]) *ConfigOption[T]`: Adds a dynamic getter function for the configuration.

- `SetRequired(required bool) *ConfigOption[T]`: Sets whether the configuration option is required.

- `SetSecret(secret bool) *ConfigOption[T]`: Sets whether the configuration option is secret (sensitive).

### Module Creation and Configuration

- `NewConfigurableModule() *ConfigurableModule`: Creates a new module instance.

- `NewConfigurableModuleWithOptions(options ...ModuleOption) (*ConfigurableModule, error)`: Creates a new module with the provided options applied.

- `WithConfigOption(name string, option ConfigOptionInterface) ModuleOption`: Module option that registers a configuration option.

- `WithTypedConfigOption[T any](name string, option *ConfigOption[T]) ModuleOption`: Module option that registers a strongly-typed configuration option.

- `WithConfigValue[T any](name string, value T) ModuleOption`: Module option that sets a configuration value directly.

- `WithConfigGetter[T any](name string, getter ConfigGetter[T]) ModuleOption`: Module option that registers a dynamic getter for the configuration.

- `WithConfigEnvVar[T any](name string, envVar string) ModuleOption`: Module option that associates an environment variable with the configuration.

### Runtime Operations

- `SetConfigOption(name string, option ConfigOptionInterface) error`: Registers a configuration option.

- `Initialize() error`: Finalizes the module configuration and verifies required values.

- `ListConfigs() map[string]map[string]interface{}`: Returns information about all configurations.

- `GetConfigOption(name string) (ConfigOptionInterface, error)`: Retrieves a configuration option by name.

- `GetConfigValue[T any](m *ConfigurableModule, name string) (T, error)`: Helper function to retrieve a typed configuration value.

- `SetConfigValue[T any](m *ConfigurableModule, name string, value T) error`: Helper function to set a typed configuration value.

- `LoadModule(moduleName string, additionalFuncs starlark.StringDict) starlet.ModuleLoader`: Creates a Starlark module.

### Starlark Functions

When you load the module in Starlark, these functions are automatically provided:

- `set_<config_name>(value)`: Sets a configuration value (available for all configs).
- `get_<config_name>()`: Gets a configuration value (only available for non-secret configs).

#### Example

```python
load("mymodule", "set_api_key", "get_endpoint", "get_timestamp", "do_something")

# Set configuration (both secret and non-secret configs can be set)
set_api_key("new-api-key")

# Get configuration (only non-secret configs can be retrieved)
endpoint = get_endpoint()
print("Using endpoint:", endpoint)

# Get dynamic configuration that always returns the latest value
timestamp = get_timestamp()
print("Current timestamp:", timestamp)

# Use the module functionality
do_something()
```

## Secret Configuration Handling

Configurations marked as secret using `SetSecret(true)` are handled differently:

1. Secret configs can be set from both Go code and Starlark scripts.
2. Secret configs cannot be accessed from Starlark code - no `get_<name>` function is exposed.
3. Secret config values are not shown in `ListConfigs()` output.
4. Go code can still access secret configs using `GetConfigValue`.

This design ensures that sensitive information like API keys can be set by Starlark scripts but cannot be accidentally leaked or exposed.

## Environment Variable Integration

Any configuration option can be tied to an environment variable using `WithEnvVar`. The system will:

1. Try to convert the environment variable string to the target type
2. Support common string formats for booleans (true/false, yes/no, 1/0, on/off)
3. Convert numeric types appropriately (int, float, etc.)
4. Parse JSON for complex types like slices and maps

## Contributing

Contributions are welcome! Please open an issue or submit a pull request if you have any improvements or suggestions.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
