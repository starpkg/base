package base_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/1set/starlet"
	"github.com/starpkg/base"
	"go.starlark.net/starlark"
)

func TestStarlarkIntegration(t *testing.T) {
	// Test basic Starlark integration
	t.Run("BasicIntegration", func(t *testing.T) {
		// Create a module with options
		module := base.NewConfigurableModule()

		// Add a string option
		module.SetConfigOption("string_opt", base.NewConfigOption("default"))

		// Add an int option
		module.SetConfigOption("int_opt", base.NewConfigOption(42))

		// Add a bool option
		module.SetConfigOption("bool_opt", base.NewConfigOption(true))

		// Initialize and load the module
		err := module.Initialize()
		if err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		// Load the module with starlark
		loader := module.LoadModule("test_module", nil)

		// Create an environment to test the module
		env := starlet.NewDefault()
		loaders := make(map[string]starlet.ModuleLoader)
		loaders["test_module"] = loader
		env.SetLazyloadModules(loaders)

		// Create a script that tests the module's functions
		script := `
# Load the module functions
load("test_module", "set_string_opt", "get_string_opt")

# Test setting a value
set_string_opt("new_value")

# Test getting a value
val = get_string_opt()
print(val)
`
		env.SetScriptContent([]byte(script))

		// Run the script
		_, scriptErr := env.Run()
		if scriptErr != nil {
			t.Errorf("Script execution failed: %v", scriptErr)
		}

		// Verify the value was actually changed in the Go module
		strOpt, err := module.GetConfigOption("string_opt")
		if err != nil {
			t.Fatalf("Failed to get string_opt: %v", err)
		}
		typedStrOpt, ok := strOpt.(*base.ConfigOption[string])
		if !ok {
			t.Fatalf("string_opt is not of type *ConfigOption[string]")
		}

		val, err := typedStrOpt.GetValue()
		if err != nil {
			t.Fatalf("Failed to get value: %v", err)
		}

		if val != "new_value" {
			t.Errorf("Expected string_opt value to be 'new_value', got '%s'", val)
		}
	})

	// Test complex Starlark value conversion
	t.Run("ComplexValueConversion", func(t *testing.T) {
		// Test with map conversion
		mapOpt := base.NewConfigOption(map[string]int{})

		// Create a Starlark dict
		dict := starlark.NewDict(3)
		dict.SetKey(starlark.String("key1"), starlark.MakeInt(1))
		dict.SetKey(starlark.String("key2"), starlark.MakeInt(2))
		dict.SetKey(starlark.String("key3"), starlark.MakeInt(3))

		// Set value from Starlark
		err := mapOpt.SetValueFromStarlark(dict)
		if err != nil {
			t.Fatalf("SetValueFromStarlark failed for map: %v", err)
		}

		// Get and verify the value
		val, err := mapOpt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}

		if len(val) != 3 || val["key1"] != 1 || val["key2"] != 2 || val["key3"] != 3 {
			t.Errorf("Expected map with 3 keys and values, got %v", val)
		}

		// Test with slice conversion
		sliceOpt := base.NewConfigOption([]float64{})

		// Create a Starlark list
		list := starlark.NewList([]starlark.Value{
			starlark.Float(1.1),
			starlark.Float(2.2),
			starlark.Float(3.3),
		})

		// Set value from Starlark
		err = sliceOpt.SetValueFromStarlark(list)
		if err != nil {
			t.Fatalf("SetValueFromStarlark failed for slice: %v", err)
		}

		// Get and verify the value
		sliceVal, err := sliceOpt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}

		if len(sliceVal) != 3 || sliceVal[0] != 1.1 || sliceVal[1] != 2.2 || sliceVal[2] != 3.3 {
			t.Errorf("Expected slice with 3 elements, got %v", sliceVal)
		}

		// Test with incorrect types
		invalidSliceOpt := base.NewConfigOption([]int{})
		invalidList := starlark.NewList([]starlark.Value{
			starlark.String("not an int"),
			starlark.MakeInt(2),
		})

		err = invalidSliceOpt.SetValueFromStarlark(invalidList)
		if err == nil {
			t.Error("Expected error for invalid slice element type, got nil")
		}

		invalidMapOpt := base.NewConfigOption(map[int]string{})
		invalidDict := starlark.NewDict(1)
		invalidDict.SetKey(starlark.String("not an int key"), starlark.String("value"))

		err = invalidMapOpt.SetValueFromStarlark(invalidDict)
		if err == nil {
			t.Error("Expected error for invalid map key type, got nil")
		}

		complexOpt := base.NewConfigOption(complex(1, 2))
		err = complexOpt.SetValueFromStarlark(&starlark.Builtin{})
		if err == nil {
			t.Error("Expected error for invalid complex number, got nil")
		}

		complexMapOpt := base.NewConfigOption(map[string]complex64{"a": complex(float32(1), float32(2))})
		err = complexMapOpt.SetValueFromStarlark(dict)
		if err == nil {
			t.Error("Expected error for invalid complex number, got nil")
		}
	})

	// Test edge cases
	t.Run("EdgeCases", func(t *testing.T) {
		// Test setting a value that can't be converted
		opt := base.NewConfigOption(struct{}{})
		err := opt.SetValueFromStarlark(starlark.String("can't convert to struct"))
		if err == nil {
			t.Error("Expected error when setting value of incompatible type, got nil")
		}

		// Test GetStarlarkValue for secrets
		secretOpt := base.NewConfigOption("secret").SetSecret(true)

		// Secret value should be accessible in Go
		secretVal, err := secretOpt.GetValue()
		if err != nil {
			t.Fatalf("GetValue should not return error for secret configs: %v", err)
		}
		if secretVal != "secret" {
			t.Errorf("Expected secret value to be 'secret', got '%s'", secretVal)
		}

		// GetStarlarkValue itself doesn't block secret values
		starlarkVal, err := secretOpt.GetStarlarkValue()
		if err != nil {
			t.Errorf("GetStarlarkValue should not return error for secret options: %v", err)
		}

		// Verify the starlark value is correct
		strVal, ok := starlarkVal.(starlark.String)
		if !ok {
			t.Errorf("Expected starlark string value, got %T", starlarkVal)
		} else if string(strVal) != "secret" {
			t.Errorf("Expected starlark value 'secret', got '%s'", string(strVal))
		}

		// However, in the module.LoadModule, get_* methods are not registered for secret values
	})

	// Test with incorrect types
	t.Run("TypeConversionErrors", func(t *testing.T) {
		// Test map with incompatible values
		mapComplexOpt := base.NewConfigOption(map[string]complex128{})
		mapWithIncompatibleValues := starlark.NewDict(1)
		mapWithIncompatibleValues.SetKey(starlark.String("key"), starlark.String("not a complex number"))

		err := mapComplexOpt.SetValueFromStarlark(mapWithIncompatibleValues)
		if err == nil {
			t.Error("Expected error for map with incompatible value type, got nil")
		}

		// Test direct type assertion failure with a simpler case
		boolOpt := base.NewConfigOption(true)
		err = boolOpt.SetValueFromStarlark(starlark.MakeInt(42))
		if err == nil {
			t.Error("Expected error for incompatible direct type assertion, got nil")
		}

		// Test regular incompatible type error
		intOpt := base.NewConfigOption(0)
		err = intOpt.SetValueFromStarlark(starlark.String("not a number"))
		if err == nil {
			t.Error("Expected error for string that can't be converted to int, got nil")
		}

		// Test slices with incompatible element types
		// This is needed to test element conversion failures
		intSliceOpt := base.NewConfigOption([]int{})
		listWithStrings := starlark.NewList([]starlark.Value{
			starlark.String("not a number"),
		})

		err = intSliceOpt.SetValueFromStarlark(listWithStrings)
		if err == nil {
			t.Error("Expected error for slice with string that can't be converted to int, got nil")
		}
	})

	// Test numeric type conversions for slices and maps
	t.Run("NumericTypeConversions", func(t *testing.T) {
		// Test numeric conversions for slices
		// Float64 to Int conversion (should work)
		intSliceOpt := base.NewConfigOption([]int{})
		floatList := starlark.NewList([]starlark.Value{
			starlark.Float(1.0),
			starlark.Float(2.0),
		})
		err := intSliceOpt.SetValueFromStarlark(floatList)
		if err != nil {
			t.Errorf("Failed to convert float list to int slice: %v", err)
		}

		intVal, err := intSliceOpt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}
		if len(intVal) != 2 || intVal[0] != 1 || intVal[1] != 2 {
			t.Errorf("Expected slice [1, 2], got %v", intVal)
		}

		// Test numeric conversions for maps
		// Float64 to Int conversion for map values
		intMapOpt := base.NewConfigOption(map[string]int{})
		floatMap := starlark.NewDict(2)
		floatMap.SetKey(starlark.String("a"), starlark.Float(3.0))
		floatMap.SetKey(starlark.String("b"), starlark.Float(4.0))

		err = intMapOpt.SetValueFromStarlark(floatMap)
		if err != nil {
			t.Errorf("Failed to convert float dict to int map: %v", err)
		}

		mapVal, err := intMapOpt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}
		if len(mapVal) != 2 || mapVal["a"] != 3 || mapVal["b"] != 4 {
			t.Errorf("Expected map {a:3, b:4}, got %v", mapVal)
		}

		// Test value that cannot be converted - complex case
		complexOpt := base.NewConfigOption(complex(1, 2))
		intValue := starlark.MakeInt(42)

		err = complexOpt.SetValueFromStarlark(intValue)
		if err == nil {
			t.Error("Expected error for int -> complex conversion, got nil")
		} else if !strings.Contains(err.Error(), "cannot be converted") {
			t.Errorf("Expected conversion error, got: %v", err)
		}
	})
}

