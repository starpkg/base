# 🧱 `base` Module for Starlark Extensions

[![godoc](https://pkg.go.dev/badge/github.com/starpkg/base.svg)](https://pkg.go.dev/github.com/starpkg/base)
[![licenese](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

A generic base module that bridges the gap between online services, external libraries, and the [Starlark](https://github.com/google/starlark-go) runtime. This module allows you to create configurable Starlark modules by extending its functionality with different configurations.

## Features

- **Flexible Configuration**: Rich configuration options with validation, dynamic getters, and metadata
- **Type Safety**: Fully leverages Go generics for type-safe configurations
- **Comprehensive Validation**: Support for custom validation rules for configuration values
- **Starlark Integration**: Exposes getter/setter functions to Starlark scripts
- **Secret Handling**: Special handling for sensitive configuration values
- **Thread Safety**: Concurrency-safe operations for all configuration access

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
        WithDescription("API key for authentication").
        WithValidator(func(value string) error {
            if len(value) < 10 {
                return errors.New("API key must be at least 10 characters")
            }
            return nil
        }).
        Required().
        Secret()

    // Register the configuration option
    cm.SetConfigOption("api_key", apiKeyOption)

    // Create an endpoint configuration with a default value
    endpointOption := base.NewConfigOption("https://api.example.com").
        WithDescription("API endpoint URL")

    // Register the endpoint configuration
    cm.SetConfigOption("endpoint", endpointOption)

    // Set a value for the API key
    cm.SetConfigValue("api_key", "your-api-key-here")

    // Add additional functions if needed
    additionalFuncs := starlark.StringDict{
        "do_something": starlark.NewBuiltin("do_something", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
            apiKey, err := cm.GetConfig("api_key")
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

# Get configuration values
endpoint = get_endpoint()
print("Using endpoint:", endpoint)

# Display all configurations
configs = list_configs()
for name, config in configs.items():
    if not config["secret"]:
        print(f"Config {name}: {config}")

# Use the custom function
do_something()
`

    // Run the script with the module loader ...
}
```

## Documentation

### Core Types

#### `ConfigOption[T]`

A rich configuration option that provides validation, metadata, and special behaviors.

```go
type ConfigOption[T any] struct {
    Default     T                   // Default value
    Getter      ConfigGetter[T]     // Dynamic value getter
    Validator   ConfigValidator[T]  // Value validator
    Description string              // Human-readable description
    IsRequired  bool                // Whether the config is required
    IsSecret    bool                // Whether the config is sensitive
    // ... internal fields
}
```

#### `ConfigurableModule[T]`

A generic module that can be extended with various configuration options.

```go
type ConfigurableModule[T any] struct {
    // ... internal fields
}
```

### Config Option Creation and Configuration

- `NewConfigOption[T](defaultValue T) *ConfigOption[T]`: Creates a new configuration option with a default value.

- `WithDescription(desc string) *ConfigOption[T]`: Adds a description to the configuration option.

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

- `GetConfig(name string) (T, error)`: Retrieves a configuration value.

- `ListConfigs() map[string]map[string]interface{}`: Returns information about all configurations.

- `LoadModule(moduleName string, additionalFuncs starlark.StringDict) starlet.ModuleLoader`: Creates a Starlark module.

### Starlark Functions

When you load the module in Starlark, these functions are automatically provided:

- `set_<config_name>(value)`: Sets a configuration value.
- `get_<config_name>()`: Gets a configuration value.
- `list_configs()`: Returns information about all configurations.

#### Example

```python
load("mymodule", "set_api_key", "get_endpoint", "list_configs", "do_something")

# Set configuration
set_api_key("new-api-key")

# Get configuration
endpoint = get_endpoint()
print("Using endpoint:", endpoint)

# Inspect configurations
configs = list_configs()
for name, info in configs.items():
    print(f"Config {name}: {info['description']}")

# Use the module functionality
do_something()
```

## Contributing

Contributions are welcome! Please open an issue or submit a pull request if you have any improvements or suggestions.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
