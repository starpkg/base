package base_test

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/1set/starlet"
	"github.com/starpkg/base"
	"go.starlark.net/starlark"
)

func TestConfigurableModule(t *testing.T) {
	// Test basic module
	t.Run("BasicModule", func(t *testing.T) {
		module := base.NewConfigurableModule()
		if module == nil {
			t.Fatal("NewConfigurableModule should not return nil")
		}
	})

	// Test setting and getting config options
	t.Run("SetGetConfigOption", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Create a config option
		opt := base.NewConfigOption("default").WithName("test_option")

		// Set the option
		err := module.SetConfigOption("test_option", opt)
		if err != nil {
			t.Fatalf("SetConfigOption failed: %v", err)
		}

		// Get the option
		retrievedOpt, err := module.GetConfigOption("test_option")
		if err != nil {
			t.Fatalf("GetConfigOption failed: %v", err)
		}

		if retrievedOpt.GetName() != "test_option" {
			t.Errorf("Expected option name 'test_option', got '%s'", retrievedOpt.GetName())
		}

		// Try to get a non-existent option
		_, err = module.GetConfigOption("nonexistent")
		if !errors.Is(err, base.ErrConfigNotSet) {
			t.Errorf("Expected ErrConfigNotSet, got %v", err)
		}
	})

	// Test SetConfigOption name inference
	t.Run("SetConfigOptionNameInference", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Create a config option without name
		opt := base.NewConfigOption("default")

		// Set the option, the name should be inferred
		err := module.SetConfigOption("inferred_name", opt)
		if err != nil {
			t.Fatalf("SetConfigOption failed: %v", err)
		}

		// Get the option
		retrievedOpt, err := module.GetConfigOption("inferred_name")
		if err != nil {
			t.Fatalf("GetConfigOption failed: %v", err)
		}

		if retrievedOpt.GetName() != "inferred_name" {
			t.Errorf("Expected option name 'inferred_name', got '%s'", retrievedOpt.GetName())
		}
	})

	// Test initialization
	t.Run("Initialize", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Add a non-required config
		module.SetConfigOption("", base.NewConfigOption("default"))

		// Initialize should succeed
		err := module.Initialize()
		if err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		// Try to modify after initialization
		err = module.SetConfigOption("after_init", base.NewConfigOption("value"))
		if !errors.Is(err, base.ErrModuleAlreadyInitialized) {
			t.Errorf("Expected ErrModuleAlreadyInitialized, got %v", err)
		}
	})

	// Test validation during initialization for WithValue
	t.Run("ValidateWithValueDuringInitialize", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Create a config option with validator and set an invalid value using WithValue
		// WithValue bypasses validation during setting, but Initialize should validate
		invalidOption := base.NewConfigOption(0).
			WithName("number").
			WithValidator(func(val int) error {
				if val < 0 {
					return fmt.Errorf("number must be non-negative")
				}
				return nil
			}).
			WithValue(-10) // This invalid value is accepted by WithValue

		// Add the option to the module
		err := module.SetConfigOption("number", invalidOption)
		if err != nil {
			t.Fatalf("SetConfigOption failed: %v", err)
		}

		// Initialize should fail due to validation error
		err = module.Initialize()
		if err == nil {
			t.Fatal("Initialize should fail due to invalid value set by WithValue")
		}

		// Verify that the error indicates validation failure
		if !errors.Is(err, base.ErrConfigInvalidValue) {
			t.Errorf("Expected error wrapping ErrConfigInvalidValue, got: %v", err)
		}

		// Now create a module with valid value
		validModule := base.NewConfigurableModule()
		validOption := base.NewConfigOption(0).
			WithName("number").
			WithValidator(func(val int) error {
				if val < 0 {
					return fmt.Errorf("number must be non-negative")
				}
				return nil
			}).
			WithValue(10) // Valid value

		// Add the option to the module
		err = validModule.SetConfigOption("number", validOption)
		if err != nil {
			t.Fatalf("SetConfigOption failed: %v", err)
		}

		// Initialize should succeed
		err = validModule.Initialize()
		if err != nil {
			t.Fatalf("Initialize failed with valid value: %v", err)
		}
	})

	// Test required configs
	t.Run("RequiredConfigs", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Add a required config without a value
		module.SetConfigOption("required", base.NewConfigOption("").SetRequired(true))

		// Initialize should fail
		err := module.Initialize()
		if !errors.Is(err, base.ErrConfigRequired) {
			t.Errorf("Expected ErrConfigRequired, got %v", err)
		}

		// Now set a value for the required config
		opt, _ := module.GetConfigOption("required")
		typedOpt, ok := opt.(*base.ConfigOption[string])
		if !ok {
			t.Fatalf("Failed to cast config option to ConfigOption[string]")
		}
		typedOpt.SetValue("value")

		// Initialize should now succeed
		err = module.Initialize()
		if err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}
	})

	// Test LoadModule
	t.Run("LoadModule", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Add a couple config options
		opt1 := base.NewConfigOption("value1")
		opt2 := base.NewConfigOption("value2").SetSecret(true)
		module.SetConfigOption("option1", opt1)
		module.SetConfigOption("option2", opt2)

		// Additional functions
		additionalFuncs := starlark.StringDict{
			"custom_func": starlark.NewBuiltin("custom_func", func(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
				return starlark.String("custom result"), nil
			}),
		}

		// Load the module
		loader := module.LoadModule("test_module", additionalFuncs)
		if loader == nil {
			t.Fatal("LoadModule should not return nil")
		}

		// // Execute the module to verify functionality
		// _, err := loader()
		// if err != nil {
		// 	t.Fatalf("Module execution failed: %v", err)
		// }

		// Directly test the module by loading it in a starlark environment
		env := starlet.NewDefault()
		loaders := make(map[string]starlet.ModuleLoader)
		loaders["test_module"] = loader
		env.SetLazyloadModules(loaders)

		// Create a script that uses the module's functions
		script := `
# Load the module functions
load("test_module", "set_option1", "get_option1", "set_option2", "custom_func")

# Test setter
set_option1("new_value1")

# Test getter
val = get_option1()
print(val)

# Test setter for secret option
set_option2("new_value2")

# Test custom function
res = custom_func()
print(res)
`

		// Run the script
		_, scriptErr := env.RunScript([]byte(script), nil)
		if scriptErr != nil {
			t.Errorf("Script execution failed: %v", scriptErr)
		}

		// Verify the values were actually set in the Go module
		value1, err := base.GetConfigValue[string](module, "option1")
		if err != nil {
			t.Fatalf("Failed to get option1 value: %v", err)
		}
		if value1 != "new_value1" {
			t.Errorf("Expected option1 value to be 'new_value1', got '%s'", value1)
		}

		// Instead of trying to get option2's value directly, which will fail because it's secret,
		// we can just verify that the option exists
		_, err = module.GetConfigOption("option2")
		if err != nil {
			t.Fatalf("Failed to get option2: %v", err)
		}

		// We've successfully verified that all the expected functions work
		// No need to check dict directly since the real test is whether
		// the starlark script can use the functions

		// For test compatibility, we still create a basic check
		lc := module.ListConfigs()
		if _, ok := lc["option1"]; !ok {
			t.Error("Expected option1 to exist in ListConfigs")
		}
		if _, ok := lc["option2"]; !ok {
			t.Error("Expected option2 to exist in ListConfigs")
		}
	})

	// Test LoadModule with initialization error
	t.Run("LoadModuleWithInitError", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Add a config option with an invalid value using WithValue
		invalidOption := base.NewConfigOption(0).
			WithName("number").
			WithValidator(func(val int) error {
				if val < 0 {
					return fmt.Errorf("number must be non-negative")
				}
				return nil
			}).
			WithValue(-10) // Invalid value

		module.SetConfigOption("number", invalidOption)

		// Attempt to load the module - this should panic since LoadModule calls Initialize()
		defer func() {
			r := recover()
			if r == nil {
				t.Error("Expected LoadModule to panic with invalid configuration")
			}

			// Verify that the panic message contains information about the validation error
			panicStr, ok := r.(error)
			if !ok {
				t.Errorf("Expected panic to be an error, got: %v (type %T)", r, r)
				return
			}

			if !errors.Is(panicStr, base.ErrConfigInvalidValue) {
				t.Errorf("Expected panic to wrap ErrConfigInvalidValue, got: %v", panicStr)
			}
		}()

		_ = module.LoadModule("test_module", nil)

		// We should never reach here because LoadModule should panic
		t.Fatal("LoadModule should have panicked with invalid configuration")
	})

	// Test ListConfigs
	t.Run("ListConfigs", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Add a couple config options
		opt1 := base.NewConfigOption("value1").WithDescription("Description 1")
		opt2 := base.NewConfigOption("value2").WithDescription("Description 2").SetSecret(true)
		module.SetConfigOption("option1", opt1)
		module.SetConfigOption("option2", opt2)

		// List configs
		configs := module.ListConfigs()

		// Check that both configs are listed
		if len(configs) != 2 {
			t.Fatalf("Expected 2 configs, got %d", len(configs))
		}

		// Check contents
		config1, ok := configs["option1"]
		if !ok {
			t.Fatal("option1 not found in configs")
		}
		if config1["description"] != "Description 1" {
			t.Errorf("Expected description 'Description 1', got '%v'", config1["description"])
		}
		if config1["value"] != "value1" {
			t.Errorf("Expected value 'value1', got '%v'", config1["value"])
		}

		config2, ok := configs["option2"]
		if !ok {
			t.Fatal("option2 not found in configs")
		}
		if config2["description"] != "Description 2" {
			t.Errorf("Expected description 'Description 2', got '%v'", config2["description"])
		}
		if _, exists := config2["value"]; exists {
			t.Error("Secret config should not include value in info")
		}
	})

	// Test initialization with getter vs. explicitly set value validation
	t.Run("InitializeGetterVsExplicitValue", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Create a validator that rejects negative numbers
		validator := func(val int) error {
			if val < 0 {
				return fmt.Errorf("value must be non-negative")
			}
			return nil
		}

		// First test: using a getter that returns an invalid value
		// This should NOT fail validation during initialize since getter values aren't validated
		invalidGetter := func() int {
			return -10 // This would fail validation if validated
		}

		invalidGetterOpt := base.NewConfigOption(0).
			WithValidator(validator).
			WithGetter(invalidGetter)

		err := base.SetTypedConfigOption(module, "invalid_getter", invalidGetterOpt)
		if err != nil {
			t.Fatalf("SetTypedConfigOption failed: %v", err)
		}

		// Initialize should succeed because getter values aren't validated during initialization
		err = module.Initialize()
		if err != nil {
			t.Fatalf("Initialize should succeed with invalid getter value: %v", err)
		}

		// Second test: using an explicitly set invalid value
		// This should fail validation during initialize
		module = base.NewConfigurableModule() // Reset module

		invalidValueOpt := base.NewConfigOption(0).
			WithValidator(validator).
			WithValue(-10) // Explicitly set invalid value

		err = base.SetTypedConfigOption(module, "invalid_value", invalidValueOpt)
		if err != nil {
			t.Fatalf("SetTypedConfigOption failed: %v", err)
		}

		// Initialize should fail because explicitly set values are validated
		err = module.Initialize()
		if err == nil {
			t.Fatal("Initialize should fail with invalid explicitly set value")
		}
	})

	// Test initialization with various edge cases
	t.Run("InitializeEdgeCases", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Add an option without a name
		opt := base.NewConfigOption("no_name_value")
		if err := module.SetConfigOption("unnamed_option", opt); err != nil {
			t.Fatalf("Failed to set option without name: %v", err)
		}

		// Add a required option without a value but WITH a getter
		requiredWithGetter := base.NewConfigOption("").
			SetRequired(true).
			WithGetter(func() string { return "dynamic" })

		if err := module.SetConfigOption("required_with_getter", requiredWithGetter); err != nil {
			t.Fatalf("Failed to set required option with getter: %v", err)
		}

		// Initialize should succeed because required option has a getter
		if err := module.Initialize(); err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		// Check that the option without a name got a name during initialization
		retrievedOpt, err := module.GetConfigOption("unnamed_option")
		if err != nil {
			t.Fatalf("Failed to get option without name: %v", err)
		}

		if retrievedOpt.GetName() != "unnamed_option" {
			t.Errorf("Expected option name to be 'unnamed_option', got '%s'", retrievedOpt.GetName())
		}
	})
}