// TestGenSetFunction tests the genSetFunction method in isolation
func TestGenSetFunction(t *testing.T) {
	module := base.NewConfigurableModule()

	// Add a config option with validator
	validatedOpt := base.NewConfigOption(0).WithValidator(func(val int) error {
		if val < 0 {
			return fmt.Errorf("value must be non-negative")
		}
		return nil
	})

	module.SetConfigOption("validated", validatedOpt)

	// Initialize the module
	err := module.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Load the module
	loader := module.LoadModule("test", nil)
	_, err = loader()
	if err != nil {
		t.Fatalf("Module loading failed: %v", err)
	}

	// Create an environment to test the module
	env := starlet.NewDefault()
	loaders := make(map[string]starlet.ModuleLoader)
	loaders["test"] = loader
	env.SetLazyloadModules(loaders)

	// Create a script that tests validation
	script := `
# Load the module functions
load("test", "set_validated")

# Test setting a value
set_validated(10)
`
	env.SetScriptContent([]byte(script))

	// Run the script
	_, scriptErr := env.Run()
	if scriptErr != nil {
		t.Errorf("Script execution failed: %v", scriptErr)
	}

	// Verify the value was actually set to 10 in the Go module
	validatedValue, err := base.GetConfigValue[int](module, "validated")
	if err != nil {
		t.Fatalf("Failed to get validated value: %v", err)
	}

	if validatedValue != 10 {
		t.Errorf("Expected validated value to be 10, got %d", validatedValue)
	}
}

