# 🧱 `base` — Configurable Starlark Module Foundation

[![godoc](https://pkg.go.dev/badge/github.com/starpkg/base.svg)](https://pkg.go.dev/github.com/starpkg/base)
[![license](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![codecov](https://codecov.io/gh/starpkg/base/graph/badge.svg)](https://codecov.io/gh/starpkg/base)

A typed, configurable foundation for building Starlark modules that connect to
online services and external libraries.

## Overview

The `starpkg` ecosystem gives Starlark scripts *support for the necessary
**local** operations plus simple abstractions over common **online** services,
for ease of use.* `base` is the layer beneath all of that: not itself a local
capability or an online-service binding, but the **shared plumbing** every domain
module (`sqlite`, `web`, `llm`, `mq`, `s3`, `email`, …) uses to declare its typed
configuration, resolve it from values / getters / environment / defaults, keep
secrets out of scripts, and surface the result to Starlark.

It provides:

- **Type-safe configuration** using Go generics (`ConfigOption[T]`).
- **Multiple configuration sources** with a fixed precedence: explicit value →
  getter → environment variable → default.
- **Secret value handling** so sensitive options are settable but never readable
  from a script.
- **Environment-variable integration**, with automatic string-to-type parsing.
- **Automatic Starlark wiring**: `LoadModule` generates a `set_<key>` setter for
  every non-host-only option and a `get_<key>` getter for every non-secret option.

`base` is an L4 `starpkg` module that depends downward on `1set/starlet` (the
Machine plus `dataconv`) and transitively on `1set/starlight` plus
`go.starlark.net`; nothing in the ecosystem sits below it except those runtimes.

For the complete per-builtin reference — the generated accessors, their
signatures, parameters, returns, errors, value conversion, and the configuration
contract — see **[docs/API.md](docs/API.md)**.

## Installation

```bash
go get github.com/starpkg/base
```

## Quick Start

Register typed options on a `ConfigurableModule`, load it into a Starlet
interpreter, then `load(...)` the generated accessors from a script:

```go
package main

import (
    "github.com/1set/starlet"
    "github.com/starpkg/base"
)

func main() {
    module := base.NewConfigurableModule()

    // A secret option: settable from a script, but never readable.
    module.SetConfigOption("api_key",
        base.NewConfigOption("").
            WithDescription("API key for authentication").
            SetSecret(true))

    // A non-secret option backed by an environment variable.
    module.SetConfigOption("endpoint",
        base.NewConfigOption("https://api.example.com").
            WithEnvVar("API_ENDPOINT"))

    // A host-only limit: the host enforces it, so a script may read it
    // (get_max_bytes) but cannot change it — no set_max_bytes is generated.
    module.SetConfigOption("max_bytes",
        base.NewConfigOption(1<<20).
            WithDescription("maximum input size the module accepts").
            SetHostOnly(true))

    loader := module.LoadModule("mymodule", nil)

    machine := starlet.NewDefault()
    machine.SetLazyloadModules(map[string]starlet.ModuleLoader{"mymodule": loader})
    machine.SetScriptContent([]byte(`
load("mymodule", "set_api_key", "get_endpoint")

set_api_key("my-secret-key")          # secret: settable, no get_api_key exists
print("Using endpoint:", get_endpoint())
`))
    if _, err := machine.Run(); err != nil {
        panic(err)
    }
}
```

Pass host-defined builtins alongside the generated accessors via the
`additionalFuncs` argument of `LoadModule`, and read config values back in Go
with `base.GetConfigValue[T](module, "endpoint")`.

## Starlark API at a glance

`base` defines **no fixed-name builtins of its own.** `LoadModule` generates the
script surface from the options a host registers, so the accessor names depend on
the config keys:

- `set_<key>(value)` — set option `<key>` (highest-priority source); returns
  `None`. Generated for every **non-host-only** option (secret or not); a
  host-only option has no setter.
- `get_<key>()` — return the resolved value of non-secret option `<key>`.
  Generated for **non-secret options only** — a secret option exposes its setter
  but no getter.

Any callables passed in `additionalFuncs` are merged in under their own names.

See **[docs/API.md](docs/API.md)** for the full signatures, value conversion,
errors, and examples of every generated accessor.

## Configuration

Each option becomes a `set_<key>` setter (unless it is host-only) and a
`get_<key>` getter (unless it is secret); a value resolves in priority order —
explicit `set_<key>` → getter → environment variable (`WithEnvVar`,
conventionally `<MODULE>_<KEY>`) → default. Secret options expose only
`set_<key>`, never a getter; host-only options (`SetHostOnly(true)`, for a limit
the module enforces) expose only `get_<key>`, never a setter. See the
[Configuration section of docs/API.md](docs/API.md#configuration) for the full
accessor contract and conversion rules.

## Contributing

Contributions are welcome. Please open an issue or submit a pull request if you
have any improvements or suggestions.

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file
for details.
</content>