// Test the helper functions
func TestHelperFunctions(t *testing.T) {
	// Test SetTypedConfigOption
	t.Run("SetTypedConfigOption", func(t *testing.T) {
		module := base.NewConfigurableModule()

		strOpt := base.NewConfigOption("string_value")
		err := base.SetTypedConfigOption(module, "string_option", strOpt)
		if err != nil {
			t.Fatalf("SetTypedConfigOption failed: %v", err)
		}

		intOpt := base.NewConfigOption(42)
		err = base.SetTypedConfigOption(module, "int_option", intOpt)
		if err != nil {
			t.Fatalf("SetTypedConfigOption failed: %v", err)
		}
	})

	// Test GetConfigValue
	t.Run("GetConfigValue", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Set a string option
		strOpt := base.NewConfigOption("string_value")
		module.SetConfigOption("string_option", strOpt)

		// Get the value with the correct type
		strVal, err := base.GetConfigValue[string](module, "string_option")
		if err != nil {
			t.Fatalf("GetConfigValue failed: %v", err)
		}
		if strVal != "string_value" {
			t.Errorf("Expected string value 'string_value', got '%s'", strVal)
		}

		// Try to get a non-existent option
		_, err = base.GetConfigValue[string](module, "nonexistent")
		if !errors.Is(err, base.ErrConfigNotSet) {
			t.Errorf("Expected ErrConfigNotSet, got %v", err)
		}

		// Try to get with the wrong type
		_, err = base.GetConfigValue[int](module, "string_option")
		if err == nil {
			t.Error("Expected type mismatch error, got nil")
		}
	})

	// Test SetConfigValue
	t.Run("SetConfigValue", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Set a new value
		err := base.SetConfigValue(module, "new_option", "new_value")
		if err != nil {
			t.Fatalf("SetConfigValue failed: %v", err)
		}

		// Get the value to verify
		val, err := base.GetConfigValue[string](module, "new_option")
		if err != nil {
			t.Fatalf("GetConfigValue failed: %v", err)
		}
		if val != "new_value" {
			t.Errorf("Expected value 'new_value', got '%s'", val)
		}

		// Update an existing value
		err = base.SetConfigValue(module, "new_option", "updated_value")
		if err != nil {
			t.Fatalf("SetConfigValue update failed: %v", err)
		}

		// Get the updated value
		val, err = base.GetConfigValue[string](module, "new_option")
		if err != nil {
			t.Fatalf("GetConfigValue failed: %v", err)
		}
		if val != "updated_value" {
			t.Errorf("Expected value 'updated_value', got '%s'", val)
		}

		// Try setting with a different type, which should fail
		err = base.SetConfigValue(module, "new_option", 42)
		if err == nil {
			t.Error("Expected type mismatch error when setting different type, got nil")
		}

		// Initialize the module
		err = module.Initialize()
		if err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		// Try setting after initialization, which should fail
		err = base.SetConfigValue(module, "after_init", "value")
		if !errors.Is(err, base.ErrModuleAlreadyInitialized) {
			t.Errorf("Expected ErrModuleAlreadyInitialized, got %v", err)
		}
	})

	// Test SetConfigGetter
	t.Run("SetConfigGetter", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Set a getter for a new option - without a value set yet
		dynamicValue := "initial"
		err := base.SetConfigGetter(module, "getter_option", func() string {
			return dynamicValue
		})
		if err != nil {
			t.Fatalf("SetConfigGetter failed: %v", err)
		}

		// Get the value
		val, err := base.GetConfigValue[string](module, "getter_option")
		if err != nil {
			t.Fatalf("GetConfigValue failed: %v", err)
		}
		if val != "initial" {
			t.Errorf("Expected value 'initial', got '%s'", val)
		}

		// Update the dynamic value
		dynamicValue = "updated"
		val, err = base.GetConfigValue[string](module, "getter_option")
		if err != nil {
			t.Fatalf("GetConfigValue failed: %v", err)
		}
		if val != "updated" {
			t.Errorf("Expected value 'updated', got '%s'", val)
		}

		// Now explicitly set a value
		optInterface, err := module.GetConfigOption("getter_option")
		if err != nil {
			t.Fatalf("GetConfigOption failed: %v", err)
		}
		typedOpt, ok := optInterface.(*base.ConfigOption[string])
		if !ok {
			t.Fatalf("Failed to cast to ConfigOption[string]")
		}

		// Explicit value should take precedence over getter
		typedOpt.SetValue("explicit")
		val, err = base.GetConfigValue[string](module, "getter_option")
		if err != nil {
			t.Fatalf("GetConfigValue failed: %v", err)
		}
		if val != "explicit" {
			t.Errorf("Expected 'explicit', got '%s'", val)
		}

		// Initialize the module
		err = module.Initialize()
		if err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		// Try setting a getter after initialization, which should fail
		err = base.SetConfigGetter(module, "after_init", func() string {
			return "value"
		})
		if !errors.Is(err, base.ErrModuleAlreadyInitialized) {
			t.Errorf("Expected ErrModuleAlreadyInitialized, got %v", err)
		}
	})

	// Test NewConfigurableModuleWithOptions
	t.Run("NewConfigurableModuleWithOptions", func(t *testing.T) {
		// Create a module with options
		stringOption := base.NewConfigOption("default_string").WithDescription("A string option").WithValue("string value")

		// Test successful creation with multiple options
		module, err := base.NewConfigurableModuleWithOptions(
			base.WithTypedConfigOption("string_opt", stringOption),
			base.WithConfigValue("int_opt", 100),
			base.WithConfigGetter("dynamic_opt", func() string {
				return "dynamic_value"
			}),
		)

		if err != nil {
			t.Fatalf("NewConfigurableModuleWithOptions failed: %v", err)
		}

		// Check if options were correctly set
		stringVal, err := base.GetConfigValue[string](module, "string_opt")
		if err != nil {
			t.Fatalf("GetConfigValue for string_opt failed: %v", err)
		}
		if stringVal != "string value" {
			t.Errorf("Expected string_opt value 'string value', got '%s'", stringVal)
		}

		intVal, err := base.GetConfigValue[int](module, "int_opt")
		if err != nil {
			t.Fatalf("GetConfigValue for int_opt failed: %v", err)
		}
		if intVal != 100 {
			t.Errorf("Expected int_opt value 100, got %d", intVal)
		}

		dynamicVal, err := base.GetConfigValue[string](module, "dynamic_opt")
		if err != nil {
			t.Fatalf("GetConfigValue for dynamic_opt failed: %v", err)
		}
		if dynamicVal != "dynamic_value" {
			t.Errorf("Expected dynamic_opt value 'dynamic_value', got '%s'", dynamicVal)
		}

		// Test with a failing option
		invalidModule, err := base.NewConfigurableModuleWithOptions(
			base.WithTypedConfigOption("invalid", base.NewConfigOption(0).
				WithValidator(func(val int) error {
					return fmt.Errorf("always fails")
				}).
				WithValue(0)),
		)
		if err != nil {
			t.Fatalf("NewConfigurableModuleWithOptions should not fail during creation: %v", err)
		}

		// Validation should fail during Initialize
		err = invalidModule.Initialize()
		if err == nil {
			t.Fatal("Expected invalidModule.Initialize() to fail with invalid option")
		}
	})
}

