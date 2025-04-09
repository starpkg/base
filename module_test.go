package base_test

import (
	"errors"
	"fmt"
	"testing"

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
		module.SetConfigOption("non_required", base.NewConfigOption("default"))

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

		// Execute the module to verify functionality
		dict, err := loader()
		if err != nil {
			t.Fatalf("Module execution failed: %v", err)
		}

		// Check that getter functions exist for non-secret configs
		if _, ok := dict["get_option1"]; !ok {
			t.Error("Expected get_option1 function to exist")
		}

		// Check that getter functions don't exist for secret configs
		if _, ok := dict["get_option2"]; ok {
			t.Error("get_option2 function should not exist for secret config")
		}

		// Check that setter functions exist for all configs
		if _, ok := dict["set_option1"]; !ok {
			t.Error("Expected set_option1 function to exist")
		}
		if _, ok := dict["set_option2"]; !ok {
			t.Error("Expected set_option2 function to exist")
		}

		// Check that the additional function exists
		if _, ok := dict["custom_func"]; !ok {
			t.Error("Expected custom_func to exist")
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

	// Verify the options were set
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
