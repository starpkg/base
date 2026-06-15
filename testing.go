package base

// This file provides foundational test helpers for Starlark modules built on
// ConfigurableModule. The canonical package doc comment lives in errors.go;
// only one file in a package should carry the "// Package base ..." comment
// (revive's package-comments rule), so this file deliberately omits it.

import (
	"bufio"
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
//   - moduleFactory: function that creates a fresh module loader for each test run
//   - extraModules: additional modules to include in the Starlark environment
//   - testDirPath: path to the directory containing test scripts (if empty, defaults to "../test/{moduleName}")
func RunStarlarkTests(t *testing.T, moduleName string, moduleFactory func() starlet.ModuleLoader, extraModules []string, testDirPath string) {
	// If testDirPath is not provided, use the default path
	if testDirPath == "" {
		testDirPath = filepath.Join("..", "test", moduleName)
	}

	// Locate test directory
	if _, err := os.Stat(testDirPath); os.IsNotExist(err) {
		t.Skip("Test directory not found:", testDirPath)
		return
	}

	// Get absolute path for testDirPath
	absTestDirPath, err := filepath.Abs(testDirPath)
	if err != nil {
		t.Logf("Warning: Failed to get absolute path for test directory: %v", err)
		absTestDirPath = testDirPath // Fall back to relative path
	}

	// Load environment variables from .env file if it exists
	if err := loadEnvFile(absTestDirPath); err != nil {
		t.Logf("Warning: Failed to load .env file: %v", err)
	}

	// Find test scripts
	scripts, err := filepath.Glob(filepath.Join(absTestDirPath, "*.star"))
	if err != nil || len(scripts) == 0 {
		t.Skip("No test scripts found in:", absTestDirPath)
		return
	}

	// Track found script patterns
	foundPatterns := map[string]bool{"test-": false, "panic-": false}

	// Execute each script
	for _, scriptPath := range scripts {
		scriptName := filepath.Base(scriptPath)
		t.Run(scriptName, func(t *testing.T) {
			// Save current working directory
			originalWd, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get current working directory: %v", err)
			}

			// Change to test directory and restore after test
			err = os.Chdir(absTestDirPath)
			if err != nil {
				t.Fatalf("Failed to change to test directory: %v", err)
			}
			defer func() {
				// Restore original working directory
				if err := os.Chdir(originalWd); err != nil {
					t.Logf("Warning: Failed to restore original working directory: %v", err)
				}
			}()

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
			env := starlet.NewWithNames(starlet.StringAnyMap{}, extraModules, extraModules)
			env.AddLazyloadModules(map[string]starlet.ModuleLoader{moduleName: moduleFactory()})
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

// RunTestScript is a helper function that executes a Starlark script with the specified module.
// This reduces boilerplate code in test functions by centralizing the common setup logic.
//
// Parameters:
//   - t: the testing instance
//   - script: the Starlark script content to execute
//   - moduleName: name of the module being tested
//   - moduleFactory: function that creates a fresh module loader
//   - extraModuleLoaders: optional map of additional module loaders to include
//
// The function will automatically:
// 1. Create a new module instance using the provided factory
// 2. Set up a Starlet interpreter
// 3. Add the module and any extra modules to the interpreter
// 4. Execute the provided script
// 5. Handle any execution errors
func RunTestScript(t *testing.T, script string, moduleName string, moduleFactory func() starlet.ModuleLoader, extraModuleLoaders map[string]starlet.ModuleLoader) {
	// Create Starlet interpreter with default configuration
	s := starlet.NewDefault()

	// Create module loaders map starting with the main module
	moduleLoaders := map[string]starlet.ModuleLoader{
		moduleName: moduleFactory(),
	}

	// Add any extra module loaders if provided
	if extraModuleLoaders != nil {
		for name, loader := range extraModuleLoaders {
			moduleLoaders[name] = loader
		}
	}

	// Add all modules to the interpreter
	s.AddLazyloadModules(moduleLoaders)

	// Execute the script
	_, err := s.RunScript([]byte(script), nil)
	if err != nil {
		t.Fatalf("Error executing script: %v\n", err)
	}

	t.Log("Test executed successfully")
}

// loadEnvFile loads environment variables from a .env file
// if it exists in the specified directory.
func loadEnvFile(dirPath string) error {
	envPath := filepath.Join(dirPath, ".env")

	// Check if .env file exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return nil // No .env file, not an error
	}

	// Open the .env file
	file, err := os.Open(envPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key=value pairs
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // Invalid format, skip
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove a surrounding pair of matching quotes if present. The
		// len(value) > 1 guard must apply to both quote styles: without the
		// parentheses, operator precedence (&& over ||) left the single-quote
		// branch unguarded, so a lone-quote value (e.g. KEY=') sliced
		// value[1:0] and panicked. Stripping still requires at least two
		// characters, so previously-stripped values are unaffected.
		if len(value) > 1 &&
			((strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
				(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'"))) {
			value = value[1 : len(value)-1]
		}

		// Set environment variable
		os.Setenv(key, value)
	}

	return scanner.Err()
}