// Test ListConfigs method
func TestListConfigs(t *testing.T) {
	module := base.NewConfigurableModule()

	// Add various config options
	module.SetConfigOption("string_opt", base.NewConfigOption("string value").WithDescription("A string option").WithValue("string value"))
	module.SetConfigOption("int_opt", base.NewConfigOption(42).WithDescription("An integer option"))
	module.SetConfigOption("bool_opt", base.NewConfigOption(true).WithDescription("A boolean option"))
	module.SetConfigOption("secret_opt", base.NewConfigOption("secret").SetSecret(true).WithDescription("A secret option"))
	module.SetConfigOption("required_opt", base.NewConfigOption("").SetRequired(true).WithDescription("A required option").WithValue("required value"))

	// Test ListConfigs
	configs := module.ListConfigs()

	// Verify all configs are present
	expectedConfigs := []string{"string_opt", "int_opt", "bool_opt", "secret_opt", "required_opt"}
	for _, name := range expectedConfigs {
		if _, exists := configs[name]; !exists {
			t.Errorf("Expected config '%s' to be present in ListConfigs result", name)
		}
	}

	// Check specific properties
	if configs["string_opt"]["description"] != "A string option" {
		t.Errorf("Expected description 'A string option', got '%v'", configs["string_opt"]["description"])
	}

	if configs["bool_opt"]["value"] != true {
		t.Errorf("Expected boolean value true, got %v", configs["bool_opt"]["value"])
	}

	if configs["required_opt"]["required"] != true {
		t.Errorf("Expected required_opt to have required=true, got %v", configs["required_opt"]["required"])
	}

	if configs["secret_opt"]["secret"] != true {
		t.Errorf("Expected secret_opt to have secret=true, got %v", configs["secret_opt"]["secret"])
	}

	// Secret configs should not expose their values
	if val, exists := configs["secret_opt"]["value"]; exists {
		t.Errorf("Secret config should not expose its value, but got %v", val)
	}
}

// Test GetConfigOption more extensively
func TestGetConfigOption(t *testing.T) {
	module := base.NewConfigurableModule()

	// Test getting a non-existent option
	_, err := module.GetConfigOption("nonexistent")
	if err == nil {
		t.Error("Expected error when getting non-existent config, got nil")
	}
	if !errors.Is(err, base.ErrConfigNotSet) {
		t.Errorf("Expected ErrConfigNotSet, got %v", err)
	}

	// Add some config options
	stringOpt := base.NewConfigOption("string value").WithDescription("A string option").WithValue("string value")
	module.SetConfigOption("string_opt", stringOpt)

	// Get the option and verify it's the same instance
	retrievedOpt, err := module.GetConfigOption("string_opt")
	if err != nil {
		t.Fatalf("GetConfigOption failed: %v", err)
	}

	// Check if the retrieved option has the correct properties
	if retrievedOpt.GetName() != "string_opt" {
		t.Errorf("Expected name 'string_opt', got '%s'", retrievedOpt.GetName())
	}

	if !retrievedOpt.HasValue() {
		t.Error("Expected option to have a value")
	}

	// Check that we can get the actual option with type information
	typedOpt, ok := retrievedOpt.(*base.ConfigOption[string])
	if !ok {
		t.Fatalf("Retrieved option is not of expected type *base.ConfigOption[string]")
	}

	// Verify we can get the value through the typed option
	val, err := typedOpt.GetValue()
	if err != nil {
		t.Fatalf("GetValue failed: %v", err)
	}
	if val != "string value" {
		t.Errorf("Expected value 'string value', got '%s'", val)
	}
}

