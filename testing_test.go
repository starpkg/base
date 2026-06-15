package base_test

// Tests for the exported Starlark test-harness helpers in testing.go.
// These helpers normally target the private ../test/<module> integration
// suite (TTY / live-SaaS scripts) and self-skip when that directory is
// absent, which is why they show 0% coverage in CI. The sections below drive
// them against synthetic, network-free temp directories so the harness logic
// itself — directory discovery, the test-/panic- contract, .env loading, the
// working-directory save/restore, and RunTestScript's single-script path — is
// exercised without a TTY or any credentials.
//
// Sections:
//   - RunStarlarkTests_Skips        — missing dir / empty dir / no .star files all Skip cleanly
//   - RunStarlarkTests_Contract     — test- scripts must pass, panic- scripts must fail
//   - RunStarlarkTests_EnvFile      — a .env in the test dir is loaded into the process env
//   - RunStarlarkTests_Cwd          — the working directory is restored after the run
//   - RunTestScript_Executes        — RunTestScript runs a script against a module + extras

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/1set/starlet"
	"github.com/starpkg/base"
	"go.starlark.net/starlark"
)

// newHarnessModule returns a module-loader factory exposing set_/get_ builtins
// for a string option whose value can come from the FOO_FROM_ENV environment
// variable (so the .env section can assert the file was loaded).
func newHarnessModule() func() starlet.ModuleLoader {
	return func() starlet.ModuleLoader {
		m := base.NewConfigurableModule()
		m.SetConfigOption("greeting", base.NewConfigOption("hello"))
		m.SetConfigOption("from_env", base.NewConfigOption("unset").WithEnvVar("FOO_FROM_ENV"))
		return m.LoadModule("harness", nil)
	}
}