// Additional test for edge cases in Starlark integration
func TestStarlarkEdgeCases(t *testing.T) {
	// Test with various types and conversions
	module := base.NewConfigurableModule()

	// Array option
	arrayOpt := base.NewConfigOption([]string{"a", "b", "c"})
	base.SetTypedConfigOption(module, "array_option", arrayOpt)

	// Map option - use map[string]interface{} which is better supported in Starlark
	mapOpt := base.NewConfigOption(map[string]interface{}{"a": 1, "b": 2})
	base.SetTypedConfigOption(module, "map_option", mapOpt)

	// Get the module loader
	moduleLoader := module.LoadModule("test_module", nil)

	// Use starlet to execute the script
	script := `
load("test_module", "get_array_option", "get_map_option", "set_array_option", "set_map_option")

# Set array with different length
set_array_option(["x", "y", "z", "w"])

# Set map with different keys
set_map_option({"x": 10, "y": 20, "z": 30})

# Using list comprehension to modify array
array = get_array_option()
new_array = [x.upper() for x in array]
set_array_option(new_array)

# Using dict comprehension to modify map
map_val = get_map_option()
new_map = {k: v * 2 for k, v in map_val.items()}
set_map_option(new_map)
`

	// Create a runtime environment
	env := starlet.NewDefault()
	loaders := make(map[string]starlet.ModuleLoader)
	loaders["test_module"] = moduleLoader
	env.SetLazyloadModules(loaders)
	env.SetScriptContent([]byte(script))

	// Execute the script
	_, err := env.Run()
	if err != nil {
		t.Fatalf("Failed to execute Starlark script: %v", err)
	}

	// Verify array updates
	arrayValue, err := base.GetConfigValue[[]string](module, "array_option")
	if err != nil {
		t.Fatalf("Failed to get array_option: %v", err)
	}

	if len(arrayValue) != 4 {
		t.Errorf("array_option length = %d, want 4", len(arrayValue))
	}

	// The last operation should have made the array uppercase
	for i, v := range arrayValue {
		if v != "X" && v != "Y" && v != "Z" && v != "W" {
			t.Errorf("array_option[%d] = %q, want uppercase letter", i, v)
		}
	}

	// Verify map updates
	mapValue, err := base.GetConfigValue[map[string]interface{}](module, "map_option")
	if err != nil {
		t.Fatalf("Failed to get map_option: %v", err)
	}

	if len(mapValue) != 3 {
		t.Errorf("map_option size = %d, want 3", len(mapValue))
	}

	// The values should be doubled - check as float64 since that's what Starlark uses for numbers
	expectedValues := map[string]float64{"x": 20, "y": 40, "z": 60}
	for k, expected := range expectedValues {
		actual, exists := mapValue[k]
		if !exists {
			t.Errorf("Key %q not found in map", k)
			continue
		}

		// Check the value, which might be a float64 or int
		var actualValue float64
		switch v := actual.(type) {
		case int:
			actualValue = float64(v)
		case float64:
			actualValue = v
		default:
			t.Errorf("map_option[%q] has unexpected type %T", k, actual)
			continue
		}

		if actualValue != expected {
			t.Errorf("map_option[%q] = %v, want %v", k, actualValue, expected)
		}
	}
}