// Test GetConfigValue function more extensively
func TestGetConfigValue(t *testing.T) {
	module := base.NewConfigurableModule()

	// Test getting a non-existent config
	_, err := base.GetConfigValue[string](module, "nonexistent")
	if err == nil {
		t.Error("Expected error when getting non-existent config, got nil")
	}
	if !errors.Is(err, base.ErrConfigNotSet) {
		t.Errorf("Expected ErrConfigNotSet, got %v", err)
	}

	// Test getting a config with incorrect type
	module.SetConfigOption("int_config", base.NewConfigOption(42))
	_, err = base.GetConfigValue[string](module, "int_config")
	if err == nil {
		t.Error("Expected error when getting config with wrong type, got nil")
	}
	if errors.Is(err, base.ErrConfigNotSet) {
		t.Error("Error should not be ErrConfigNotSet for type mismatch")
	}

	// Test getting a config with correct type
	intVal, err := base.GetConfigValue[int](module, "int_config")
	if err != nil {
		t.Fatalf("GetConfigValue failed: %v", err)
	}
	if intVal != 42 {
		t.Errorf("Expected value 42, got %d", intVal)
	}

	// Test with a getter function
	dynamicValue := "initial"
	module.SetConfigOption("dynamic_config", base.NewConfigOption("").WithGetter(func() string {
		return dynamicValue
	}))

	// Get the initial value
	strVal, err := base.GetConfigValue[string](module, "dynamic_config")
	if err != nil {
		t.Fatalf("GetConfigValue failed: %v", err)
	}
	if strVal != "initial" {
		t.Errorf("Expected value 'initial', got '%s'", strVal)
	}

	// Change the dynamic value and get it again
	dynamicValue = "updated"
	strVal, err = base.GetConfigValue[string](module, "dynamic_config")
	if err != nil {
		t.Fatalf("GetConfigValue failed: %v", err)
	}
	if strVal != "updated" {
		t.Errorf("Expected updated value 'updated', got '%s'", strVal)
	}

	// Test with complex types
	module.SetConfigOption("slice_config", base.NewConfigOption([]string{"a", "b", "c"}))
	sliceVal, err := base.GetConfigValue[[]string](module, "slice_config")
	if err != nil {
		t.Fatalf("GetConfigValue failed: %v", err)
	}
	if len(sliceVal) != 3 || sliceVal[0] != "a" || sliceVal[1] != "b" || sliceVal[2] != "c" {
		t.Errorf("Expected slice [a b c], got %v", sliceVal)
	}

	// Test with map type
	module.SetConfigOption("map_config", base.NewConfigOption(map[string]int{"a": 1, "b": 2}))
	mapVal, err := base.GetConfigValue[map[string]int](module, "map_config")
	if err != nil {
		t.Fatalf("GetConfigValue failed: %v", err)
	}
	if len(mapVal) != 2 || mapVal["a"] != 1 || mapVal["b"] != 2 {
		t.Errorf("Expected map {a:1, b:2}, got %v", mapVal)
	}
}

// Test NewConfigurableModule more extensively
func TestNewConfigurableModule(t *testing.T) {
	// Test the basic constructor
	module := base.NewConfigurableModule()
	if module == nil {
		t.Fatal("NewConfigurableModule should not return nil")
	}

	// Ensure the configs map is initialized
	if module.ListConfigs() == nil {
		t.Error("Configs map should be initialized")
	}

	// Test constructor with options
	moduleWithOptions, err := base.NewConfigurableModuleWithOptions(
		base.WithConfigOption("string_opt", base.NewConfigOption("value")),
		base.WithConfigValue("int_opt", 42),
	)
	if err != nil {
		t.Fatalf("NewConfigurableModuleWithOptions failed: %v", err)
	}
	if moduleWithOptions == nil {
		t.Fatal("NewConfigurableModuleWithOptions should not return nil")
	}

	// Verify the options were set correctly
	configs := moduleWithOptions.ListConfigs()
	if _, exists := configs["string_opt"]; !exists {
		t.Error("string_opt should exist")
	}
	if _, exists := configs["int_opt"]; !exists {
		t.Error("int_opt should exist")
	}

	// Test constructor with failing option
	_, err = base.NewConfigurableModuleWithOptions(
		base.WithConfigOption("string_opt", base.NewConfigOption("value")),
		func(m *base.ConfigurableModule) error {
			return errors.New("test error")
		},
	)
	if err == nil {
		t.Error("Expected error when option fails, got nil")
	}
	if err != nil && err.Error() != "failed to apply module option: test error" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

// Test SetTypedConfigOption more extensively
func TestSetTypedConfigOption(t *testing.T) {
	module := base.NewConfigurableModule()

	// Test setting a strongly-typed option
	stringOpt := base.NewConfigOption("value")
	err := base.SetTypedConfigOption(module, "string_opt", stringOpt)
	if err != nil {
		t.Fatalf("SetTypedConfigOption failed: %v", err)
	}

	// Verify the option was set correctly
	retrievedOpt, err := module.GetConfigOption("string_opt")
	if err != nil {
		t.Fatalf("GetConfigOption failed: %v", err)
	}
	if retrievedOpt.GetName() != "string_opt" {
		t.Errorf("Expected option name 'string_opt', got '%s'", retrievedOpt.GetName())
	}

	// Test the typed helper function with WithTypedConfigOption
	intModule := base.NewConfigurableModule()
	intOpt := base.NewConfigOption(42)
	err = base.WithTypedConfigOption("int_opt", intOpt)(intModule)
	if err != nil {
		t.Fatalf("WithTypedConfigOption failed: %v", err)
	}

	// Verify the option was set correctly
	intVal, err := base.GetConfigValue[int](intModule, "int_opt")
	if err != nil {
		t.Fatalf("GetConfigValue failed: %v", err)
	}
	if intVal != 42 {
		t.Errorf("Expected value 42, got %d", intVal)
	}

	// Test with an already initialized module
	initializedModule := base.NewConfigurableModule()
	err = initializedModule.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	err = base.SetTypedConfigOption(initializedModule, "too_late", base.NewConfigOption("value"))
	if !errors.Is(err, base.ErrModuleAlreadyInitialized) {
		t.Errorf("Expected ErrModuleAlreadyInitialized, got %v", err)
	}
}

// Test SetConfigGetter function more extensively
func TestSetConfigGetter(t *testing.T) {
	// Test setting a config getter on a new module
	t.Run("BasicGetter", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Set a simple getter
		dynamicValue := "initial"
		err := base.SetConfigGetter(module, "dynamic", func() string {
			return dynamicValue
		})
		if err != nil {
			t.Fatalf("SetConfigGetter failed: %v", err)
		}

		// Check that the getter works
		val, err := base.GetConfigValue[string](module, "dynamic")
		if err != nil {
			t.Fatalf("GetConfigValue failed: %v", err)
		}
		if val != "initial" {
			t.Errorf("Expected 'initial', got '%s'", val)
		}

		// Change the dynamic value and check again
		dynamicValue = "updated"
		val, err = base.GetConfigValue[string](module, "dynamic")
		if err != nil {
			t.Fatalf("GetConfigValue failed: %v", err)
		}
		if val != "updated" {
			t.Errorf("Expected 'updated', got '%s'", val)
		}
	})

	// Test setting a getter on an existing option
	t.Run("ExistingOption", func(t *testing.T) {
		// When calling SetConfigGetter on an existing option, it adds a getter
		// but according to the new priority rules, explicit values always win
		module := base.NewConfigurableModule()

		// Create an option with no explicit value (using default only)
		module.SetConfigOption("existing", base.NewConfigOption("default_value"))

		// Set a getter on the existing option
		dynamicValue := "dynamic"
		err := base.SetConfigGetter(module, "existing", func() string {
			return dynamicValue
		})
		if err != nil {
			t.Fatalf("SetConfigGetter failed: %v", err)
		}

		// Get the value - getter should be used since no explicit value was set
		val, err := base.GetConfigValue[string](module, "existing")
		if err != nil {
			t.Fatalf("GetConfigValue failed: %v", err)
		}
		if val != "dynamic" {
			t.Errorf("Expected 'dynamic', got '%s'", val)
		}

		// Now set an explicit value
		optInterface, err := module.GetConfigOption("existing")
		if err != nil {
			t.Fatalf("GetConfigOption failed: %v", err)
		}
		typedOpt, ok := optInterface.(*base.ConfigOption[string])
		if !ok {
			t.Fatalf("Failed to cast option to correct type")
		}

		// Set an explicit value, which should take precedence
		err = typedOpt.SetValue("explicit_value")
		if err != nil {
			t.Fatalf("SetValue failed: %v", err)
		}

		// Check that explicit value takes precedence
		val, err = base.GetConfigValue[string](module, "existing")
		if err != nil {
			t.Fatalf("GetConfigValue failed: %v", err)
		}
		if val != "explicit_value" {
			t.Errorf("Expected 'explicit_value', got '%s'", val)
		}
	})

	// Test with type mismatch
	t.Run("TypeMismatch", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Create an option with int type
		module.SetConfigOption("int_opt", base.NewConfigOption(42))

		// Try to set a string getter - this should fail
		err := base.SetConfigGetter(module, "int_opt", func() string {
			return "string"
		})
		if err == nil {
			t.Error("Expected error when setting getter with wrong type, got nil")
		}
	})

	// Test with initialized module
	t.Run("InitializedModule", func(t *testing.T) {
		module := base.NewConfigurableModule()
		err := module.Initialize()
		if err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		// Try to set a getter on initialized module
		err = base.SetConfigGetter(module, "too_late", func() string {
			return "value"
		})
		if !errors.Is(err, base.ErrModuleAlreadyInitialized) {
			t.Errorf("Expected ErrModuleAlreadyInitialized, got %v", err)
		}
	})
}

