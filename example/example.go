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
	// Create and configure the module
	module := setupModule()

	// Show current config values
	printValues(module)

	// Run Starlark script to test the module
	runStarlarkTest(module)
}

// setupModule creates and configures a module with various config types
func setupModule() *base.ConfigurableModule {
	module := base.NewConfigurableModule()

	// Basic configs: string, int, and float
	base.SetConfigValue(module, "string_option", "default")
	base.SetConfigValue(module, "int_option", 42)
	base.SetConfigValue(module, "float_value", 3.14159)

	// Boolean config with validation
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
	base.SetTypedConfigOption(module, "bool_option", boolOption)
	base.SetConfigValue(module, "bool_option", true) // Set to satisfy validation

	// Complex config (map)
	complexConfig := map[string]interface{}{
		"name":     "default",
		"timeout":  time.Second.Seconds(),
		"attempts": 3,
	}
	base.SetConfigValue(module, "complex_option", complexConfig)

	// Secret config (won't be accessible from Starlark)
	base.SetConfigValue(module, "secret_option", "secret-value")
	secretOption, err := module.GetConfigOption("secret_option")
	if err != nil {
		log.Fatalf("Failed to get secret_option: %v", err)
	}
	secretOption.SetSecret(true)

	// Dynamic config with getter function
	base.SetConfigGetter(module, "current_time", func() string {
		return time.Now().Format(time.RFC3339)
	})

	// Initialize the module
	if err := module.Initialize(); err != nil {
		log.Fatalf("Failed to initialize module: %v", err)
	}

	return module
}

// printValues shows the current configuration values
func printValues(module *base.ConfigurableModule) {
	fmt.Println("Configuration Values:")

	strVal, _ := base.GetConfigValue[string](module, "string_option")
	intVal, _ := base.GetConfigValue[int](module, "int_option")
	floatVal, _ := base.GetConfigValue[float64](module, "float_value")
	boolVal, _ := base.GetConfigValue[bool](module, "bool_option")
	timeVal, _ := base.GetConfigValue[string](module, "current_time")

	fmt.Printf("  string_option: %s\n", strVal)
	fmt.Printf("  int_option: %d\n", intVal)
	fmt.Printf("  float_value: %g\n", floatVal)
	fmt.Printf("  bool_option: %t\n", boolVal)
	fmt.Printf("  current_time: %s\n", timeVal)

	// Secret values won't be accessible
	secretVal, err := base.GetConfigValue[string](module, "secret_option")
	fmt.Printf("  secret_option: %v (Error: %v)\n", secretVal, err)

	fmt.Println()
}

// runStarlarkTest runs a Starlark script that interacts with the module
func runStarlarkTest(module *base.ConfigurableModule) {
	// Add a timestamp function for demonstration
	customFuncs := starlark.StringDict{
		"get_timestamp": starlark.NewBuiltin("get_timestamp", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			return starlark.MakeInt64(time.Now().Unix()), nil
		}),
	}

	// Create module loader
	moduleLoader := module.LoadModule("config", customFuncs)

	fmt.Println("Running Starlark script:")
	script := `
# Access configuration values
print("Initial values:")
print("  string_option:", config.get_string_option())
print("  int_option:", config.get_int_option())
print("  float_value:", config.get_float_value())
print("  bool_option:", config.get_bool_option())
print("  current_time:", config.get_current_time())
print("  complex_option:", config.get_complex_option())
print("  timestamp:", config.get_timestamp())

# Update configurations
config.set_string_option("new string value")
config.set_int_option(100)
config.set_bool_option(True)

print("\nUpdated values:")
print("  string_option:", config.get_string_option())
print("  int_option:", config.get_int_option())
print("  bool_option:", config.get_bool_option())
`

	// Run script
	box := starlet.NewWithLoaders(nil, []starlet.ModuleLoader{moduleLoader}, nil)
	if _, err := box.RunScript([]byte(script), nil); err != nil {
		log.Fatalf("Script execution failed: %v", err)
	}

	// Verify the updated values in Go
	fmt.Println("\nVerified updated values in Go:")
	strVal, _ := base.GetConfigValue[string](module, "string_option")
	intVal, _ := base.GetConfigValue[int](module, "int_option")
	boolVal, _ := base.GetConfigValue[bool](module, "bool_option")

	fmt.Printf("  string_option: %s\n", strVal)
	fmt.Printf("  int_option: %d\n", intVal)
	fmt.Printf("  bool_option: %t\n", boolVal)
}
