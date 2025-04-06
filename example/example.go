package main

import (
	"fmt"
	"log"
	"time"

	"github.com/1set/starlet"
	"github.com/starpkg/base"
	"go.starlark.net/starlark"
)

func main() {
	// Create a new configurable module
	module := base.NewConfigurableModule()

	// Add string configuration
	strOption := base.NewConfigOption("default").
		WithName("string_option").
		WithDescription("A string configuration option")

	// Add int configuration
	intOption := base.NewConfigOption(42).
		WithName("int_option").
		WithDescription("An integer configuration option")

	// Add bool configuration with validation
	boolOption := base.NewConfigOption(false).
		WithName("bool_option").
		WithDescription("A boolean configuration option").
		WithValidator(func(v bool) error {
			if !v {
				return fmt.Errorf("value must be true")
			}
			return nil
		}).
		Required()

	// Add a map configuration instead of a struct
	complexConfig := map[string]interface{}{
		"name":     "default",
		"timeout":  time.Second.Seconds(), // Convert to seconds for Starlark compatibility
		"attempts": 3,
	}

	configOption := base.NewConfigOption(complexConfig).
		WithName("complex_option").
		WithDescription("A complex configuration option")

	// Add a secret configuration
	secretOption := base.NewConfigOption("secret-value").
		WithName("secret_option").
		WithDescription("A secret configuration option").
		Secret()

	// Add a dynamic configuration with a getter
	// Use time as a string for Starlark compatibility
	dynamicOption := base.NewConfigOption(time.Now().Format(time.RFC3339)).
		WithName("dynamic_option").
		WithDescription("A dynamic configuration option").
		WithGetter(func() string {
			return time.Now().Format(time.RFC3339)
		}).
		PreferGetter()

	// Add all configs to the module
	if err := base.SetTypedConfigOption(module, "string_option", strOption); err != nil {
		log.Fatalf("Failed to set string option: %v", err)
	}

	if err := base.SetTypedConfigOption(module, "int_option", intOption); err != nil {
		log.Fatalf("Failed to set int option: %v", err)
	}

	if err := base.SetTypedConfigOption(module, "bool_option", boolOption); err != nil {
		log.Fatalf("Failed to set bool option: %v", err)
	}

	if err := base.SetTypedConfigOption(module, "complex_option", configOption); err != nil {
		log.Fatalf("Failed to set complex option: %v", err)
	}

	if err := base.SetTypedConfigOption(module, "secret_option", secretOption); err != nil {
		log.Fatalf("Failed to set secret option: %v", err)
	}

	if err := base.SetTypedConfigOption(module, "dynamic_option", dynamicOption); err != nil {
		log.Fatalf("Failed to set dynamic option: %v", err)
	}

	// You can also use the helper functions
	if err := base.SetConfigValue(module, "string_value", "direct string value"); err != nil {
		log.Fatalf("Failed to set string value: %v", err)
	}

	if err := base.SetConfigValue(module, "float_value", 3.14159); err != nil {
		log.Fatalf("Failed to set float value: %v", err)
	}

	// Use string value for time for Starlark compatibility
	if err := base.SetConfigGetter(module, "current_time", func() string {
		return time.Now().Format(time.RFC3339)
	}); err != nil {
		log.Fatalf("Failed to set time getter: %v", err)
	}

	// Set the boolean option to true to satisfy validation
	if err := base.SetConfigValue(module, "bool_option", true); err != nil {
		log.Fatalf("Failed to set bool option: %v", err)
	}

	// Initialize the module (will validate required configurations)
	if err := module.Initialize(); err != nil {
		log.Fatalf("Failed to initialize module: %v", err)
	}

	// Get configurations from the module
	strValue, err := base.GetConfigValue[string](module, "string_option")
	if err != nil {
		log.Fatalf("Failed to get string option: %v", err)
	}
	fmt.Printf("String option: %s\n", strValue)

	intValue, err := base.GetConfigValue[int](module, "int_option")
	if err != nil {
		log.Fatalf("Failed to get int option: %v", err)
	}
	fmt.Printf("Int option: %d\n", intValue)

	// Add custom functions for Starlark
	customFuncs := starlark.StringDict{
		"get_timestamp": starlark.NewBuiltin("get_timestamp", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return starlark.MakeInt64(time.Now().Unix()), nil
		}),
	}

	// Create a Starlark module loader
	moduleLoader := module.LoadModule("config", customFuncs)

	// Print the module information
	fmt.Println("\nModule configurations:")
	for name, info := range module.ListConfigs() {
		fmt.Printf("  %s: %v\n", name, info)
	}

	// Example of running a Starlark script with the module
	fmt.Println("\nRunning Starlark script:")
	script := `
# Test configuration module

print("Configurations:")
print("  string_option:", config.get_string_option())
print("  int_option:", config.get_int_option())
print("  bool_option:", config.get_bool_option())
print("  string_value:", config.get_string_value())
print("  float_value:", config.get_float_value())
print("  current_time:", config.get_current_time())
print("  complex_option:", config.get_complex_option())
print("  timestamp:", config.get_timestamp())

# Secret value won't be available
# print("  secret_option:", config.get_secret_option())  # This would cause an error

# Update configurations
config.set_string_option("new string value")
config.set_int_option(100)
config.set_bool_option(True)

print("\nUpdated configurations:")
print("  string_option:", config.get_string_option())
print("  int_option:", config.get_int_option())
print("  bool_option:", config.get_bool_option())

# Don't try to print the configs map - it's too complex for Starlark
# print("\nAll configs:", config.list_configs())
print("\nConfiguration complete!")
`

	box := starlet.NewWithLoaders(nil, []starlet.ModuleLoader{moduleLoader}, nil)
	globals, err := box.RunScript([]byte(script), nil)
	if err != nil {
		log.Fatalf("Failed to execute Starlark script: %v", err)
	}

	// Extract keys from globals
	var keys []string
	for k := range globals {
		keys = append(keys, k)
	}

	fmt.Println("\nScript execution complete, globals:", keys)

	// Verify the updated values
	updatedStrValue, _ := base.GetConfigValue[string](module, "string_option")
	updatedIntValue, _ := base.GetConfigValue[int](module, "int_option")
	updatedBoolValue, _ := base.GetConfigValue[bool](module, "bool_option")

	fmt.Println("\nVerified updated values in Go:")
	fmt.Printf("  string_option: %s\n", updatedStrValue)
	fmt.Printf("  int_option: %d\n", updatedIntValue)
	fmt.Printf("  bool_option: %t\n", updatedBoolValue)
}
