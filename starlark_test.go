package base_test

import (
	"errors"
	"fmt"
	"strings"
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
		dict, err := loader()
		if err != nil {
			t.Fatalf("Module loading failed: %v", err)
		}

		// Test setter and getter functions
		setString, ok := dict["set_string_opt"].(starlark.Callable)
		if !ok {
			t.Fatal("set_string_opt should be a Callable")
		}

		getString, ok := dict["get_string_opt"].(starlark.Callable)
		if !ok {
			t.Fatal("get_string_opt should be a Callable")
		}

		// Call the setter
		_, err = setString.CallInternal(nil, starlark.Tuple{starlark.String("new_value")}, nil)
		if err != nil {
			t.Fatalf("Failed to call set_string_opt: %v", err)
		}

		// Call the getter to verify the value was set
		result, err := getString.CallInternal(nil, nil, nil)
		if err != nil {
			t.Fatalf("Failed to call get_string_opt: %v", err)
		}

		if result.String() != `"new_value"` {
			t.Errorf("Expected \"new_value\", got %s", result.String())
		}

		// Test invalid calls
		_, err = setString.CallInternal(nil, starlark.Tuple{}, nil) // Missing argument
		if err == nil {
			t.Error("Expected error for missing argument, got nil")
		}

		_, err = setString.CallInternal(nil, starlark.Tuple{starlark.MakeInt(123)}, nil) // Wrong type
		if err == nil {
			t.Error("Expected error for wrong type, got nil")
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
	})

	// Test edge cases
	t.Run("EdgeCases", func(t *testing.T) {
		// Test setting a value that can't be converted
		opt := base.NewConfigOption(struct{}{})
		err := opt.SetValueFromStarlark(starlark.String("can't convert to struct"))
		if err == nil {
			t.Error("Expected error when setting value of incompatible type, got nil")
		}

		// Test GetStarlarkValue for nil or error cases
		secretOpt := base.NewConfigOption("secret").SetSecret(true)
		_, err = secretOpt.GetStarlarkValue()
		if err == nil {
			t.Error("Expected error when getting value of secret option, got nil")
		}
		if !errors.Is(err, base.ErrSecretConfigNotRetrievable) {
			t.Errorf("Expected ErrSecretConfigNotRetrievable, got %v", err)
		}
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
	dict, err := loader()
	if err != nil {
		t.Fatalf("Module loading failed: %v", err)
	}

	// Get the setter function
	setValidated, ok := dict["set_validated"].(starlark.Callable)
	if !ok {
		t.Fatal("set_validated should be a Callable")
	}

	// Try setting a valid value
	_, err = setValidated.CallInternal(nil, starlark.Tuple{starlark.MakeInt(10)}, nil)
	if err != nil {
		t.Errorf("Failed to set a valid value: %v", err)
	}

	// Try setting an invalid value
	_, err = setValidated.CallInternal(nil, starlark.Tuple{starlark.MakeInt(-10)}, nil)
	if err == nil {
		t.Error("Expected error when setting invalid value, got nil")
	}

	// Try calling with wrong number of arguments
	_, err = setValidated.CallInternal(nil, starlark.Tuple{}, nil)
	if err == nil {
		t.Error("Expected error when calling with no arguments, got nil")
	}

	// Try calling with extra named arguments
	kwargs := []starlark.Tuple{
		{starlark.String("extra"), starlark.String("arg")},
	}
	_, err = setValidated.CallInternal(nil, starlark.Tuple{starlark.MakeInt(5)}, kwargs)
	if err == nil {
		t.Error("Expected error when calling with extra named arguments, got nil")
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
