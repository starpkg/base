# `base` — Starlark API Reference

The complete reference for the script-facing surface a `base`-built module
exposes: the generated configuration accessor builtins and any host-supplied
functions. For an overview, installation, the resolution-priority model, and a
quickstart, see the [README](../README.md).

`base` defines **no fixed-name builtins of its own**. Instead,
`LoadModule(moduleName, additionalFuncs)` derives the script-facing surface from
the configuration options a host registered, so the exact function names depend
on the module's config keys. For every registered option named `<name>` the
loader generates a `set_<name>` setter, and for every **non-secret** option a
`get_<name>` getter; any callables passed in `additionalFuncs` are merged in
alongside under their own names.

## Contents

- [Generated configuration accessors](#generated-configuration-accessors)
  - [`set_<name>(value)`](#set_namevalue)
  - [`get_<name>()`](#get_name)
- [Additional functions](#additional-functions)
- [Value conversion](#value-conversion)
- [Configuration](#configuration)

## Generated configuration accessors

When a `ConfigurableModule` is loaded with `LoadModule(moduleName, …)`, each
registered option contributes one or two builtins to the module namespace, named
by string concatenation from the option's key:

| Generated builtin | Generated for           | Signature           |
|-------------------|-------------------------|---------------------|
| `set_<name>`      | every option            | `set_<name>(value)` |
| `get_<name>`      | non-secret options only | `get_<name>()`      |

So a module with options `api_key` (secret) and `endpoint` (non-secret) exposes
exactly `set_api_key`, `set_endpoint`, and `get_endpoint` — a setter for every
option and a getter for every non-secret option.

### `set_<name>(value)`

Sets the configuration option `<name>` from a single positional `value`. This
is the highest-priority source in the resolution order (it overrides getter,
environment variable, and default), so a value set from a script takes effect
immediately for subsequent reads.

**Parameters:**

- `value`: the new value for the option. It is converted from Starlark to Go via
  `dataconv.Unmarshal`, then coerced to match the option's declared Go type:
  - numeric scalars are converted between numeric kinds (e.g. a Starlark int into
    a Go `int64`/`float64` option). The conversion is **checked**: a value that
    would overflow the target (e.g. `300` into an `int8`), a negative value into
    an unsigned option, `NaN`/`Inf`, or a non-integral float into an integer
    option is **rejected with an error**, never silently truncated or wrapped;
  - lists are converted element-by-element into the target slice type;
  - dicts are converted into the target map type (numeric map keys, which
    `dataconv` renders as decimal strings, are parsed back to the numeric key
    type);
  - other types are accepted only by a direct type assertion to the option's type.

**Returns:** `None`.

**Errors:** raises a script error (rather than crashing the host) when:

- the value is `None`/`nil` (a `None` cannot populate a typed option);
- the value's type cannot be converted to the option's type (type mismatch);
- a numeric value is out of range for the target (overflow), negative into an
  unsigned option, `NaN`/`Inf`, or a non-integral float into an integer option
  (checked conversion — no silent truncation/wrapping);
- a slice element or map key/value cannot be converted to the target element type;
- the option has a validator and the converted value fails it.

A reflection/conversion panic is recovered internally and surfaced as an
`invalid config value` error, never as a host panic.

**Example:**

```python
load("mymodule", "set_api_key", "set_endpoint", "set_timeout")

set_api_key("sk-secret-12345")        # secret option: settable, but no getter exists
set_endpoint("https://api.prod")      # string option
set_timeout(60)                        # int option
```

### `get_<name>()`

Returns the resolved value of the non-secret option `<name>`, marshalled back to
a Starlark value via `dataconv.Marshal`. The value is resolved in priority order:
an explicit `set_<name>` value, then the getter, then the environment variable,
then the default.

This builtin is **only generated for non-secret options.** A `SetSecret(true)`
option exposes its `set_<name>` but no `get_<name>` — secret values are never
readable from a script (Go code can still read them via `GetConfigValue`).

**Parameters:** none.

**Returns:** the option's current value as a Starlark value (string, int, float,
bool, list, dict, …, matching the option's Go type).

**Errors:** raises a script error if the option's value cannot be resolved (for
example, a configured getter function panics).

**Example:**

```python
load("mymodule", "get_endpoint", "get_timeout")

print(get_endpoint())   # resolved value: explicit > getter > env > default
print(get_timeout())
```

## Additional functions

Any callables passed to `LoadModule(moduleName, additionalFuncs)` (a
`starlark.StringDict` of `starlark.Callable` values supplied by the host module)
are merged into the same module namespace under their own names, alongside the
generated `set_<name>` / `get_<name>` accessors. Their signatures, parameters,
returns, and errors are defined by the host module that registers them, not by
`base`; consult that module's own reference.

```python
load("mymodule", "set_api_key", "get_endpoint", "make_request")

set_api_key("sk-...")     # generated accessor
print(get_endpoint())     # generated accessor
make_request()            # a host-supplied additional function
```

## Value conversion

The accessors marshal values between Starlark and the option's Go type:

- **Starlark → Go (`set_<name>`)** — `dataconv.Unmarshal` followed by reflect
  coercion: numeric kinds convert across; lists become slices element-wise; dicts
  become maps (numeric keys are parsed from their decimal-string form); `None`/`nil`
  is rejected.
- **Go → Starlark (`get_<name>`)** — `dataconv.Marshal`: the resolved value
  (explicit > getter > env > default) is marshalled back.

Environment-variable string values (resolution priority 3) are parsed into the
option's Go type before marshalling, by target type:

- **string** — the raw string, as-is.
- **bool** — `true`/`false`, `yes`/`no`, `1`/`0`, `on`/`off` (case-insensitive).
- **integer** (`int`, `int8…int64`, `uint`, `uint8…uint64`) — a base-10 integer
  literal.
- **float** (`float32`, `float64`) — a decimal/float literal.
- **slice / map** (complex types) — a JSON document (a value starting with `[` or
  `{`).

A value that fails to parse for its target type is ignored, and resolution falls
through to the default.

## Configuration

`base` is itself the configuration framework, so it registers **no fixed config
keys of its own** — there are no built-in option names to enumerate here. The
accessors a script sees are generated from whatever options the **host module**
registers, following one uniform contract:

- **`get_<key>()`** — returns the option's current value. Generated for
  **non-secret** options only.
- **`set_<key>(value)`** — sets the option (returns `None`). Generated for
  **every** option, secret or not.

A `<key>`'s value resolves in priority order — an explicit `set_<key>` value, the
option's getter, the environment variable, then the default. The environment
variable is whatever name the host passes to `WithEnvVar`; the conventional name
a `starpkg` domain module uses is `<MODULE>_<KEY>` (uppercased), e.g. a
`timeout` option on the `sqlite` module reads `SQLITE_TIMEOUT`.

The generated-accessor contract that every `base`-built option follows is below.
`<MODULE>` is the loaded module name, `<KEY>` the uppercased option key; the env
var is host-chosen via `WithEnvVar` (conventionally `<MODULE>_<KEY>`), and the
default/description are whatever the host declared (`WithDefault` /
`NewConfigOption`, `WithDescription`).

- **`<key>` (non-secret)** — getter `get_<key>`, setter `set_<key>`; both value
  reads and writes are reachable from a script.
- **`<key>` (secret)** — setter `set_<key>` only; no getter is generated, since
  secrets are never readable from a script.

**Secret options expose only `set_<key>` — never a getter.** A host marks an
option secret with `SetSecret(true)` (or the equivalent `genSecretConfigOption`
helper a domain module wraps it in); the loader then emits the `set_<key>` setter
but no `get_<key>`, and `ListConfigs()` / `GetInfo()` omit the value. Go code can
still read a secret via `GetConfigValue`, but it is never reachable from a script.

**Example:**

A host that registers `api_key` (secret) and `endpoint` (non-secret) on a module
loaded as `mymodule` exposes exactly these accessors:

```python
load("mymodule", "set_api_key", "set_endpoint", "get_endpoint")

set_api_key("sk-secret-12345")     # secret: settable, no get_api_key exists
set_endpoint("https://api.prod")   # non-secret: settable
print(get_endpoint())              # non-secret: readable
```

For the resolution-priority model, the Go-side builder/accessor API
(`NewConfigOption`, `WithEnvVar`, `WithDefault`, `SetSecret`, `GetConfigValue`,
…), and how a host registers options, see the [README](../README.md) and the
[GoDoc](https://pkg.go.dev/github.com/starpkg/base).
</content>
</invoke>
