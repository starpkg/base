# 🧱 `base` Module for Starlark Extensions

[![godoc](https://pkg.go.dev/badge/github.com/starpkg/base.svg)](https://pkg.go.dev/github.com/starpkg/base)
[![licenese](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

A generic base module that bridges the gap between online services, external libraries, and the [Starlark](https://github.com/google/starlark-go) runtime. This module allows you to create configurable Starlark modules by extending its functionality with different configurations.

## Features

- **`ConfigurableModule`**: A generic module that can be extended with custom configurations.
- **Flexible Configuration**: Supports setting and retrieving configuration values dynamically.
- **Starlark Integration**: Provides Starlark callable functions to interact with configurations.

## Installation

To install the module, run:

```bash
go get github.com/starpkg/base
```

## Usage

Here's how you can use the `ConfigurableModule` to create custom Starlark modules:

```go
package main

import (
    "fmt"

    "github.com/starpkg/base"
    "github.com/1set/starlet"
    "go.starlark.net/starlark"
)

func main() {
    // Create a new configurable module
    cm := base.NewConfigurableModule[string]()

    // Set configuration values
    cm.SetConfigValue("api_key", "your-api-key")
    cm.SetConfigValue("endpoint", "https://api.example.com")

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
load("mymodule", "set_api_key", "set_endpoint", "do_something")

set_api_key("new-api-key")
set_endpoint("https://api.newexample.com")
do_something()
`

    // Run the script with the module loader ...
}
```

## Documentation

### `ConfigurableModule`

`ConfigurableModule[T any]` is a generic module that allows you to set and get configuration values of any type `T`.

#### Methods

- `NewConfigurableModule[T any]() *ConfigurableModule[T]`: Creates a new instance of `ConfigurableModule`.

- `SetConfig(name string, getter ConfigGetter[T])`: Sets a configuration getter for a given name.

  ```go
  cm.SetConfig("api_key", func() string { return "dynamic-api-key" })
  ```

- `SetConfigValue(name string, value T)`: Sets a direct configuration value for a given name.

  ```go
  cm.SetConfigValue("api_key", "your-api-key")
  ```

- `GetConfig(name string) (T, error)`: Retrieves the configuration value for a given name.

  ```go
  apiKey, err := cm.GetConfig("api_key")
  ```

- `LoadModule(moduleName string, additionalFuncs starlark.StringDict) starlet.ModuleLoader`: Returns a Starlark module loader with the given configurations and additional functions.

  ```go
  loader := cm.LoadModule("mymodule", starlark.StringDict{
      "my_function": myStarlarkFunction,
  })
  ```

### Starlark Functions

When you load the module in Starlark, it automatically provides setter functions for each configuration:

- `set_<config_name>(value)`: Sets the configuration value from within Starlark.

#### Example

```python
load("mymodule", "set_api_key", "do_something")

set_api_key("new-api-key")
do_something()
```

## Contributing

Contributions are welcome! Please open an issue or submit a pull request if you have any improvements or suggestions.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