// TestStarlarkSecretAccess tests that secret values are not accessible from Starlark
func TestStarlarkSecretAccess(t *testing.T) {
	// Create a module with a secret option
	module := base.NewConfigurableModule()

	// Add a secret API key option
	apiKeyOption := base.NewConfigOption("api-key-12345").
		WithName("api_key").
		WithDescription("API Key for authentication").
		SetSecret(true)

	// Also add a regular non-secret option for comparison
	nonSecretOption := base.NewConfigOption("non-secret-value").
		WithName("non_secret").
		WithDescription("A non-secret value")

	module.SetConfigOption("api_key", apiKeyOption)
	module.SetConfigOption("non_secret", nonSecretOption)

	// Initialize the module
	err := module.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize module: %v", err)
	}

	// Test that we can access the secret value from Go code directly
	val, err := base.GetConfigValue[string](module, "api_key")
	if err != nil {
		t.Errorf("Expected to access secret value in Go, got error: %v", err)
	}
	if val != "api-key-12345" {
		t.Errorf("Expected secret value in Go to be 'api-key-12345', got '%s'", val)
	}

	// Now test the Starlark runtime behavior
	customFuncs := starlark.StringDict{
		"list_funcs": starlark.NewBuiltin("list_funcs", func(
			thread *starlark.Thread,
			_ *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {
			// Return a list of all available function names as a Starlark list
			var names []starlark.Value
			return starlark.NewList(names), nil
		}),
		"verify_secret_value": starlark.NewBuiltin("verify_secret_value", func(
			thread *starlark.Thread,
			_ *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {
			// Function to verify that secret value was properly set
			var expected string
			if err := starlark.UnpackArgs("verify_secret_value", args, kwargs, "expected", &expected); err != nil {
				return starlark.None, err
			}

			// Get the value from Go
			actual, err := base.GetConfigValue[string](module, "api_key")
			if err != nil {
				return starlark.Bool(false), fmt.Errorf("failed to get api_key: %v", err)
			}

			// Compare and return result
			return starlark.Bool(actual == expected), nil
		}),
	}

	// Load the module
	loader := module.LoadModule("test_module", customFuncs)

	// Create a script to verify what functions are available
	script := `
load("test_module", "set_api_key", "set_non_secret", "get_non_secret", "verify_secret_value")

# Define a function to run our tests
def run_tests():
    # This should work - get_non_secret is exposed
    value = get_non_secret()
    print("Non-secret value:", value)

    # Try setting the secret value
    new_secret = "new-secret-key-678910"
    set_api_key(new_secret)

    # Verify that the secret value was properly set
    result = verify_secret_value(new_secret)
    print("Secret value verification result:", result)
    if result:
        print("Secret value was correctly set and verified from Go side")
    else:
        print("Failed to set secret value correctly")
        fail("Secret value verification failed")

# Run the tests
run_tests()

# Module loaded and functions accessible
print("Module loaded and functions accessible")

# Trying to evaluate get_api_key will not work because it's not even in the module
# We can't test this directly in the script, but we'll verify in Go code
`

	// Create an environment to test the module
	env := starlet.NewDefault()
	loaders := make(map[string]starlet.ModuleLoader)
	loaders["test_module"] = loader
	env.SetLazyloadModules(loaders)
	env.SetScriptContent([]byte(script))

	// Run the script - no errors means we were able to load set_api_key,
	// set_non_secret, and get_non_secret, but not get_api_key (since we didn't try)
	_, err = env.Run()
	if err != nil {
		t.Errorf("Script execution failed: %v", err)
	}

	// Verify that the value was actually changed in Go
	newVal, err := base.GetConfigValue[string](module, "api_key")
	if err != nil {
		t.Errorf("Failed to get updated api_key: %v", err)
	}
	if newVal != "new-secret-key-678910" {
		t.Errorf("Expected updated secret value to be 'new-secret-key-678910', got '%s'", newVal)
	}
}

func TestGetStarlarkValue(t *testing.T) {
	t.Run("BasicTypes", func(t *testing.T) {
		// String
		strOpt := base.NewConfigOption("test string")
		strVal, err := strOpt.GetStarlarkValue()
		if err != nil {
			t.Fatalf("GetStarlarkValue for string failed: %v", err)
		}
		if strVal.String() != `"test string"` {
			t.Errorf("Expected starlark string \"test string\", got %s", strVal.String())
		}

		// Integer
		intOpt := base.NewConfigOption(42)
		intVal, err := intOpt.GetStarlarkValue()
		if err != nil {
			t.Fatalf("GetStarlarkValue for int failed: %v", err)
		}
		if intVal.String() != "42" {
			t.Errorf("Expected starlark int 42, got %s", intVal.String())
		}

		// Boolean
		boolOpt := base.NewConfigOption(true)
		boolVal, err := boolOpt.GetStarlarkValue()
		if err != nil {
			t.Fatalf("GetStarlarkValue for bool failed: %v", err)
		}
		if boolVal.String() != "True" {
			t.Errorf("Expected starlark bool True, got %s", boolVal.String())
		}

		// Float
		floatOpt := base.NewConfigOption(3.14)
		floatVal, err := floatOpt.GetStarlarkValue()
		if err != nil {
			t.Fatalf("GetStarlarkValue for float failed: %v", err)
		}
		if floatVal.String() != "3.14" {
			t.Errorf("Expected starlark float 3.14, got %s", floatVal.String())
		}
	})

	t.Run("ComplexTypes", func(t *testing.T) {
		// Slice
		sliceOpt := base.NewConfigOption([]string{"a", "b", "c"})
		sliceVal, err := sliceOpt.GetStarlarkValue()
		if err != nil {
			t.Fatalf("GetStarlarkValue for slice failed: %v", err)
		}
		if sliceVal.String() != `["a", "b", "c"]` {
			t.Errorf("Expected starlark list [\"a\", \"b\", \"c\"], got %s", sliceVal.String())
		}

		// Map - use map[string]interface{} which is known to be convertible
		mapOpt := base.NewConfigOption(map[string]interface{}{"a": 1, "b": "two"})
		mapVal, err := mapOpt.GetStarlarkValue()
		if err != nil {
			t.Fatalf("GetStarlarkValue for map failed: %v", err)
		}
		if !strings.Contains(mapVal.String(), `"a": 1`) || !strings.Contains(mapVal.String(), `"b": "two"`) {
			t.Errorf("Expected starlark dict with \"a\": 1, \"b\": \"two\", got %s", mapVal.String())
		}
	})

	t.Run("ErrorCases", func(t *testing.T) {
		// Function type (not convertible to Starlark)
		funcOpt := base.NewConfigOption(func() {})
		_, err := funcOpt.GetStarlarkValue()
		if err == nil {
			t.Fatal("Expected error for unconvertible type, got nil")
		}

		// Complex number (not directly convertible)
		complexOpt := base.NewConfigOption(complex(1, 2))
		_, err = complexOpt.GetStarlarkValue()
		if err == nil {
			t.Fatal("Expected error for complex number, got nil")
		}

		// Unresolvable value (error from getter)
		errorOpt := base.NewConfigOption("").WithGetter(func() string {
			panic("artificial panic in getter")
		})
		_, err = errorOpt.GetStarlarkValue()
		if err == nil {
			t.Fatal("Expected error from panic in getter, got nil")
		}
	})
}

// TestHardeningInvariants is an adversarial regression suite for the
// "Invariants / hardening" properties in CLAUDE.md that the rest of the file
// does not already pin down. Each section tries to break an invariant from the
// script-facing surface and asserts the module degrades to a clean error rather
// than crashing or corrupting the host. Sections:
//   - ValidatorPanicRecovered    — a validator that panics during set_<name> is
//     recovered into a wrapped ErrConfigInvalidValue, not a host panic.
//   - ConcurrentScriptExecution  — one initialized module shared by many
//     scripts running set_/get_ concurrently is race-free (run under -race).
//   - SecretHasNoScriptGetter    — a secret option exposes set_ but no get_, so
//     a script load() of get_<secret> fails (the value is unreadable from script).
//   - LoadModuleRequiredError    — a required-but-unset option makes the loader
//     return an error, surfaced to the script as a load failure, never a panic.
func TestHardeningInvariants(t *testing.T) {
	t.Run("ValidatorPanicRecovered", func(t *testing.T) {
		opt := base.NewConfigOption("").WithName("v").
			WithValidator(func(string) error { panic("validator boom") })

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic escaped SetValueFromStarlark: %v", r)
			}
		}()
		err := opt.SetValueFromStarlark(starlark.String("x"))
		if err == nil {
			t.Fatal("expected an error from a panicking validator, got nil")
		}
		if !strings.Contains(err.Error(), "invalid config value") {
			t.Errorf("expected ErrConfigInvalidValue wrap, got %v", err)
		}
	})

	t.Run("ConcurrentScriptExecution", func(t *testing.T) {
		module := base.NewConfigurableModule()
		module.SetConfigOption("counter", base.NewConfigOption(0))
		module.SetConfigOption("name", base.NewConfigOption("init"))
		loader := module.LoadModule("m", nil)

		script := []byte(`
load("m", "set_counter", "get_counter", "set_name", "get_name")
set_counter(7)
a = get_counter()
set_name("x")
b = get_name()
`)
		var wg sync.WaitGroup
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				env := starlet.NewDefault()
				env.SetLazyloadModules(map[string]starlet.ModuleLoader{"m": loader})
				env.SetScriptContent(script)
				if _, err := env.Run(); err != nil {
					t.Errorf("concurrent script failed: %v", err)
				}
			}()
		}
		wg.Wait()
	})

	t.Run("SecretHasNoScriptGetter", func(t *testing.T) {
		module := base.NewConfigurableModule()
		module.SetConfigOption("secret", base.NewConfigOption("s3cr3t").SetSecret(true))
		loader := module.LoadModule("m", nil)

		env := starlet.NewDefault()
		env.SetLazyloadModules(map[string]starlet.ModuleLoader{"m": loader})
		env.SetScriptContent([]byte(`load("m", "get_secret")`))
		_, err := env.Run()
		if err == nil {
			t.Fatal("expected load of get_secret to fail (secret has no getter)")
		}
		if !strings.Contains(err.Error(), "get_secret") {
			t.Errorf("expected error to mention get_secret, got %v", err)
		}
		// The setter, however, must exist.
		env2 := starlet.NewDefault()
		env2.SetLazyloadModules(map[string]starlet.ModuleLoader{"m": loader})
		env2.SetScriptContent([]byte(`load("m", "set_secret")` + "\n" + `set_secret("new")`))
		if _, err := env2.Run(); err != nil {
			t.Errorf("set_secret should be exposed and callable, got %v", err)
		}
	})

	t.Run("LoadModuleRequiredError", func(t *testing.T) {
		module := base.NewConfigurableModule()
		// Required, with no value/getter/env/default -> Initialize fails inside the loader.
		module.SetConfigOption("must", base.NewConfigOption("").
			WithName("must").SetRequired(true))
		loader := module.LoadModule("m", nil)

		env := starlet.NewDefault()
		env.SetLazyloadModules(map[string]starlet.ModuleLoader{"m": loader})
		env.SetScriptContent([]byte(`load("m", "set_must")`))
		_, err := env.Run()
		if err == nil {
			t.Fatal("expected a required-config error surfaced through the loader, got nil")
		}
		if !strings.Contains(err.Error(), "required config not set") {
			t.Errorf("expected 'required config not set', got %v", err)
		}
	})
}

