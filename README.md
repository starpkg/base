# 🧱 `base` - Configurable Starlark Module Foundation

[![godoc](https://pkg.go.dev/badge/github.com/starpkg/base.svg)](https://pkg.go.dev/github.com/starpkg/base)
[![licenese](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![codecov](https://codecov.io/gh/starpkg/base/graph/badge.svg)](https://codecov.io/gh/starpkg/base)

A typed, configurable foundation for building Starlark modules that connect to online services and external libraries.

> **Where this sits.** The `starpkg` ecosystem exists to give Starlark scripts *support for the necessary **local** operations + simple abstractions over common **online** services, for ease of use.* `base` is the layer beneath all of that: it is not itself a local capability or an online-service binding, but the **shared plumbing** every domain module (`sqlite`, `web`, `llm`, `mq`, `s3`, `email`, …) uses to declare its typed configuration, resolve it from values / getters / environment / defaults, keep secrets out of scripts, and surface the result to Starlark. It is an **L4 starpkg** module that depends downward on `1set/starlet` (the Machine + `dataconv`) and transitively on `1set/starlight` + `go.starlark.net`; nothing in the ecosystem sits below it except those runtimes.

## Overview

The `base` package provides a framework for creating Starlark modules with:

- **Type-safe configuration** using Go generics
- **Multiple configuration sources** with clear precedence rules
- **Secret value handling** for sensitive data
- **Environment variable integration** for flexible deployment
- **Full Starlark integration** with automatic getter/setter generation

## Quick Start

```go
package main

import (
    "github.com/starpkg/base"
    "github.com/1set/starlet"
)

func main() {
    // 1. Create a new module
    module := base.NewConfigurableModule()
    
    // 2. Define configuration options
    module.SetConfigOption("api_key", 
        base.NewConfigOption("").
            WithDescription("API key for authentication").
            SetSecret(true))
            
    module.SetConfigOption("endpoint", 
        base.NewConfigOption("https://api.example.com").
            WithEnvVar("API_ENDPOINT"))
            
    // 3. Load the module with additional functions
    loader := module.LoadModule("mymodule", nil)

    // 4. Run Starlark code with the module
    machine := starlet.NewDefault()
    machine.SetLazyloadModules(map[string]starlet.ModuleLoader{"mymodule": loader})
    machine.SetScriptContent([]byte(`
load("mymodule", "set_api_key", "get_endpoint")

set_api_key("my-secret-key")
print("Using endpoint:", get_endpoint())
`))
    if _, err := machine.Run(); err != nil {
        panic(err)
    }
}
```

## Installation

```bash
go get github.com/starpkg/base
```

## Key Concepts

### Configuration Value Resolution

The system uses a clear priority order when resolving configuration values:

1. **Explicit values** (highest): Values set via `SetValue()` or `WithValue()`
2. **Dynamic getters**: Values from functions set with `WithGetter()`
3. **Environment variables**: Values from environment variables set with `WithEnvVar()`
4. **Default values** (lowest): The value provided when creating the option

```go
// Examples of the priority system in action:
option := base.NewConfigOption("default-value").     // Priority 4 (lowest)
    WithEnvVar("CONFIG_VAR").                        // Priority 3
    WithGetter(func() string { return "dynamic" }).  // Priority 2
    WithValue("explicit-value")                      // Priority 1 (highest)

// You can also update the default value after creation:
option := base.NewConfigOption("original-default").
    WithDefault("new-default-value")  // Replace the default value
```

### Secret Configuration

Mark sensitive configurations as secret to prevent exposure in Starlark:

```go
// In Go setup code:
apiKeyOption := base.NewConfigOption("").SetSecret(true)
module.SetConfigOption("api_key", apiKeyOption)

// In Starlark code:
load("mymodule", "set_api_key")  # No get_api_key is exposed
set_api_key("my-secret-key")     # Can set the value
```

Key behaviors:
- Secret values can be set from both Go and Starlark
- No `get_` function is exposed to Starlark for secret configs
- Values are hidden in `ListConfigs()` output
- Go code can still access values using `GetConfigValue`

### Environment Variables

Tie configurations to environment variables for flexible deployment:

```go
// Define configuration with environment variable
option := base.NewConfigOption("default").WithEnvVar("API_ENDPOINT")

// At runtime, if API_ENDPOINT environment variable exists, it will
// override the default value
```

The system automatically converts environment variable string values to the appropriate type:
- Boolean values: true/false, yes/no, 1/0, on/off
- Numeric types: converted to int, float, etc.
- Complex types: parsed from JSON for slices and maps

## Detailed Usage

### Creating a Configurable Module

```go
// Basic creation
module := base.NewConfigurableModule()

// Using functional options
module, err := base.NewConfigurableModuleWithOptions(
    base.WithConfigValue("timeout", 30),
    base.WithConfigDefault("retry_count", 3),
    base.WithConfigOption("api_key", apiKeyOption),
    base.WithTypedConfigOption("rate_limit", rateOption),
)
```

### Configuration Options

Create and configure typed options with builder pattern:

```go
// String option with validation
apiKeyOption := base.NewConfigOption("").
    WithName("api_key").
    WithDescription("API key for service authentication").
    WithValue("initial-key").         // Highest priority
    WithGetter(fetchKeyFromVault).    // Second priority
    WithEnvVar("API_KEY").            // Third priority
    WithDefault("dev-key").           // Lowest priority
    WithValidator(func(val string) error {
        if len(val) < 10 {
            return errors.New("API key too short")
        }
        return nil
    }).
    SetRequired(true)

// Integer option with environment variable
timeoutOption := base.NewConfigOption(30).
    WithName("timeout").
    WithDescription("Request timeout in seconds").
    WithEnvVar("API_TIMEOUT")
    
// Dynamic option with getter function
timestampOption := base.NewConfigOption("").
    WithName("timestamp").
    WithDescription("Current server timestamp").
    WithGetter(func() string {
        return time.Now().Format(time.RFC3339)
    })
```

### Using Modules in Starlark

When loaded, your module automatically exposes functions:

```python
# Load the module
load("mymodule", "set_api_key", "get_timeout", "get_timestamp")

# Set values
set_api_key("my-secret-key")  # Sets the api_key configuration

# Get values (only available for non-secret configs)
timeout = get_timeout()
timestamp = get_timestamp()
```

### Generated Starlark API surface

`base` defines **no fixed-name builtins of its own**. Instead, `LoadModule(moduleName, additionalFuncs)` derives the script-facing surface from the options you registered, so the exact function names depend on your config keys. For every registered config option named `<name>`, the loader generates:

| Generated builtin | Signature | Behaviour |
|---|---|---|
| `set_<name>` | `set_<name>(value)` | Sets the option from a single positional `value`. The value is converted from Starlark to Go via `dataconv.Unmarshal`; numeric/slice/map coercion is applied to match the option's Go type. Returns `None`; a type mismatch (or `None`/`nil`) raises a script error rather than crashing the host. |
| `get_<name>` | `get_<name>()` | Returns the option's resolved value (per the priority order), marshalled back to a Starlark value. **Only generated for non-secret options** — a `SetSecret(true)` option exposes its `set_<name>` but no `get_<name>`. |

So a module with options `api_key` (secret) and `endpoint` exposes exactly `set_api_key`, `set_endpoint`, and `get_endpoint` — note the `set_`-prefixed setter for every option and the `get_`-prefixed getter for every non-secret option. Any callables you pass in `additionalFuncs` (a `starlark.StringDict`) are merged in alongside these under their own names.

```python
load("mymodule", "set_api_key", "set_endpoint", "get_endpoint")

set_api_key("sk-...")              # secret: settable, but no get_api_key exists
set_endpoint("https://api.prod")   # generated set_ builtin
print(get_endpoint())              # generated get_ builtin (non-secret only)
```

> The doc-coverage gate (`doccov`) sees these as the `set_` and `get_` builtins because their names are built by string concatenation (`"set_" + name`). They are documented above as the `set_<name>` / `get_<name>` families.

### Complete Example

```go
package main

import (
    "fmt"

    "github.com/starpkg/base"
    "github.com/1set/starlet"
    "go.starlark.net/starlark"
)

func main() {
    // Create a new module
    module := base.NewConfigurableModule()

    // Add configurations
    module.SetConfigOption("api_key", 
        base.NewConfigOption("").
            WithDescription("API key for service").
            SetRequired(true).
            SetSecret(true))
            
    module.SetConfigOption("endpoint", 
        base.NewConfigOption("https://api.example.com").
            WithDescription("API endpoint URL").
            WithEnvVar("API_ENDPOINT"))
            
    module.SetConfigOption("timeout", 
        base.NewConfigOption(30).
            WithDescription("Request timeout in seconds"))
    
    // Add custom functions to the module
    additionalFuncs := starlark.StringDict{
        "make_request": starlark.NewBuiltin("make_request", func(
            thread *starlark.Thread, 
            b *starlark.Builtin, 
            args starlark.Tuple, 
            kwargs []starlark.Tuple) (starlark.Value, error) {
            
            // Get config values from Go code
            apiKey, _ := base.GetConfigValue[string](module, "api_key")
            endpoint, _ := base.GetConfigValue[string](module, "endpoint")
            timeout, _ := base.GetConfigValue[int](module, "timeout")
            
            // Use the values to make a request
            fmt.Printf("Making request to %s with timeout %ds\n", endpoint, timeout)
            // Implementation details...
            
            return starlark.None, nil
        }),
    }

    // Load the module
    loader := module.LoadModule("mymodule", additionalFuncs)

    // Execute Starlark code
    machine := starlet.NewDefault()
    machine.SetLazyloadModules(map[string]starlet.ModuleLoader{"mymodule": loader})
    machine.SetScriptContent([]byte(`
load("mymodule", "set_api_key", "set_timeout", "get_endpoint", "make_request")

# Configure the module
set_api_key("my-secret-key-12345")
set_timeout(60)

# Print the endpoint (uses default or environment variable)
print("Endpoint:", get_endpoint())

# Use the module function
make_request()
`))
    if _, err := machine.Run(); err != nil {
        panic(err)
    }
}
```

## API Reference

### Core Types

- `ConfigOption[T]`: Generic typed configuration option
- `ConfigOptionInterface`: Common interface for all configuration options
- `ConfigurableModule`: Container for configuration options

### Creation Methods

- `NewConfigOption[T](defaultValue T) *ConfigOption[T]`
- `NewConfigurableModule() *ConfigurableModule`
- `NewConfigurableModuleWithOptions(options ...ModuleOption) (*ConfigurableModule, error)`
- `NewConfigurableModuleWithConfigOptions(options ...ConfigOptionInterface) (*ConfigurableModule, error)`

### Configuration Option Methods

- `WithName(name string) *ConfigOption[T]`
- `WithDescription(desc string) *ConfigOption[T]`
- `WithEnvVar(envVar string) *ConfigOption[T]`
- `WithValue(value T) *ConfigOption[T]`
- `WithDefault(defaultValue T) *ConfigOption[T]`
- `WithValidator(validator ConfigValidator[T]) *ConfigOption[T]`
- `WithGetter(getter ConfigGetter[T]) *ConfigOption[T]`
- `SetRequired(required bool) *ConfigOption[T]`
- `SetSecret(secret bool) *ConfigOption[T]`
- `GetValueOrFallback(fallbackVal T) T`

### Module Options

- `WithConfigOption(name string, option ConfigOptionInterface) ModuleOption`
- `WithTypedConfigOption[T any](name string, option *ConfigOption[T]) ModuleOption`
- `WithConfigValue[T any](name string, value T) ModuleOption`
- `WithConfigDefault[T any](name string, defaultValue T) ModuleOption`
- `WithConfigGetter[T any](name string, getter ConfigGetter[T]) ModuleOption`
- `WithConfigEnvVar[T any](name string, envVar string) ModuleOption`

### Runtime Operations

- `SetConfigOption(name string, option ConfigOptionInterface) error`
- `Initialize() error`
- `ListConfigs() map[string]map[string]interface{}`
- `GetConfigOption(name string) (ConfigOptionInterface, error)`
- `GetConfigValue[T any](m *ConfigurableModule, name string) (T, error)`
- `SetConfigValue[T any](m *ConfigurableModule, name string, value T) error`
- `SetConfigGetter[T any](m *ConfigurableModule, name string, getter ConfigGetter[T]) error`
- `SetConfigEnvVar[T any](m *ConfigurableModule, name string, envVar string) error`
- `SetConfigDefault[T any](m *ConfigurableModule, name string, defaultValue T) error`
- `LoadModule(moduleName string, additionalFuncs starlark.StringDict) starlet.ModuleLoader`

### Convenience helpers

These reduce the boilerplate of checking errors when reading values; they fall back to the
provided value when the config is missing or fails to resolve. The `WithConfig*` forms are the
functional-option equivalents of the `SetConfig*` runtime setters listed above.

- `GetConfigValueWithFallback[T any](m *ConfigurableModule, name string, fallbackVal T) T`
- `(o *ConfigOption[T]) GetValueOrFallback(fallbackVal T) T`
- `WithConfigGetter[T any](name string, getter ConfigGetter[T]) ModuleOption`
- `WithConfigEnvVar[T any](name string, envVar string) ModuleOption`
- `WithConfigDefault[T any](name string, defaultValue T) ModuleOption`

### Extend()

`Extend()` wraps a `ConfigurableModule` in a `ConfigurableModuleExt`, exposing typed getters that
return the requested type directly and accept an optional fallback (defaulting to the type's zero
value). These are thin wrappers over `GetConfigValueWithFallback`.

- `(m *ConfigurableModule) Extend() *ConfigurableModuleExt`
- `(e *ConfigurableModuleExt) GetString(key string, fallbackVal ...string) string`
- `(e *ConfigurableModuleExt) GetInt(key string, fallbackVal ...int) int`
- `(e *ConfigurableModuleExt) GetUint(key string, fallbackVal ...uint) uint`
- `(e *ConfigurableModuleExt) GetBool(key string, fallbackVal ...bool) bool`
- `(e *ConfigurableModuleExt) GetFloat(key string, fallbackVal ...float64) float64`

## Contributing

Contributions are welcome! Please open an issue or submit a pull request if you have any improvements or suggestions.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