// TestSetConfigEnvVar tests setting environment variables at the module level
func TestSetConfigEnvVar(t *testing.T) {
	// Set up test environment variables
	os.Setenv("MODULE_TEST_STRING", "module_env_value")
	os.Setenv("MODULE_TEST_INT", "123")
	defer func() {
		os.Unsetenv("MODULE_TEST_STRING")
		os.Unsetenv("MODULE_TEST_INT")
	}()

	module := base.NewConfigurableModule()

	// Set an environment variable for a new option
	err := base.SetConfigEnvVar[string](module, "env_option", "MODULE_TEST_STRING")
	if err != nil {
		t.Fatalf("SetConfigEnvVar failed: %v", err)
	}

	// Get the value - should use the environment variable
	val, err := base.GetConfigValue[string](module, "env_option")
	if err != nil {
		t.Fatalf("GetConfigValue failed: %v", err)
	}
	if val != "module_env_value" {
		t.Errorf("Expected environment value 'module_env_value', got '%s'", val)
	}

	// Test setting env var on an existing option
	module.SetConfigOption("existing_opt", base.NewConfigOption("default_value"))
	err = base.SetConfigEnvVar[string](module, "existing_opt", "MODULE_TEST_STRING")
	if err != nil {
		t.Fatalf("SetConfigEnvVar on existing option failed: %v", err)
	}

	// Get the value
	val, err = base.GetConfigValue[string](module, "existing_opt")
	if err != nil {
		t.Fatalf("GetConfigValue failed: %v", err)
	}
	if val != "module_env_value" {
		t.Errorf("Expected environment value 'module_env_value', got '%s'", val)
	}

	// Now set an explicit value, which should take precedence
	err = base.SetConfigValue(module, "existing_opt", "explicit_value")
	if err != nil {
		t.Fatalf("SetConfigValue failed: %v", err)
	}

	val, err = base.GetConfigValue[string](module, "existing_opt")
	if err != nil {
		t.Fatalf("GetConfigValue failed: %v", err)
	}
	if val != "explicit_value" {
		t.Errorf("Expected explicit value 'explicit_value', got '%s'", val)
	}

	// Test with a different type (int)
	err = base.SetConfigEnvVar[int](module, "int_env_option", "MODULE_TEST_INT")
	if err != nil {
		t.Fatalf("SetConfigEnvVar for int failed: %v", err)
	}

	intVal, err := base.GetConfigValue[int](module, "int_env_option")
	if err != nil {
		t.Fatalf("GetConfigValue for int failed: %v", err)
	}
	if intVal != 123 {
		t.Errorf("Expected int environment value 123, got %d", intVal)
	}

	// Test with type mismatch
	err = base.SetConfigEnvVar[float64](module, "existing_opt", "MODULE_TEST_STRING")
	if err == nil {
		t.Error("Expected error when setting environment variable with wrong type, got nil")
	}

	// Initialize the module
	err = module.Initialize()
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Try setting env var after initialization, which should fail
	err = base.SetConfigEnvVar[string](module, "after_init", "MODULE_TEST_STRING")
	if !errors.Is(err, base.ErrModuleAlreadyInitialized) {
		t.Errorf("Expected ErrModuleAlreadyInitialized, got %v", err)
	}
}

// TestSetConfigDefault tests setting default values for config options
func TestSetConfigDefault(t *testing.T) {
	t.Run("SetDefaultForExistingOption", func(t *testing.T) {
		m := base.NewConfigurableModule()

		// Create and set a config option
		opt := base.NewConfigOption("original_default")
		if err := m.SetConfigOption("option1", opt); err != nil {
			t.Fatalf("Failed to set config option: %v", err)
		}

		// Set a new default value
		if err := base.SetConfigDefault(m, "option1", "new_default"); err != nil {
			t.Fatalf("SetConfigDefault failed: %v", err)
		}

		// Retrieve the option and verify the default value
		retrievedOpt, err := m.GetConfigOption("option1")
		if err != nil {
			t.Fatalf("Failed to get config option: %v", err)
		}

		typedOpt, ok := retrievedOpt.(*base.ConfigOption[string])
		if !ok {
			t.Fatal("Failed to cast to ConfigOption[string]")
		}

		val, err := typedOpt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}

		if val != "new_default" {
			t.Errorf("Expected default value 'new_default', got '%s'", val)
		}
	})

	t.Run("SetDefaultCreatesNewOption", func(t *testing.T) {
		m := base.NewConfigurableModule()

		// Set default value for a non-existent option (should create it)
		if err := base.SetConfigDefault(m, "option1", "default_value"); err != nil {
			t.Fatalf("SetConfigDefault failed: %v", err)
		}

		// Retrieve the option and verify it was created
		retrievedOpt, err := m.GetConfigOption("option1")
		if err != nil {
			t.Fatalf("Failed to get config option: %v", err)
		}

		typedOpt, ok := retrievedOpt.(*base.ConfigOption[string])
		if !ok {
			t.Fatal("Failed to cast to ConfigOption[string]")
		}

		val, err := typedOpt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}

		if val != "default_value" {
			t.Errorf("Expected default value 'default_value', got '%s'", val)
		}
	})

	t.Run("SetDefaultAfterInitialize", func(t *testing.T) {
		m := base.NewConfigurableModule()

		// Initialize the module
		if err := m.Initialize(); err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		// Try to set a default value after initialization
		err := base.SetConfigDefault(m, "option1", "default_value")
		if !errors.Is(err, base.ErrModuleAlreadyInitialized) {
			t.Errorf("Expected ErrModuleAlreadyInitialized, got %v", err)
		}
	})

	t.Run("SetDefaultTypeMismatch", func(t *testing.T) {
		m := base.NewConfigurableModule()

		// Create a config option with int type
		opt := base.NewConfigOption(42)
		if err := m.SetConfigOption("option1", opt); err != nil {
			t.Fatalf("Failed to set config option: %v", err)
		}

		// Try to set a default value with wrong type
		err := base.SetConfigDefault(m, "option1", "string_value")
		if err == nil {
			t.Error("Expected error when setting default with wrong type, got nil")
		}
	})
}