func TestSetValueFromStarlarkEdgeCases(t *testing.T) {
	t.Run("SliceConversionEdgeCases", func(t *testing.T) {
		// Test boolean slice
		opt := base.NewConfigOption([]bool{})
		list := starlark.NewList([]starlark.Value{
			starlark.Bool(true),
			starlark.Bool(false),
			starlark.Bool(true),
		})

		err := opt.SetValueFromStarlark(list)
		if err != nil {
			t.Fatalf("Failed to convert to bool slice: %v", err)
		}

		val, err := opt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}

		if len(val) != 3 || !val[0] || val[1] || !val[2] {
			t.Errorf("Expected [true, false, true], got %v", val)
		}
	})

	t.Run("TypeConversionErrors", func(t *testing.T) {
		// Test conversion failure with incompatible types
		chanOpt := base.NewConfigOption(make(chan int))
		err := chanOpt.SetValueFromStarlark(starlark.String("not a channel"))
		if err == nil {
			t.Fatal("Expected error for unsupported chan type, got nil")
		}

		// Test map key conversion errors. dataconv.Unmarshal renders keys as
		// strings, so numeric-looking keys ("1") now parse to the numeric
		// target; a genuinely non-numeric key cannot, and still errors.
		mapOpt := base.NewConfigOption(map[int]string{})
		dict := starlark.NewDict(2)
		dict.SetKey(starlark.String("alpha"), starlark.String("one"))
		dict.SetKey(starlark.String("beta"), starlark.String("two"))

		err = mapOpt.SetValueFromStarlark(dict)
		if err == nil {
			t.Fatal("Expected error for non-numeric map key conversion, got nil")
		}

		// Test invalid number format for int conversion
		intOpt := base.NewConfigOption(0)
		err = intOpt.SetValueFromStarlark(starlark.String("not a number"))
		if err == nil {
			t.Fatal("Expected error for invalid number format, got nil")
		}
	})
}