// writeScript creates a *.star file in dir.
func writeScript(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// harnessResult captures whether a RunStarlarkTests invocation skipped or
// failed, observed via its own *testing.T (RunStarlarkTests calls Skip/Errorf
// on the T it is given).
type harnessResult struct {
	skipped bool
	failed  bool
}

// runHarness invokes base.RunStarlarkTests against testDir inside a dedicated
// subtest and reports the subtest's skip/fail status without propagating it to
// the parent. t.Run returns false when the subtest failed; the inner T's
// Skipped() reports a skip.
func runHarness(t *testing.T, testDir string) harnessResult {
	t.Helper()
	var res harnessResult
	ok := t.Run("harness", func(it *testing.T) {
		// Run in the subtest so a Skip's runtime.Goexit unwinds here, not in
		// the parent; record the skip flag before Skip exits the goroutine.
		defer func() { res.skipped = it.Skipped() }()
		base.RunStarlarkTests(it, "harness", newHarnessModule(), nil, testDir)
		res.skipped = it.Skipped()
	})
	res.failed = !ok && !res.skipped
	return res
}

func TestRunStarlarkTestsHarness(t *testing.T) {
	t.Run("RunStarlarkTests_Skips", func(t *testing.T) {
		t.Run("MissingDir", func(t *testing.T) {
			missing := filepath.Join(t.TempDir(), "does-not-exist")
			// A self-skipping subtest: RunStarlarkTests calls t.Skip on its own
			// *testing.T, which marks this subtest skipped rather than failed.
			res := runHarness(t, missing)
			if !res.skipped {
				t.Errorf("expected skip for missing dir, got skipped=%v failed=%v", res.skipped, res.failed)
			}
		})

		t.Run("EmptyDir", func(t *testing.T) {
			empty := t.TempDir()
			res := runHarness(t, empty)
			if !res.skipped {
				t.Errorf("expected skip for dir with no .star files, got skipped=%v failed=%v", res.skipped, res.failed)
			}
		})
	})

	t.Run("RunStarlarkTests_Contract", func(t *testing.T) {
		dir := t.TempDir()
		// A passing script (test- prefix must succeed).
		writeScript(t, dir, "test-ok.star", `
load("harness", "set_greeting", "get_greeting")
def run():
    set_greeting("hi")
    if get_greeting() != "hi":
        fail("expected hi")
run()
`)
		// A failing script (panic- prefix must fail).
		writeScript(t, dir, "panic-fails.star", `
load("harness", "get_greeting")
fail("deliberate failure")
`)
		res := runHarness(t, dir)
		if res.failed {
			t.Errorf("expected the test-/panic- contract to be satisfied, but the harness reported failure")
		}
	})

	t.Run("RunStarlarkTests_EnvFile", func(t *testing.T) {
		dir := t.TempDir()
		// loadEnvFile must parse this file and set FOO_FROM_ENV in the process
		// env; the script then reads it back through the env-backed option.
		// Includes a comment line, a blank line, a quoted value, and a malformed
		// line to exercise loadEnvFile's parsing branches.
		envBody := "" +
			"# a comment\n" +
			"\n" +
			"FOO_FROM_ENV=\"from-dot-env\"\n" +
			"malformed-line-without-equals\n"
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envBody), 0o600); err != nil {
			t.Fatalf("write .env: %v", err)
		}
		// Ensure a clean slate and restore afterwards.
		os.Unsetenv("FOO_FROM_ENV")
		defer os.Unsetenv("FOO_FROM_ENV")

		writeScript(t, dir, "test-env.star", `
load("harness", "get_from_env")
def run():
    if get_from_env() != "from-dot-env":
        fail("env var was not loaded from .env, got: " + get_from_env())
run()
`)
		// Include a panic- script so both prefixes are present and the harness
		// does not end with a missing-pattern Skip (which would mask a failure).
		writeScript(t, dir, "panic-expected.star", `fail("expected")`)
		res := runHarness(t, dir)
		if res.failed {
			t.Errorf("expected .env-backed script to pass; harness reported failure")
		}
		if got := os.Getenv("FOO_FROM_ENV"); got != "from-dot-env" {
			t.Errorf("expected loadEnvFile to set FOO_FROM_ENV=from-dot-env, got %q", got)
		}
	})

	t.Run("RunStarlarkTests_Cwd", func(t *testing.T) {
		before, err := os.Getwd()
		if err != nil {
			t.Fatalf("getwd: %v", err)
		}
		dir := t.TempDir()
		writeScript(t, dir, "test-trivial.star", `
load("harness", "get_greeting")
x = get_greeting()
`)
		writeScript(t, dir, "panic-expected.star", `fail("expected")`)
		_ = runHarness(t, dir)
		after, err := os.Getwd()
		if err != nil {
			t.Fatalf("getwd: %v", err)
		}
		if before != after {
			t.Errorf("working directory not restored: before=%q after=%q", before, after)
		}
	})

	// RunStarlarkTests_EnvQuoting pins loadEnvFile's quote handling: a
	// surrounding pair of matching quotes is stripped, but a malformed lone
	// quote must NOT panic (regression for the operator-precedence bug where
	// the len>1 guard did not cover the single-quote branch and sliced [1:0]).
	t.Run("RunStarlarkTests_EnvQuoting", func(t *testing.T) {
		dir := t.TempDir()
		envBody := "" +
			"DQUOTED=\"dq\"\n" + // double-quoted -> dq
			"SQUOTED='sq'\n" + // single-quoted -> sq
			"LONEQ='\n" + // lone single quote -> must not panic, kept verbatim
			"EMPTYQ=''\n" + // empty quoted -> empty string
			"BARE=bare\n" // unquoted -> bare
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envBody), 0o600); err != nil {
			t.Fatalf("write .env: %v", err)
		}
		for _, k := range []string{"DQUOTED", "SQUOTED", "LONEQ", "EMPTYQ", "BARE"} {
			os.Unsetenv(k)
			defer os.Unsetenv(k)
		}
		writeScript(t, dir, "test-trivial.star", `x = 1`)
		writeScript(t, dir, "panic-expected.star", `fail("expected")`)

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("loadEnvFile panicked on a malformed .env value: %v", r)
			}
		}()
		_ = runHarness(t, dir)

		cases := map[string]string{
			"DQUOTED": "dq",
			"SQUOTED": "sq",
			"LONEQ":   "'", // lone quote: not a pair, left as-is
			"EMPTYQ":  "",  // two quotes stripped to empty
			"BARE":    "bare",
		}
		for k, want := range cases {
			if got := os.Getenv(k); got != want {
				t.Errorf("env %s: want %q, got %q", k, want, got)
			}
		}
	})

	t.Run("RunTestScript_Executes", func(t *testing.T) {
		script := `
load("harness", "set_greeting", "get_greeting")
load("extra", "double")
def run():
    set_greeting("yo")
    if get_greeting() != "yo":
        fail("greeting mismatch")
    if double(21) != 42:
        fail("double mismatch")
run()
`
		extra := map[string]starlet.ModuleLoader{
			"extra": func() (starlark.StringDict, error) {
				return starlark.StringDict{
					"double": starlark.NewBuiltin("double", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
						var n int
						if err := starlark.UnpackArgs("double", args, nil, "n", &n); err != nil {
							return nil, err
						}
						return starlark.MakeInt(n * 2), nil
					}),
				}, nil
			},
		}
		// RunTestScript fails the test on script error; a clean run is the assertion.
		base.RunTestScript(t, script, "harness", newHarnessModule(), extra)
	})
}
