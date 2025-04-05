# 🧱 `base` Module for Starlark Extensions

[![godoc](https://pkg.go.dev/badge/github.com/starpkg/base.svg)](https://pkg.go.dev/github.com/starpkg/base)
[![licenese](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

A generic base module that bridges the gap between online services, external libraries, and the [Starlark](https://github.com/google/starlark-go) runtime. This module allows you to create configurable Starlark modules by extending its functionality with different configurations.

## Features

- **Flexible Configuration**: Rich configuration options with validation, dynamic getters, and metadata
- **Type Safety**: Fully leverages Go generics for type-safe configurations
- **Comprehensive Validation**: Support for custom validation rules for configuration values
- **Starlark Integration**: Exposes getter/setter functions to Starlark scripts
- **Secret Handling**: Special handling for sensitive configuration values (not exposable to Starlark)
- **Thread Safety**: Concurrency-safe operations for all configuration access
- **Modular Structure**: Organized into separate files for better maintainability
- **Proper Encapsulation**: Well-defined public API with private implementation details

## Installation

To install the module, run:

```bash
go get github.com/starpkg/base
```

## Usage

Here's how you can use the enhanced `ConfigurableModule` to create custom Starlark modules:

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
    cm := base.NewConfigurableModule[string]()

    // Create a configuration option with validation and description
    apiKeyOption := base.NewConfigOption("").
        WithDescription("API key for authentication with the service").
        WithValidator(func(value string) error {
            if len(value) < 10 {
                return errors.New("API key must be at least 10 characters")
            }
            return nil
        }).
        Required().
        Secret()  // Secret configs won't be exposable via get_* functions

    // Register the configuration option
    cm.SetConfigOption("api_key", apiKeyOption)

    // Create an endpoint configuration with a default value
    endpointOption := base.NewConfigOption("https://api.example.com").
        WithDescription("API endpoint URL for service connection")

    // Register the endpoint configuration
    cm.SetConfigOption("endpoint", endpointOption)

    // Set a value for the API key
    cm.SetConfigValue("api_key", "your-api-key-here")

    // Add additional functions if needed
    additionalFuncs := starlark.StringDict{
        "do_something": starlark.NewBuiltin("do_something", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
            // We can use InternalGetConfig to get secret values within Go code
            apiKey, err := cm.InternalGetConfig("api_key")
            if err != nil {
                return starlark.None, err
            }
            endpoint, err := cm.GetConfig("endpoint")
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
load("mymodule", "set_api_key", "get_endpoint", "list_configs", "do_something")

# Set configuration values
set_api_key("new-api-key-1234567890")

# Get configuration values (note that get_api_key is NOT available as it's secret)
endpoint = get_endpoint()
print("Using endpoint:", endpoint)

# Display all configurations (secret values won't be shown)
configs = list_configs()
for name, config in configs.items():
    print(f"Config {name}: {config}")

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

A rich configuration option that provides validation, metadata, and special behaviors. Created using the builder pattern with `NewConfigOption` and its associated methods.

```go
type ConfigOption[T any] struct {
    // Public fields
    Default     T      // Default value for the configuration
    Description string // Human-readable description, used for documentation and UI
    
    // Private fields - accessed through methods
    // getter, validator, isRequired, isSecret, etc.
}
```

The `Description` field plays an important role in the configuration system:

- It provides human-readable documentation about the purpose of the configuration
- It is displayed in the output of the `list_configs()` Starlark function
- It should be clear and concise, explaining what the configuration is used for
- It should include information about expected format or values when relevant

#### `ConfigurableModule[T]`

A generic module that can be extended with various configuration options.

```go
type ConfigurableModule[T any] struct {
    // Internal fields - all private
}
```

### Config Option Creation and Configuration

- `NewConfigOption[T](defaultValue T) *ConfigOption[T]`: Creates a new configuration option with a default value.

- `WithDescription(desc string) *ConfigOption[T]`: Adds a description to the configuration option. Description should clearly explain the purpose and expected format of the configuration value.

- `WithValidator(validator ConfigValidator[T]) *ConfigOption[T]`: Adds a validator to verify configuration values.

- `WithGetter(getter ConfigGetter[T]) *ConfigOption[T]`: Adds a dynamic getter function for the configuration.

- `Required() *ConfigOption[T]`: Marks the configuration option as required.

- `Secret() *ConfigOption[T]`: Marks the configuration option as a secret (sensitive).

### Module Configuration

- `NewConfigurableModule[T]() *ConfigurableModule[T]`: Creates a new module instance.

- `SetConfigOption(name string, option *ConfigOption[T]) error`: Registers a configuration option.

- `SetConfig(name string, getter ConfigGetter[T]) error`: Sets a dynamic getter function.

- `SetConfigValue(name string, value T) error`: Sets a direct configuration value.

- `Initialize() error`: Finalizes the module configuration and verifies required values.

### Runtime Operations

- `GetConfig(name string) (T, error)`: Retrieves a configuration value (non-secret only).

- `InternalGetConfig(name string) (T, error)`: Retrieves any configuration value, including secrets (for internal Go code only).

- `ListConfigs() map[string]map[string]interface{}`: Returns information about all configurations.

- `LoadModule(moduleName string, additionalFuncs starlark.StringDict) starlet.ModuleLoader`: Creates a Starlark module.

### Starlark Functions

When you load the module in Starlark, these functions are automatically provided:

- `set_<config_name>(value)`: Sets a configuration value (available for all configs).
- `get_<config_name>()`: Gets a configuration value (only available for non-secret configs).
- `list_configs()`: Returns information about all configurations (values hidden for secret configs).

#### Example

```python
load("mymodule", "set_api_key", "get_endpoint", "list_configs", "do_something")

# Set configuration (both secret and non-secret configs can be set)
set_api_key("new-api-key")

# Get configuration (only non-secret configs can be retrieved)
endpoint = get_endpoint()
print("Using endpoint:", endpoint)

# Inspect configurations (secret values will be hidden)
configs = list_configs()
for name, info in configs.items():
    print(f"Config {name}: {info['description']}")

# Use the module functionality
do_something()
```

## Secret Configuration Handling

Configurations marked as `Secret()` are handled differently:

1. Secret configs can be set from both Go code and Starlark scripts using `SetConfigValue` and `set_<name>` functions.
2. Secret configs cannot be accessed directly from Starlark code - no `get_<name>` function is exposed.
3. Secret configs are not displayed in the `list_configs()` output values.
4. Go code can access secret configs internally using the `InternalGetConfig` method.

This design ensures that sensitive information like API keys can be set by Starlark scripts but cannot be accidentally leaked or exposed.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request if you have any improvements or suggestions.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