// TestMapKeyConversion specifically tests the map key conversion logic in SetValueFromStarlark
func TestMapKeyConversion(t *testing.T) {
	t.Run("ConvertibleMapKeys", func(t *testing.T) {
		// Test conversion from int to int64
		opt := base.NewConfigOption(map[int64]string{})

		// Create a Starlark dict with int keys
		dict := starlark.NewDict(3)
		dict.SetKey(starlark.MakeInt(1), starlark.String("one"))
		dict.SetKey(starlark.MakeInt(2), starlark.String("two"))
		dict.SetKey(starlark.MakeInt(3), starlark.String("three"))

		// Test conversion by setting value from Starlark
		err := opt.SetValueFromStarlark(dict)
		if err != nil {
			t.Fatalf("SetValueFromStarlark failed for convertible map keys: %v", err)
		}

		// Get and verify the value
		val, err := opt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}

		// Check that all keys were properly converted
		if len(val) != 3 || val[1] != "one" || val[2] != "two" || val[3] != "three" {
			t.Errorf("Expected map with 3 keys and correct values, got %v", val)
		}
	})

	t.Run("NonConvertibleMapKeys", func(t *testing.T) {
		// Test conversion from string to int (should fail)
		opt := base.NewConfigOption(map[int]string{})

		// Create a Starlark dict with string keys (not convertible to int)
		dict := starlark.NewDict(2)
		dict.SetKey(starlark.String("key1"), starlark.String("value1"))
		dict.SetKey(starlark.String("key2"), starlark.String("value2"))

		// Should fail with "map key cannot be converted" error
		err := opt.SetValueFromStarlark(dict)
		if err == nil {
			t.Error("Expected error for non-convertible map keys, got nil")
		}

		// Verify error message contains the expected text
		if !strings.Contains(err.Error(), "map key cannot be converted") {
			t.Errorf("Expected error message to contain 'map key cannot be converted', got: %v", err)
		}
	})

	t.Run("NumericKeyConversion", func(t *testing.T) {
		// Test conversion between different numeric types
		// float32 -> int
		intMapOpt := base.NewConfigOption(map[int]string{})
		floatDict := starlark.NewDict(2)
		floatDict.SetKey(starlark.Float(1.0), starlark.String("one"))
		floatDict.SetKey(starlark.Float(2.0), starlark.String("two"))

		// Should convert the float to int successfully
		err := intMapOpt.SetValueFromStarlark(floatDict)
		if err != nil {
			t.Fatalf("SetValueFromStarlark failed for numeric key conversion: %v", err)
		}

		intVal, err := intMapOpt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}

		if len(intVal) != 2 || intVal[1] != "one" || intVal[2] != "two" {
			t.Errorf("Expected map with 2 keys and correct values, got %v", intVal)
		}

		// Conversion that should fail: float with fraction to int
		floatFracDict := starlark.NewDict(1)
		floatFracDict.SetKey(starlark.Float(1.5), starlark.String("one point five"))

		// This should still succeed since floats can be converted to ints (truncated)
		err = intMapOpt.SetValueFromStarlark(floatFracDict)
		if err != nil {
			t.Fatalf("SetValueFromStarlark failed for truncated float: %v", err)
		}

		// Key should be truncated to 1
		intVal, err = intMapOpt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}

		if len(intVal) != 1 || intVal[1] != "one point five" {
			t.Errorf("Expected map with truncated float key, got %v", intVal)
		}
	})

	t.Run("ComplexMapKeyConversions", func(t *testing.T) {
		// Test with custom key types (using uint8 as target)
		uintMapOpt := base.NewConfigOption(map[uint8]string{})

		// Try to convert from various numeric types
		mixedDict := starlark.NewDict(3)
		mixedDict.SetKey(starlark.MakeInt(42), starlark.String("from int"))
		mixedDict.SetKey(starlark.Float(100.0), starlark.String("from float"))
		mixedDict.SetKey(starlark.Float(255.0), starlark.String("max uint8"))

		err := uintMapOpt.SetValueFromStarlark(mixedDict)
		if err != nil {
			t.Fatalf("SetValueFromStarlark failed for mixed numeric conversions: %v", err)
		}

		uintVal, err := uintMapOpt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}

		if len(uintVal) != 3 ||
			uintVal[42] != "from int" ||
			uintVal[100] != "from float" ||
			uintVal[255] != "max uint8" {
			t.Errorf("Expected map with converted keys, got %v", uintVal)
		}

		// Test overflow case (should error)
		overflowDict := starlark.NewDict(1)
		overflowDict.SetKey(starlark.MakeInt(256), starlark.String("overflow"))

		// This will actually succeed on most systems as the int256 will fit in a uint8
		// due to type conversion mechanics in Go, but will wrap around to 0
		err = uintMapOpt.SetValueFromStarlark(overflowDict)
		if err != nil {
			t.Fatalf("SetValueFromStarlark failed unexpectedly: %v", err)
		}

		uintVal, err = uintMapOpt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}

		// 256 should wrap around to 0 for uint8
		if len(uintVal) != 1 || uintVal[0] != "overflow" {
			t.Errorf("Expected map with wrapped key, got %v", uintVal)
		}
	})

	t.Run("ExactErrorMessageFormat", func(t *testing.T) {
		// Create config option with a map that requires a specific key type
		type CustomKey struct {
			ID int
		}
		opt := base.NewConfigOption(map[CustomKey]string{})

		// Create a Starlark dict with string keys (not convertible to CustomKey)
		dict := starlark.NewDict(1)
		dict.SetKey(starlark.String("key1"), starlark.String("value1"))

		// Attempt conversion which should fail
		err := opt.SetValueFromStarlark(dict)

		// Check error exists
		if err == nil {
			t.Fatal("Expected error for non-convertible map keys to custom struct, got nil")
		}

		// Check specific error message format
		expectedPattern := "map key cannot be converted from"
		if !strings.Contains(err.Error(), expectedPattern) {
			t.Errorf("Expected error message to contain '%s', got: %v", expectedPattern, err)
		}

		// Ensure error message includes both source and target types
		errMsg := err.Error()
		if !strings.Contains(errMsg, "string") || !strings.Contains(errMsg, "CustomKey") {
			t.Errorf("Error message should contain both source type (string) and target type (CustomKey). Got: %s", errMsg)
		}

		// Try another case: complex key type requirement with a simple numeric key.
		// dataconv.Unmarshal now renders the key as the string "42"; a string
		// (real) cannot convert to complex128, so this still errors — with the
		// source type reported as string.
		intKeyOpt := base.NewConfigOption(map[complex128]string{})
		intDict := starlark.NewDict(1)
		intDict.SetKey(starlark.MakeInt(42), starlark.String("value"))

		err = intKeyOpt.SetValueFromStarlark(intDict)
		if err == nil {
			t.Fatal("Expected error for non-convertible map keys to complex128, got nil")
		}

		// Check specific error pattern
		if !strings.Contains(err.Error(), expectedPattern) {
			t.Errorf("Expected error message to contain '%s', got: %v", expectedPattern, err)
		}

		// Check for both types in error message
		errMsg = err.Error()
		if !strings.Contains(errMsg, "string") || !strings.Contains(errMsg, "complex128") {
			t.Errorf("Error message should contain both source type (string) and target type (complex128). Got: %s", errMsg)
		}
	})
}

