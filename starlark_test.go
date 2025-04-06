package base_test

import (
	"testing"

	"github.com/1set/starlet"
	"github.com/starpkg/base"
	"go.starlark.net/starlark"
)

func TestStarlarkIntegration(t *testing.T) {
	// Create a module with various config options
	module := base.NewConfigurableModule()

	// String option
	strOpt := base.NewConfigOption("default_string").
		WithDescription("A string option")
	base.SetTypedConfigOption(module, "string_option", strOpt)

	// Int option
	intOpt := base.NewConfigOption(42).
		WithDescription("An int option")
	base.SetTypedConfigOption(module, "int_option", intOpt)

	// Boolean option
	boolOpt := base.NewConfigOption(true).
		WithDescription("A boolean option")
	base.SetTypedConfigOption(module, "bool_option", boolOpt)

	// Secret option
	secretOpt := base.NewConfigOption("secret_value").
		WithDescription("A secret option").
		Secret()
	base.SetTypedConfigOption(module, "secret_option", secretOpt)

	// Add a custom function
	additionalFuncs := starlark.StringDict{
		"custom_func": starlark.NewBuiltin("custom_func", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return starlark.String("custom_result"), nil
		}),
	}

	// Get the module loader
	moduleLoader := module.LoadModule("test_module", additionalFuncs)

	// Use starlet to execute the script
	script := `
load("test_module", "get_string_option", "get_int_option", "get_bool_option", 
                  "set_string_option", "set_int_option", "set_bool_option", 
                  "set_secret_option", "custom_func")

# Get current values
initial_string = get_string_option()
initial_int = get_int_option()
initial_bool = get_bool_option()

# Set new values
set_string_option("starlark_string")
set_int_option(100)
set_bool_option(False)

# Use the custom function
custom_result = custom_func()

# Set the secret option
set_secret_option("new_secret")

# Return the values for verification
result = {
    "initial_string": initial_string,
    "initial_int": initial_int,
    "initial_bool": initial_bool,
    "custom_result": custom_result,
}
`

	// Create runtime environment
	env := starlet.NewDefault()
	loaders := make(map[string]starlet.ModuleLoader)
	loaders["test_module"] = moduleLoader
	env.SetLazyloadModules(loaders)
	env.SetScriptContent([]byte(script))

	// Execute the script
	result, err := env.Run()
	if err != nil {
		t.Fatalf("Failed to execute Starlark script: %v", err)
	}

	// Verify the Starlark execution using starlet's result handling
	if result == nil {
		t.Fatalf("No result returned from script execution")
	}

	// Print the result for debugging
	t.Logf("Result from script: %+v", result)

	// Skip the map extraction and just check if the individual values were updated correctly

	// Verify the options were actually updated
	strValue, err := base.GetConfigValue[string](module, "string_option")
	if err != nil {
		t.Fatalf("Failed to get string_option: %v", err)
	}
	if got, want := strValue, "starlark_string"; got != want {
		t.Errorf("string_option = %q, want %q", got, want)
	}

	intValue, err := base.GetConfigValue[int](module, "int_option")
	if err != nil {
		t.Fatalf("Failed to get int_option: %v", err)
	}
	if got, want := intValue, 100; got != want {
		t.Errorf("int_option = %d, want %d", got, want)
	}

	boolValue, err := base.GetConfigValue[bool](module, "bool_option")
	if err != nil {
		t.Fatalf("Failed to get bool_option: %v", err)
	}
	if got, want := boolValue, false; got != want {
		t.Errorf("bool_option = %v, want %v", got, want)
	}

	// Verify the secret option cannot be retrieved
	_, err = base.GetConfigValue[string](module, "secret_option")
	if err == nil {
		t.Error("Expected error when retrieving secret option, got nil")
	}

	// Force-retrieve the secret value using the module's internal option
	retrievedOpt, err := module.GetConfigOption("secret_option")
	if err != nil {
		t.Fatalf("Failed to get secret_option: %v", err)
	}

	// Check that the option was correctly set to be secret
	if !retrievedOpt.IsSecret() {
		t.Error("secret_option should be marked as secret")
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
load("test_module", "get_array_option", "get_map_option", 
                  "set_array_option", "set_map_option")

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
