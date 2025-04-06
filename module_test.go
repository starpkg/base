package base_test

import (
	"errors"
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

	// Test required configs
	t.Run("RequiredConfigs", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Add a required config without a value
		module.SetConfigOption("required", base.NewConfigOption("").Required())

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
		opt2 := base.NewConfigOption("value2").Secret()
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

	// Test ListConfigs
	t.Run("ListConfigs", func(t *testing.T) {
		module := base.NewConfigurableModule()

		// Add a couple config options
		opt1 := base.NewConfigOption("value1").WithDescription("Description 1")
		opt2 := base.NewConfigOption("value2").WithDescription("Description 2").Secret()
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

		// With default PrioritySetValue, the explicit value should take precedence
		typedOpt.SetValue("explicit")
		val, err = base.GetConfigValue[string](module, "getter_option")
		if err != nil {
			t.Fatalf("GetConfigValue failed: %v", err)
		}
		if val != "explicit" {
			t.Errorf("Expected 'explicit', got '%s'", val)
		}

		// Change to prefer getter
		typedOpt.PreferGetter()
		val, err = base.GetConfigValue[string](module, "getter_option")
		if err != nil {
			t.Fatalf("GetConfigValue failed: %v", err)
		}
		if val != "updated" {
			t.Errorf("Expected 'updated', got '%s'", val)
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
}