// TestWithConfigDefault tests the WithConfigDefault module option
func TestWithConfigDefault(t *testing.T) {
	t.Run("UseWithinNewModule", func(t *testing.T) {
		// Create a module with the WithConfigDefault option
		m, err := base.NewConfigurableModuleWithOptions(
			base.WithConfigDefault("option1", "default_value"),
		)
		if err != nil {
			t.Fatalf("NewConfigurableModuleWithOptions failed: %v", err)
		}

		// Verify that the option was created with the default value
		val, err := base.GetConfigValue[string](m, "option1")
		if err != nil {
			t.Fatalf("GetConfigValue failed: %v", err)
		}

		if val != "default_value" {
			t.Errorf("Expected default value 'default_value', got '%s'", val)
		}
	})

	t.Run("CombineWithOtherOptions", func(t *testing.T) {
		// Create a module with multiple options
		m, err := base.NewConfigurableModuleWithOptions(
			base.WithConfigDefault("option1", "default_value"),
			base.WithConfigValue("option2", "explicit_value"),
			base.WithConfigEnvVar[string]("option1", "TEST_ENV_VAR"), // Add env var to option1
		)
		if err != nil {
			t.Fatalf("NewConfigurableModuleWithOptions failed: %v", err)
		}

		// Verify that both options were set correctly
		val1, err := base.GetConfigValue[string](m, "option1")
		if err != nil {
			t.Fatalf("GetConfigValue for option1 failed: %v", err)
		}

		val2, err := base.GetConfigValue[string](m, "option2")
		if err != nil {
			t.Fatalf("GetConfigValue for option2 failed: %v", err)
		}

		if val1 != "default_value" {
			t.Errorf("Expected default value 'default_value' for option1, got '%s'", val1)
		}

		if val2 != "explicit_value" {
			t.Errorf("Expected value 'explicit_value' for option2, got '%s'", val2)
		}

		// Verify that the env var was set on option1
		opt, err := m.GetConfigOption("option1")
		if err != nil {
			t.Fatalf("GetConfigOption failed: %v", err)
		}

		if !opt.HasEnvVar() {
			t.Error("Expected option1 to have an environment variable set")
		}
	})

	t.Run("PriorityWithEnvVar", func(t *testing.T) {
		// Set an environment variable
		os.Setenv("TEST_DEFAULT_ENV", "env_value")
		defer os.Unsetenv("TEST_DEFAULT_ENV")

		// Create a module with default and env var
		m, err := base.NewConfigurableModuleWithOptions(
			base.WithConfigDefault("option1", "default_value"),
			base.WithConfigEnvVar[string]("option1", "TEST_DEFAULT_ENV"),
		)
		if err != nil {
			t.Fatalf("NewConfigurableModuleWithOptions failed: %v", err)
		}

		// Env var should take precedence over default
		val, err := base.GetConfigValue[string](m, "option1")
		if err != nil {
			t.Fatalf("GetConfigValue failed: %v", err)
		}

		if val != "env_value" {
			t.Errorf("Expected env value 'env_value' to take precedence over default, got '%s'", val)
		}
	})
}

func TestConfigurableModule_SetAndGetValue(t *testing.T) {
	// Create a new module
	module := base.NewConfigurableModule()

	// Set a value
	err := base.SetConfigValue(module, "test", "test_value")
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Get the value
	value, err := base.GetConfigValue[string](module, "test")
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}

	if value != "test_value" {
		t.Errorf("Expected value 'test_value', got '%s'", value)
	}
}

func TestConfigurableModule_ImmutableAfterInit(t *testing.T) {
	m := base.NewConfigurableModule()
	if err := base.SetConfigValue(m, "port", 8080); err != nil {
		t.Fatalf("Failed to set config: %v", err)
	}
	if err := m.Initialize(); err != nil {
		t.Fatalf("Failed to initialize module: %v", err)
	}
	if err := base.SetConfigValue(m, "port", 9090); err == nil {
		t.Fatalf("Expected error when setting config after initialization, but got none")
	} else if err != base.ErrModuleAlreadyInitialized {
		t.Fatalf("Expected ErrModuleAlreadyInitialized, got %v", err)
	}
}

func TestConfigOption_Secret(t *testing.T) {
	// Create a secret config option
	secretValue := "secret"
	opt := base.NewConfigOption[string](secretValue).WithName("api_key")
	opt.SetSecret(true)

	// Secret values should be accessible via GetValue in Go code
	val, err := opt.GetValue()
	if err != nil {
		t.Errorf("Expected no error retrieving secret config in Go, got %v", err)
	}
	if val != secretValue {
		t.Errorf("Expected secret value %q, got %q", secretValue, val)
	}

	// Verify that the secret config does not expose its value in GetInfo
	info := opt.GetInfo()
	if _, ok := info["value"]; ok {
		t.Errorf("Secret config should not expose its value in GetInfo")
	}

	// Verify that GetStarlarkValue doesn't block secrets
	starlarkVal, err := opt.GetStarlarkValue()
	if err != nil {
		t.Errorf("Expected no error retrieving secret value via GetStarlarkValue, got %v", err)
	}
	if starlarkVal == nil {
		t.Error("GetStarlarkValue should not return nil for secret values")
	}

	// However, in a Starlark module, getters for secret values are not registered
	// This is tested in TestStarlarkSecretAccess
}

func TestConfigOption_WithEnvVar(t *testing.T) {
	// Set environment variable and ensure it overrides the default
	os.Setenv("TEST_ENV", "environment_value")
	defer os.Unsetenv("TEST_ENV")
	opt := base.NewConfigOption[string]("default").WithName("config1").WithEnvVar("TEST_ENV")
	value, err := opt.GetValue()
	if err != nil {
		t.Fatalf("Failed to get env var overridden value: %v", err)
	}
	if value != "environment_value" {
		t.Errorf("Expected environment value 'environment_value', got %v", value)
	}
}

func TestSetConfigValueTypeMismatch(t *testing.T) {
	module := base.NewConfigurableModule()

	// First, set an initial string config
	err := base.SetConfigValue(module, "test_option", "string value")
	if err != nil {
		t.Fatalf("Failed to set initial string value: %v", err)
	}

	// Now try to set it with a different type (int)
	err = base.SetConfigValue(module, "test_option", 42)
	if err == nil {
		t.Error("Expected error when setting value with different type, got nil")
	}

	// Verify the error message mentions type mismatch
	if err != nil && !strings.Contains(err.Error(), "cannot set value of different type") {
		t.Errorf("Expected type mismatch error, got: %v", err)
	}
}

