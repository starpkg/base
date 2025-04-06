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
	// Create a new module with all configuration options
	module, err := base.NewConfigurableModuleWithOptions(
		// Basic configs: string, int, and float
		base.WithConfigValue("string_option", "default"),
		base.WithConfigValue("int_option", 42),
		base.WithConfigValue("float_value", 3.14159),

		// Boolean config with validation
		base.WithTypedConfigOption(
			"bool_option",
			base.NewConfigOption(false).
				WithName("bool_option").
				WithDescription("A boolean configuration option").
				WithValidator(func(v bool) error {
					if !v {
						return fmt.Errorf("value must be true")
					}
					return nil
				}).
				SetRequired(true),
		),

		// Complex config (map)
		base.WithConfigValue("complex_option", map[string]interface{}{
			"name":     "default",
			"timeout":  time.Second.Seconds(),
			"attempts": 3,
		}),

		// Secret config (won't be accessible from Starlark)
		base.WithTypedConfigOption(
			"secret_option",
			base.NewConfigOption("secret-value").
				WithName("secret_option").
				SetSecret(true),
		),

		// Dynamic config with getter function
		base.WithConfigGetter("current_time", func() string {
			return time.Now().Format(time.RFC3339)
		}),
	)

	if err != nil {
		log.Fatalf("Failed to create module: %v", err)
	}

	// Set boolean value to satisfy validation
	base.SetConfigValue(module, "bool_option", true)

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
load("config", "get_string_option", "get_int_option", "get_float_value", "get_bool_option", "get_current_time", "get_complex_option", "get_timestamp", "set_string_option", "set_int_option", "set_bool_option")

# Access configuration values
print("Initial values:")
print("  string_option:", get_string_option())
print("  int_option:", get_int_option())
print("  float_value:", get_float_value())
print("  bool_option:", get_bool_option())
print("  current_time:", get_current_time())
print("  complex_option:", get_complex_option())
print("  timestamp:", get_timestamp())

# Update configurations
set_string_option("new string value")
set_int_option(100)
set_bool_option(True)

print("\nUpdated values:")
print("  string_option:", get_string_option())
print("  int_option:", get_int_option())
print("  bool_option:", get_bool_option())
`

	// Run script
	machine := starlet.NewDefault()

	// Set up the module loader
	loaders := make(map[string]starlet.ModuleLoader)
	loaders["config"] = moduleLoader
	machine.SetLazyloadModules(loaders)

	// Set and run the script
	machine.SetScriptContent([]byte(script))
	_, err := machine.Run()
	if err != nil {
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