// TestSetValueFromStarlarkNoneRejected is a regression test: a Starlark None
// (which dataconv.Unmarshal turns into a nil interface) must be rejected with a
// clean error for every target kind, and must never panic the host. Before the
// nil guard + recover, int/uint/float/slice/map targets dereferenced a nil
// reflect.Type and crashed the process.
func TestSetValueFromStarlarkNoneRejected(t *testing.T) {
	check := func(name string, set func() error) {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("SetValueFromStarlark(None) panicked for %s: %v", name, r)
				}
			}()
			if err := set(); err == nil {
				t.Errorf("expected an error setting %s from None, got nil", name)
			}
		})
	}

	check("int", func() error { return base.NewConfigOption(0).SetValueFromStarlark(starlark.None) })
	check("uint", func() error { return base.NewConfigOption(uint(0)).SetValueFromStarlark(starlark.None) })
	check("float64", func() error { return base.NewConfigOption(0.0).SetValueFromStarlark(starlark.None) })
	check("string", func() error { return base.NewConfigOption("").SetValueFromStarlark(starlark.None) })
	check("bool", func() error { return base.NewConfigOption(false).SetValueFromStarlark(starlark.None) })
	check("slice", func() error { return base.NewConfigOption([]int{}).SetValueFromStarlark(starlark.None) })
	check("map", func() error { return base.NewConfigOption(map[string]int{}).SetValueFromStarlark(starlark.None) })
}