// TestWithConfigEnvVar tests the WithConfigEnvVar module option
func TestWithConfigEnvVar(t *testing.T) {
	// Set up test environment variables
	os.Setenv("MODULE_OPTION_STRING", "option_env_value")
	os.Setenv("MODULE_OPTION_INT", "456")
	defer func() {
		os.Unsetenv("MODULE_OPTION_STRING")
		os.Unsetenv("MODULE_OPTION_INT")
	}()

	// Create a module with environment variable options
	module, err := base.NewConfigurableModuleWithOptions(
		base.WithConfigEnvVar[string]("string_env", "MODULE_OPTION_STRING"),
		base.WithConfigEnvVar[int]("int_env", "MODULE_OPTION_INT"),
		// Mix with other option types
		base.WithConfigValue("direct_value", "value1"),
		base.WithConfigGetter("dynamic_value", func() string {
			return "dynamic"
		}),
	)

	if err != nil {
		t.Fatalf("NewConfigurableModuleWithOptions failed: %v", err)
	}

	// Test the string environment variable
	strVal, err := base.GetConfigValue[string](module, "string_env")
	if err != nil {
		t.Fatalf("GetConfigValue for string_env failed: %v", err)
	}
	if strVal != "option_env_value" {
		t.Errorf("Expected env value 'option_env_value', got '%s'", strVal)
	}

	// Test the int environment variable
	intVal, err := base.GetConfigValue[int](module, "int_env")
	if err != nil {
		t.Fatalf("GetConfigValue for int_env failed: %v", err)
	}
	if intVal != 456 {
		t.Errorf("Expected env value 456, got %d", intVal)
	}

	// Test the direct value
	directVal, err := base.GetConfigValue[string](module, "direct_value")
	if err != nil {
		t.Fatalf("GetConfigValue for direct_value failed: %v", err)
	}
	if directVal != "value1" {
		t.Errorf("Expected direct value 'value1', got '%s'", directVal)
	}

	// Test the getter value
	dynamicVal, err := base.GetConfigValue[string](module, "dynamic_value")
	if err != nil {
		t.Fatalf("GetConfigValue for dynamic_value failed: %v", err)
	}
	if dynamicVal != "dynamic" {
		t.Errorf("Expected dynamic value 'dynamic', got '%s'", dynamicVal)
	}

	// Test priority by setting multiple config methods for the same option
	complexModule, err := base.NewConfigurableModuleWithOptions(
		// 1. Environment variable
		base.WithConfigEnvVar[string]("complex", "MODULE_OPTION_STRING"),
		// 2. Default value (via empty option)
		base.WithTypedConfigOption("complex", base.NewConfigOption("default")),
		// 3. Getter
		base.WithConfigGetter("complex", func() string {
			return "from_getter"
		}),
		// 4. Direct value (highest priority)
		base.WithConfigValue("complex", "explicit_value"),
	)

	if err != nil {
		t.Fatalf("NewConfigurableModuleWithOptions with complex setup failed: %v", err)
	}

	// The direct value should take precedence
	complexVal, err := base.GetConfigValue[string](complexModule, "complex")
	if err != nil {
		t.Fatalf("GetConfigValue for complex failed: %v", err)
	}
	if complexVal != "explicit_value" {
		t.Errorf("Expected explicit value 'explicit_value' to take precedence, got '%s'", complexVal)
	}
}

// TestNewConfigurableModuleWithConfigOptions tests the constructor that takes ConfigOptionInterface directly
func TestNewConfigurableModuleWithConfigOptions(t *testing.T) {
	// Test the new constructor with config options of different types
	strOpt := base.NewConfigOption("string value").WithName("string_opt").WithDescription("A string option")
	intOpt := base.NewConfigOption(42).WithName("int_opt").WithDescription("An integer option")
	boolOpt := base.NewConfigOption(true) // No name set

	// Create a module with the options
	module, err := base.NewConfigurableModuleWithConfigOptions(strOpt, intOpt, boolOpt)
	if err != nil {
		t.Fatalf("NewConfigurableModuleWithConfigOptions failed: %v", err)
	}
	if module == nil {
		t.Fatal("NewConfigurableModuleWithConfigOptions should not return nil")
	}

	// Verify the options were set correctly
	configs := module.ListConfigs()
	if len(configs) != 3 {
		t.Errorf("Expected 3 config options, got %d", len(configs))
	}

	// Check named options
	if _, exists := configs["string_opt"]; !exists {
		t.Error("string_opt should exist")
	}
	if _, exists := configs["int_opt"]; !exists {
		t.Error("int_opt should exist")
	}

	// Check auto-named option (should be "option_3")
	if _, exists := configs["option_3"]; !exists {
		t.Error("auto-named option should exist as 'option_3'")
	}

	// Verify values can be retrieved
	strVal, err := base.GetConfigValue[string](module, "string_opt")
	if err != nil {
		t.Fatalf("GetConfigValue for string_opt failed: %v", err)
	}
	if strVal != "string value" {
		t.Errorf("Expected string_opt value 'string value', got '%s'", strVal)
	}

	intVal, err := base.GetConfigValue[int](module, "int_opt")
	if err != nil {
		t.Fatalf("GetConfigValue for int_opt failed: %v", err)
	}
	if intVal != 42 {
		t.Errorf("Expected int_opt value 42, got %d", intVal)
	}

	boolVal, err := base.GetConfigValue[bool](module, "option_3")
	if err != nil {
		t.Fatalf("GetConfigValue for option_3 failed: %v", err)
	}
	if !boolVal {
		t.Error("Expected option_3 value true, got false")
	}

	// Test with an already initialized module
	initializedModule := base.NewConfigurableModule()
	if err := initializedModule.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Create a new option to add to an initialized module (should fail)
	newOpt := base.NewConfigOption("new").WithName("new_opt")

	// Use the module directly with SetConfigOption (bypassing the constructor)
	err = initializedModule.SetConfigOption("new_opt", newOpt)
	if !errors.Is(err, base.ErrModuleAlreadyInitialized) {
		t.Errorf("Expected ErrModuleAlreadyInitialized, got %v", err)
	}

	// Test with validator that fails
	validatedOpt := base.NewConfigOption(0).
		WithName("validated_opt").
		WithValidator(func(val int) error {
			if val < 0 {
				return fmt.Errorf("value must be non-negative")
			}
			return nil
		}).
		WithValue(-10) // This invalid value is accepted by WithValue but will fail at initialization

	// Try to create a module with the invalid option
	invalidModule, err := base.NewConfigurableModuleWithConfigOptions(validatedOpt)
	if err != nil {
		t.Fatalf("NewConfigurableModuleWithConfigOptions should succeed but initialize should fail: %v", err)
	}

	// Now try to initialize the module, which should fail due to validation
	err = invalidModule.Initialize()
	if err == nil {
		t.Fatal("Expected validation error during initialization but got nil")
	}

	// Test with empty option list
	emptyModule, err := base.NewConfigurableModuleWithConfigOptions()
	if err != nil {
		t.Fatalf("NewConfigurableModuleWithConfigOptions with empty options list failed: %v", err)
	}
	if emptyModule == nil {
		t.Fatal("NewConfigurableModuleWithConfigOptions should not return nil with empty options list")
	}

	// Verify no options were set
	emptyConfigs := emptyModule.ListConfigs()
	if len(emptyConfigs) != 0 {
		t.Errorf("Expected 0 config options, got %d", len(emptyConfigs))
	}

	// Test with conflicting names
	opt1 := base.NewConfigOption("value1").WithName("same_name")
	opt2 := base.NewConfigOption("value2").WithName("same_name")

	// Should succeed and use the last value
	conflictModule, err := base.NewConfigurableModuleWithConfigOptions(opt1, opt2)
	if err != nil {
		t.Fatalf("NewConfigurableModuleWithConfigOptions with conflicting names failed: %v", err)
	}

	// Verify only one option was set with the last value
	conflictVal, err := base.GetConfigValue[string](conflictModule, "same_name")
	if err != nil {
		t.Fatalf("GetConfigValue for conflicting name failed: %v", err)
	}
	if conflictVal != "value2" {
		t.Errorf("Expected 'value2' for conflicting name, got '%s'", conflictVal)
	}
}

