# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`starpkg/base` is the **foundation L4 module** of the Star\* ecosystem. The `starpkg` mandate is *support for the necessary **local** operations + simple abstractions over common **online** services, for ease of use* — and `base` is the shared plumbing every domain module (`sqlite`, `web`, `llm`, `mq`, `s3`, `email`, …) is built on. It is neither a local capability nor an online-service binding itself; it is the **typed-configuration + Starlark-wiring layer** they all import.

Concretely, `base` gives a module author three things:

- **Typed config options** (`ConfigOption[T]`, Go generics) with a fixed resolution order: immediate value → getter → environment variable → default.
- **A configurable module container** (`ConfigurableModule`) that holds those options, is immutable after `Initialize()`, and emits a `starlet.ModuleLoader`.
- **Automatic Starlark wiring**: `LoadModule` generates a `set_<name>` builtin for every non-host-only option and a `get_<name>` builtin for every non-secret option, plus any host-supplied `additionalFuncs`.

Layer position: depends downward on `1set/starlet` (the Machine + `dataconv` for Go⇄Starlark marshalling) and transitively on `1set/starlight` + `go.starlark.net`. Every other `starpkg` module depends **on it**; it depends on nothing else in the ecosystem.

## Dev commands

Pure Go library with a Makefile. From this repo:

```bash
make test                                  # -race -cover -count 1, the working bar
make ci                                    # -race -cover profile + bench compile (what CI runs)
make bench                                 # benchmarks only
go test ./... -run TestSetValueFromStarlark   # a single test
gofmt -l . && go vet ./...                 # must be clean before commit
```

**Verify on the go floor in Docker** — this repo's floor is **go 1.19** (see Release discipline), older than the local toolchain, and the pinned `go.starlark.net` baseline uses `maphash.String` (needs ≥1.19). Behavior on the floor must be checked in a container:

```bash
docker run --rm -v "$PWD":/src -v "$HOME/go/pkg/mod":/go/pkg/mod -w /src golang:1.19 go test -race -count=1 ./...
```

`doccov` is the documentation gate (run it before pushing):

```bash
go run github.com/1set/meta/doccov@master .   # README must document every script-facing builtin
```

## Architecture (the part that spans files)

The module is a **typed-config core with a Starlark adapter on top**. One generic option type does the resolving; one container holds the options and turns them into a loadable Starlark module.

- **`config.go`** — the heart: `ConfigOption[T]`. Builder methods (`WithName`/`WithDescription`/`WithValue`/`WithGetter`/`WithEnvVar`/`WithDefault`/`WithValidator`/`SetRequired`/`SetSecret`) and accessors (`GetValue`/`SetValue`/`Validate`/`Has*`/`Is*`/`GetInfo`). `resolveValue` implements the **priority order** (value → getter → env → default) under a `recover` guard. The Starlark bridge lives here too: `SetValueFromStarlark` (Starlark→Go, via `dataconv.Unmarshal`, with reflect-based numeric/slice/map coercion) and `GetStarlarkValue` (Go→Starlark, via `dataconv.Marshal`). `convertEnvValue` parses env strings into T (bool/numeric direct, complex types via JSON).
- **`module.go`** — `ConfigurableModule`: a name→`ConfigOptionInterface` map behind a `sync.RWMutex`, plus `initialized`. Construction (`NewConfigurableModule*`), the functional `ModuleOption` setters (`WithConfig*`), the generic free-function setters/getters (`SetConfigValue`/`GetConfigValue`/`GetConfigValueWithFallback`/…), `Initialize` (validates required+validator, then freezes), and `LoadModule` — the one place that turns options into Starlark builtins via `generateSetBuiltin`/`generateGetBuiltin` and wraps them with `dataconv.WrapModuleData`.
- **`extend.go`** — `ConfigurableModuleExt` (`m.Extend()`): thin typed convenience getters (`GetString`/`GetInt`/`GetUint`/`GetBool`/`GetFloat`) with optional fallback, over `GetConfigValueWithFallback`.
- **`errors.go`** — the package doc comment + the sentinel errors (`ErrConfigNotSet`, `ErrConfigRequired`, `ErrConfigInvalidValue`, `ErrModuleAlreadyInitialized`, `ErrConfigGetterPanic`, …) used with `%w` wrapping.
- **`testing.go`** — exported harness helpers for **downstream** module repos: `RunStarlarkTests` (runs `*.star` from `../test/<module>`, `test-` prefix must pass / `panic-` must fail, loads a `.env`) and `RunTestScript`. These default to the private `starpkg/test` layout and self-skip when the directory is absent.

