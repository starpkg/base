// Package base provides foundational constructs for Starlark modules.
package base

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/1set/starlet"
	"go.starlark.net/starlark"
)

// RunStarlarkTests is a helper function that runs Starlark test scripts from a specified test directory.
// Scripts with "test-" prefix should succeed, "panic-" prefix should fail.
//
// Parameters:
//   - t: the testing instance
//   - moduleName: name of the module being tested
//   - moduleLoader: function that loads the module
//   - extraModules: additional modules to include in the Starlark environment
//   - testDirPath: path to the directory containing test scripts (if empty, defaults to "../test/{moduleName}")
func RunStarlarkTests(t *testing.T, moduleName string, moduleLoader starlet.ModuleLoader, extraModules []string, testDirPath string) {
	// If testDirPath is not provided, use the default path
	if testDirPath == "" {
		testDirPath = filepath.Join("..", "test", moduleName)
	}

	// Locate test directory
	if _, err := os.Stat(testDirPath); os.IsNotExist(err) {
		t.Skip("Test directory not found:", testDirPath)
		return
	}

	// Find test scripts
	scripts, err := filepath.Glob(filepath.Join(testDirPath, "*.star"))
	if err != nil || len(scripts) == 0 {
		t.Skip("No test scripts found in:", testDirPath)
		return
	}

	// Track found script patterns
	foundPatterns := map[string]bool{"test-": false, "panic-": false}

	// Execute each script
	for _, scriptPath := range scripts {
		scriptName := filepath.Base(scriptPath)
		t.Run(scriptName, func(t *testing.T) {
			// Determine expected outcome from filename
			shouldPanic := strings.HasPrefix(scriptName, "panic-")
			if shouldPanic || strings.HasPrefix(scriptName, "test-") {
				prefix := "test-"
				if shouldPanic {
					prefix = "panic-"
				}
				foundPatterns[prefix] = true
			}

			// Read and run the script
			content, err := os.ReadFile(scriptPath)
			if err != nil {
				t.Fatalf("Failed to read %q: %v", scriptName, err)
			}

			// Setup Starlark environment
			env := starlet.NewWithNames(starlet.StringAnyMap{}, nil, extraModules)
			env.AddLazyloadModules(map[string]starlet.ModuleLoader{moduleName: moduleLoader})
			env.SetScriptContent(content)

			// Capture output for debugging
			var output strings.Builder
			env.SetPrintFunc(func(_ *starlark.Thread, msg string) {
				output.WriteString(msg)
				output.WriteString("\n")
			})

			// Run the script
			_, err = env.Run()

			// Verify results match expectations
			if shouldPanic && err == nil {
				t.Errorf("Expected %q to fail", scriptName)
			} else if !shouldPanic && err != nil {
				t.Errorf("Expected %q to succeed: %v\nOutput: %s", scriptName, err, output.String())
			}
		})
	}

	// Report pattern coverage
	for pattern, found := range foundPatterns {
		if !found {
			t.Skipf("No scripts with pattern %q were found", pattern)
		}
	}
}