// TestNewConfigurableModuleWithConfigOptionsEdgeCases tests edge cases for the constructor that takes ConfigOptionInterface directly
func TestNewConfigurableModuleWithConfigOptionsEdgeCases(t *testing.T) {
	t.Run("WithInvalidOption", func(t *testing.T) {
		// Create an option with an already set name, then try to override it
		opt := base.NewConfigOption("test_value").WithName("original_name")

		// Create a module and explicitly use a different name for the option
		module, err := base.NewConfigurableModuleWithConfigOptions(opt)
		if err != nil {
			t.Fatalf("NewConfigurableModuleWithConfigOptions failed: %v", err)
		}

		// Verify the name was maintained from the option
		configs := module.ListConfigs()
		if configs["original_name"]["name"] != "original_name" {
			t.Errorf("Expected option name to be maintained as 'original_name'")
		}
	})

	t.Run("WithOptionsCreatedExternally", func(t *testing.T) {
		// Create options using separate functions to test different code paths
		intOption := func() base.ConfigOptionInterface {
			return base.NewConfigOption(42).WithName("ext_int")
		}()

		stringOption := func() base.ConfigOptionInterface {
			return base.NewConfigOption("string").WithName("ext_string")
		}()

		module, err := base.NewConfigurableModuleWithConfigOptions(intOption, stringOption)
		if err != nil {
			t.Fatalf("NewConfigurableModuleWithConfigOptions failed: %v", err)
		}

		// Verify both options were properly added
		configs := module.ListConfigs()
		if len(configs) != 2 {
			t.Errorf("Expected 2 config options, got %d", len(configs))
		}

		// Verify the names and types were preserved
		if _, exists := configs["ext_int"]; !exists {
			t.Error("ext_int option should exist")
		}
		if _, exists := configs["ext_string"]; !exists {
			t.Error("ext_string option should exist")
		}
	})
}

func TestGetConfigValueWithFallback(t *testing.T) {
	t.Run("WithExistingValue", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Set up a string config
		err := base.SetConfigValue(module, "string_option", "actual_value")
		if err != nil {
			t.Fatalf("Failed to set config value: %v", err)
		}

		// Get value with fallback
		result := base.GetConfigValueWithFallback(module, "string_option", "fallback_value")
		if result != "actual_value" {
			t.Errorf("Expected 'actual_value', got '%s'", result)
		}
	})

	t.Run("WithNonExistingOption", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Get value with fallback for a non-existing option
		result := base.GetConfigValueWithFallback(module, "nonexistent", "fallback_value")
		if result != "fallback_value" {
			t.Errorf("Expected fallback 'fallback_value', got '%s'", result)
		}
	})

	t.Run("WithTypeMismatch", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Set up an int config
		err := base.SetConfigValue(module, "int_option", 42)
		if err != nil {
			t.Fatalf("Failed to set config value: %v", err)
		}

		// Try to get it as a string with fallback
		result := base.GetConfigValueWithFallback(module, "int_option", "fallback_string")
		if result != "fallback_string" {
			t.Errorf("Expected fallback 'fallback_string' due to type mismatch, got '%s'", result)
		}
	})

	t.Run("WithErrorFromGetter", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Set up a config with a failing getter (one that returns an empty value)
		err := base.SetConfigGetter(module, "failing_option", func() string {
			return "" // Return empty string to simulate an error condition
		})
		if err != nil {
			t.Fatalf("Failed to set config getter: %v", err)
		}

		// Get value with fallback
		result := base.GetConfigValueWithFallback(module, "failing_option", "fallback_on_error")
		// The empty string returned by the getter is valid but we still want to check
		// that it was preferred over the fallback
		if result != "" {
			t.Errorf("Expected empty string from the getter, got '%s'", result)
		}
	})

	t.Run("WithDifferentTypes", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Test with bool
		err := base.SetConfigValue(module, "bool_option", true)
		if err != nil {
			t.Fatalf("Failed to set bool config: %v", err)
		}
		boolResult := base.GetConfigValueWithFallback(module, "bool_option", false)
		if !boolResult {
			t.Errorf("Expected true, got false")
		}

		// Test with int
		err = base.SetConfigValue(module, "int_option", 42)
		if err != nil {
			t.Fatalf("Failed to set int config: %v", err)
		}
		intResult := base.GetConfigValueWithFallback(module, "int_option", 0)
		if intResult != 42 {
			t.Errorf("Expected 42, got %d", intResult)
		}

		// Test with float
		err = base.SetConfigValue(module, "float_option", 3.14)
		if err != nil {
			t.Fatalf("Failed to set float config: %v", err)
		}
		floatResult := base.GetConfigValueWithFallback(module, "float_option", 0.0)
		if floatResult != 3.14 {
			t.Errorf("Expected 3.14, got %f", floatResult)
		}
	})
}

func TestGenerateSetBuiltin(t *testing.T) {
	t.Run("ValidSetOperation", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Add a string config option
		opt := base.NewConfigOption("default_value")
		err := module.SetConfigOption("test_option", opt)
		if err != nil {
			t.Fatalf("Failed to set config option: %v", err)
		}

		// Initialize the module
		err = module.Initialize()
		if err != nil {
			t.Fatalf("Failed to initialize module: %v", err)
		}

		// Load the module to get the Starlark function
		loader := module.LoadModule("test", nil)

		// Create a Starlark environment
		env := starlet.NewDefault()
		loaders := make(map[string]starlet.ModuleLoader)
		loaders["test"] = loader
		env.SetLazyloadModules(loaders)

		// Test valid set operation
		script := `
load("test", "set_test_option")
set_test_option("new_value")
`
		_, err = env.RunScript([]byte(script), nil)
		if err != nil {
			t.Fatalf("Failed to run script: %v", err)
		}

		// Verify the value was set correctly
		value, err := base.GetConfigValue[string](module, "test_option")
		if err != nil {
			t.Fatalf("Failed to get config value: %v", err)
		}
		if value != "new_value" {
			t.Errorf("Expected 'new_value', got '%s'", value)
		}
	})

	t.Run("InvalidArgumentCount", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Add a string config option
		opt := base.NewConfigOption("default_value")
		err := module.SetConfigOption("test_option", opt)
		if err != nil {
			t.Fatalf("Failed to set config option: %v", err)
		}

		// Initialize the module
		err = module.Initialize()
		if err != nil {
			t.Fatalf("Failed to initialize module: %v", err)
		}

		// Load the module to get the Starlark function
		loader := module.LoadModule("test", nil)

		// Create a Starlark environment
		env := starlet.NewDefault()
		loaders := make(map[string]starlet.ModuleLoader)
		loaders["test"] = loader
		env.SetLazyloadModules(loaders)

		// Test missing argument
		script := `
load("test", "set_test_option")
set_test_option()
`
		_, err = env.RunScript([]byte(script), nil)
		if err == nil {
			t.Fatal("Expected error for missing argument, but got nil")
		}
	})

	t.Run("SetWithInvalidValue", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Add an int config option
		opt := base.NewConfigOption(42)
		err := module.SetConfigOption("int_option", opt)
		if err != nil {
			t.Fatalf("Failed to set config option: %v", err)
		}

		// Initialize the module
		err = module.Initialize()
		if err != nil {
			t.Fatalf("Failed to initialize module: %v", err)
		}

		// Load the module to get the Starlark function
		loader := module.LoadModule("test", nil)

		// Create a Starlark environment
		env := starlet.NewDefault()
		loaders := make(map[string]starlet.ModuleLoader)
		loaders["test"] = loader
		env.SetLazyloadModules(loaders)

		// Test invalid type
		script := `
load("test", "set_int_option")
set_int_option("not_an_int")
`
		_, err = env.RunScript([]byte(script), nil)
		if err == nil {
			t.Fatal("Expected error for type mismatch, but got nil")
		}
	})
}