**Data flow (script side):** host registers options → `module.LoadModule("name", funcs)` → loader calls `Initialize()` (deferred, so a config error surfaces as the loader's error, not a host panic) → builds `set_<name>`/`get_<name>` builtins → script `load()`s them → `set_*` runs `SetValueFromStarlark`, `get_*` runs `GetStarlarkValue`.

## Invariants / hardening (preserve when editing)

1. **No host panics from script input (PKG-03).** `LoadModule` defers `Initialize()` into the returned loader so a bad config becomes the loader's `error`, never a crash. `SetValueFromStarlark` and `resolveValue` both carry a `recover()` that converts a panic into a wrapped `ErrConfigInvalidValue` / `ErrConfigGetterPanic`. Don't remove these deferred recovers.
2. **`None`/nil is rejected, not dereferenced.** `dataconv.Unmarshal(None)` yields a nil interface whose `reflect.TypeOf` is a nil `Type` — calling `.Kind()` on it panics. `SetValueFromStarlark` checks `gv == nil` up front and returns an error. Keep that guard before any reflection.
3. **Secrets never leak to scripts.** A `SetSecret(true)` option gets a `set_<name>` builtin but **no** `get_<name>`, and `GetInfo()` omits its value. Go code can still read it via `GetConfigValue`. Don't add a script-visible read path for secrets.
4. **Immutable after `Initialize()`.** Every setter checks `m.initialized` and returns `ErrModuleAlreadyInitialized`. The config map is guarded by `sync.RWMutex` (and each `ConfigOption` by its own mutex) so a loaded module is safe under concurrent script execution. Don't introduce post-init mutation.
5. **Backward compatibility (iron rule).** The resolution priority (value > getter > env > default), the generated `set_`/`get_` naming, and the exposure rules (non-secret → `get_`, non-host-only → `set_`; a host-only+secret option gets neither) are observable contract for every downstream module and its scripts. Any new lever must default to today's behavior so existing scripts run identically — `SetHostOnly` defaults to false, so an option stays script-settable unless a module opts in.

## Test organization

Group by functional goal — **do not add one `*_test.go` per fix.** The homes are: `config_test.go` (option resolution, validation, env parsing, `GetValueOrFallback`), `module_test.go` (container lifecycle, `LoadModule`, the generated builtins, the free-function setters/getters, concurrency), `extend_test.go` (the `Extend()` convenience getters), `starlark_test.go` (end-to-end script behavior plus the `SetValueFromStarlark`/`GetStarlarkValue` conversion, secret-access, map-key, and None/panic-safety sections), and `example_test.go` (the `Example*` runnable docs). Add a new test as a **section in the matching file**, not a new file. Tests are table/example-driven; no third-party test framework.

The exported `RunStarlarkTests`/`RunTestScript` helpers target `../test/base/*.star` integration scripts that live in the **private `starpkg/test` repo** and auto-skip when that directory is absent (e.g. in CI). base's own tests don't ship those scripts.

## Documentation

Three layers must stay in sync (enforced by the doc standard, `plan/starpkg文档标准（DOC-STD）`):

- **`README.md`** — every script-facing builtin documented as a backtick whole-word so `doccov` exits 0. Because `base` generates builtins by string concatenation (`"set_" + name`), the gate sees the `set_` and `get_` families; document them as `set_<name>` / `get_<name>` and keep names/signatures matching the code.
- **GoDoc** — the package comment (in `errors.go`; only one file carries it) + a doc comment on every exported symbol whose first word is the symbol name (gated by `revive`'s `exported`/`package-comments` rules in CI).
- **CLAUDE.md** — this file.

## Release discipline

- **Floor = go 1.19** (`go.mod`), matching the pinned `go.starlark.net` baseline (`ffb3f39`) and `1set/starlet v0.2.3` (which carries starlight v0.2.1 transitively). The floor only rises in this repo's own isolated pin-upgrade PR.
- **CI matrix** = `[1.19.x, 1.25.x]` via the centralized reusable workflow `1set/meta/.github/workflows/go-ci.yml` (pinned to a full commit SHA), with `doc-coverage: true` enabling the `doccov` gate.
- **Pin upgrade is the last PR** of the repo's series and is isolated; **don't tag a release until it merges.**
- **Bumping the version, the go floor, or tagging are user-confirmed actions** — never tag autonomously; default to patch bumps; published tags are immutable in the module proxy.
