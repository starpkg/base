package base_test

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/1set/starlet"
	"github.com/starpkg/base"
	"go.starlark.net/starlark"
)

// ExampleModuleUsage demonstrates the complete process of:
// 1. Creating a configurable module
// 2. Adding and configuring options with chain assignment
// 3. Running a Starlark script with the module
// 4. Verifying the updated values
func Example_moduleUsage() {
	// === PART 1: Create a configurable module ===
	module := base.NewConfigurableModule()

	// === PART 2: Configure options using chain assignment ===

	// String option with description and validation
	base.SetTypedConfigOption(
		module,
		"name",
		base.NewConfigOption("default").
			WithDescription("The name to use for the operation").
			WithValidator(func(v string) error {
				if len(v) < 3 {
					return fmt.Errorf("name must be at least 3 characters")
				}
				return nil
			}),
	)

	// Integer option with description and default
	base.SetTypedConfigOption(
		module,
		"timeout",
		base.NewConfigOption(30).
			WithDescription("The timeout in seconds").
			WithValidator(func(v int) error {
				if v <= 0 {
					return fmt.Errorf("timeout must be positive")
				}
				return nil
			}),
	)

	// Boolean option that's required
	base.SetTypedConfigOption(
		module,
		"debug",
		base.NewConfigOption(false).
			WithDescription("Whether to enable debug mode").
			Required(),
	)

	// Set a value for the required debug option
	base.SetConfigValue(module, "debug", false)

	// Array option
	base.SetTypedConfigOption(
		module,
		"tags",
		base.NewConfigOption([]string{"default", "test"}).
			WithDescription("Tags for categorization"),
	)

	// Map option
	base.SetTypedConfigOption(
		module,
		"config",
		base.NewConfigOption(map[string]interface{}{
			"retries": 3,
			"delay":   1.5,
		}).WithDescription("Advanced configuration options"),
	)

	// Secret option
	base.SetTypedConfigOption(
		module,
		"api_key",
		base.NewConfigOption("default-key").
			WithDescription("API Key for authentication").
			Secret(),
	)

	// Dynamic option with getter function
	base.SetConfigGetter(
		module,
		"timestamp",
		func() int64 {
			return time.Now().Unix()
		},
	)

	// Initialize the module
	if err := module.Initialize(); err != nil {
		log.Fatalf("Failed to initialize module: %v", err)
	}

	// === PART 3: Add custom Starlark functions ===
	customFuncs := starlark.StringDict{
		"get_current_time": starlark.NewBuiltin("get_current_time", func(
			thread *starlark.Thread,
			_ *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {
			format := "2006-01-02 15:04:05"
			if len(args) > 0 {
				if s, ok := args[0].(starlark.String); ok {
					format = string(s)
				}
			}
			return starlark.String(time.Now().Format(format)), nil
		}),
	}

	// Create module loader
	moduleLoader := module.LoadModule("mymodule", customFuncs)

	// === PART 4: Run Starlark script using Starlet machine ===
	script := `
load("mymodule", "get_name", "set_name", "get_timeout", "set_timeout", 
     "get_debug", "set_debug", "get_tags", "set_tags", 
     "get_config", "set_config", "get_timestamp", "get_current_time")

# Print initial values
print("Initial configuration:")
print("  name:", get_name())
print("  timeout:", get_timeout())
print("  debug:", get_debug())
print("  tags:", get_tags())
print("  config:", get_config())
print("  timestamp:", get_timestamp())
print("  current_time:", get_current_time())

# Modify values
set_name("production")
set_timeout(60)
set_debug(True)
set_tags(["production", "live", "v1"])
set_config({"retries": 5, "delay": 2.0, "monitoring": True})

# Print modified values
print("\nUpdated configuration:")
print("  name:", get_name())
print("  timeout:", get_timeout())
print("  debug:", get_debug())
print("  tags:", get_tags())
print("  config:", get_config())

# Test using custom format with the time function
print("  formatted time:", get_current_time("2006-01-02"))
`

	// Create a starlet machine with our module
	machine := starlet.NewDefault()

	// Set up the module loader
	loaders := make(map[string]starlet.ModuleLoader)
	loaders["mymodule"] = moduleLoader
	machine.SetLazyloadModules(loaders)

	// Set and run the script
	machine.SetScriptContent([]byte(script))
	_, err := machine.Run()
	if err != nil {
		log.Fatalf("Failed to run script: %v", err)
	}

	// === PART 5: Verify the updated values ===
	fmt.Println("\nVerifying updated values in Go:")

	name, _ := base.GetConfigValue[string](module, "name")
	fmt.Printf("  name: %s\n", name)

	timeout, _ := base.GetConfigValue[int](module, "timeout")
	fmt.Printf("  timeout: %d\n", timeout)

	debug, _ := base.GetConfigValue[bool](module, "debug")
	fmt.Printf("  debug: %t\n", debug)

	tags, _ := base.GetConfigValue[[]string](module, "tags")
	fmt.Printf("  tags: %v\n", tags)

	config, _ := base.GetConfigValue[map[string]interface{}](module, "config")
	fmt.Printf("  config: %v\n", config)

	// Secret values can't be retrieved
	_, err = base.GetConfigValue[string](module, "api_key")
	fmt.Printf("  api_key: %v\n", err)

	// Output:
	// Verifying updated values in Go:
	//   name: production
	//   timeout: 60
	//   debug: true
	//   tags: [production live v1]
	//   config: map[delay:2 monitoring:true retries:5]
	//   api_key: secret configuration is not retrievable
}

// Example_complexModule demonstrates building a more complex module
// with multiple option types and complex validation logic
func Example_complexModule() {
	// Create a new module for a hypothetical HTTP client configuration
	module := base.NewConfigurableModule()

	// Base URL with validation
	base.SetTypedConfigOption(
		module,
		"base_url",
		base.NewConfigOption("https://api.example.com").
			WithDescription("The base URL for API requests").
			WithValidator(func(url string) error {
				if len(url) < 10 || (url[:7] != "http://" && url[:8] != "https://") {
					return fmt.Errorf("invalid URL format: must start with http:// or https://")
				}
				return nil
			}).
			Required(),
	)

	// Authentication options
	base.SetTypedConfigOption(
		module,
		"auth",
		base.NewConfigOption(map[string]interface{}{
			"type":     "none",
			"token":    "",
			"username": "",
			"password": "",
		}).WithDescription("Authentication configuration").
			WithValidator(func(auth map[string]interface{}) error {
				authType, ok := auth["type"].(string)
				if !ok {
					return fmt.Errorf("auth must have a 'type' string field")
				}

				switch authType {
				case "none":
					// No validation needed
				case "token":
					token, ok := auth["token"].(string)
					if !ok || token == "" {
						return fmt.Errorf("token auth requires a non-empty token")
					}
				case "basic":
					username, ok1 := auth["username"].(string)
					password, ok2 := auth["password"].(string)
					if !ok1 || !ok2 || username == "" || password == "" {
						return fmt.Errorf("basic auth requires username and password")
					}
				default:
					return fmt.Errorf("unsupported auth type: %s", authType)
				}
				return nil
			}),
	)

	// Request options with nested validation
	base.SetTypedConfigOption(
		module,
		"request",
		base.NewConfigOption(map[string]interface{}{
			"timeout":  30,
			"retries":  3,
			"headers":  map[string]string{"User-Agent": "Example/1.0"},
			"verify":   true,
			"encoding": "json",
		}).WithDescription("HTTP request options").
			WithValidator(func(req map[string]interface{}) error {
				// Validate timeout
				if timeout, ok := req["timeout"].(int); !ok || timeout <= 0 {
					return fmt.Errorf("timeout must be a positive integer")
				}

				// Validate retries
				if retries, ok := req["retries"].(int); !ok || retries < 0 {
					return fmt.Errorf("retries must be a non-negative integer")
				}

				// Validate encoding
				if encoding, ok := req["encoding"].(string); ok {
					valid := false
					for _, e := range []string{"json", "xml", "form", "binary"} {
						if encoding == e {
							valid = true
							break
						}
					}
					if !valid {
						return fmt.Errorf("encoding must be one of: json, xml, form, binary")
					}
				}

				return nil
			}),
	)

	// Initialize the module
	if err := module.Initialize(); err != nil {
		log.Fatalf("Failed to initialize module: %v", err)
	}

	// Add custom functions for the HTTP client
	customFuncs := starlark.StringDict{
		"make_request": starlark.NewBuiltin("make_request", func(
			thread *starlark.Thread,
			_ *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {
			// In a real implementation, this would make an actual HTTP request
			// using the configured settings.
			// Here we just return a mock response for demonstration.
			var path, method string
			if len(args) > 0 {
				if s, ok := args[0].(starlark.String); ok {
					path = string(s)
				}
			}
			if len(args) > 1 {
				if s, ok := args[1].(starlark.String); ok {
					method = string(s)
				}
			}
			if method == "" {
				method = "GET"
			}

			baseURL, _ := base.GetConfigValue[string](module, "base_url")
			reqOptions, _ := base.GetConfigValue[map[string]interface{}](module, "request")
			timeout := reqOptions["timeout"]

			return starlark.String(fmt.Sprintf(
				"Mock Response: %s %s%s (timeout: %v)",
				method, baseURL, path, timeout,
			)), nil
		}),
	}

	// Create module loader
	moduleLoader := module.LoadModule("http_client", customFuncs)

	// Run a Starlark script with our HTTP client module
	script := `
load("http_client", "get_base_url", "set_base_url", "get_auth", "set_auth", "get_request", "set_request", "make_request")

# Print initial configuration
print("HTTP Client Configuration:")
print("  Base URL:", get_base_url())
print("  Auth:", get_auth())
print("  Request Options:", get_request())

# Update configuration
set_base_url("https://api.production.com")
set_auth({
    "type": "token",
    "token": "secure-api-token-12345",
})
request_opts = get_request()
request_opts["timeout"] = 60
request_opts["retries"] = 5
request_opts["headers"]["X-Api-Version"] = "2"
set_request(request_opts)

# Make some API requests
response1 = make_request("/users")
response2 = make_request("/data", "POST")

print("\nResponses:")
print("  Response 1:", response1)
print("  Response 2:", response2)
`

	// Create a starlet machine with our module
	machine := starlet.NewDefault()

	// Set up the module loader
	loaders := make(map[string]starlet.ModuleLoader)
	loaders["http_client"] = moduleLoader
	machine.SetLazyloadModules(loaders)

	// Set and run the script
	machine.SetScriptContent([]byte(script))
	_, _ = machine.Run()

	// Verify the updated configuration
	fmt.Println("\nVerified HTTP client configuration:")

	baseURL, _ := base.GetConfigValue[string](module, "base_url")
	fmt.Printf("  Base URL: %s\n", baseURL)

	auth, _ := base.GetConfigValue[map[string]interface{}](module, "auth")
	fmt.Printf("  Auth Type: %s\n", auth["type"])
	fmt.Printf("  Auth Token: %s\n", auth["token"])

	req, _ := base.GetConfigValue[map[string]interface{}](module, "request")
	fmt.Printf("  Timeout: %v\n", req["timeout"])
	fmt.Printf("  Retries: %v\n", req["retries"])

	// Output:
	// Verified HTTP client configuration:
	//   Base URL: https://api.production.com
	//   Auth Type: token
	//   Auth Token: secure-api-token-12345
	//   Timeout: 60
	//   Retries: 5
}

// Example_multipleModules demonstrates how to use multiple modules together
func Example_multipleModules() {
	// Create modules for database and logging components
	dbModule := base.NewConfigurableModule()
	logModule := base.NewConfigurableModule()

	// Configure database module
	base.SetConfigValue(dbModule, "host", "localhost")
	base.SetConfigValue(dbModule, "port", 5432)
	base.SetConfigValue(dbModule, "username", "user")

	// Set password as secret option
	passwordOpt := base.NewConfigOption("password").Secret()
	base.SetTypedConfigOption(dbModule, "password", passwordOpt)

	base.SetConfigValue(dbModule, "database", "myapp")
	base.SetConfigValue(dbModule, "pool_size", 10)
	dbModule.Initialize()

	// Configure logging module
	base.SetConfigValue(logModule, "level", "info")
	base.SetConfigValue(logModule, "file", "/var/log/myapp.log")
	base.SetConfigValue(logModule, "format", "json")
	base.SetConfigValue(logModule, "rotate", true)
	base.SetConfigValue(logModule, "max_size", 100)
	logModule.Initialize()

	// Custom functions for database module
	dbFuncs := starlark.StringDict{
		"query": starlark.NewBuiltin("query", func(
			thread *starlark.Thread,
			_ *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("query requires at least one argument")
			}

			host, _ := base.GetConfigValue[string](dbModule, "host")
			database, _ := base.GetConfigValue[string](dbModule, "database")

			// Get the query string
			var queryStr string
			if s, ok := args[0].(starlark.String); ok {
				queryStr = string(s)
			} else {
				queryStr = args[0].String()
			}

			// Mock query result
			return starlark.String(fmt.Sprintf(
					"Query result from %s/%s: %s",
					host, database, queryStr)),
				nil
		}),
	}

	// Custom functions for logging module
	logFuncs := starlark.StringDict{
		"log": starlark.NewBuiltin("log", func(
			thread *starlark.Thread,
			_ *starlark.Builtin,
			args starlark.Tuple,
			kwargs []starlark.Tuple,
		) (starlark.Value, error) {
			if len(args) == 0 {
				return nil, fmt.Errorf("log requires at least one argument")
			}

			level := "info"
			if len(args) > 1 {
				if s, ok := args[1].(starlark.String); ok {
					level = string(s)
				}
			}

			configLevel, _ := base.GetConfigValue[string](logModule, "level")
			format, _ := base.GetConfigValue[string](logModule, "format")

			// Get the message as unquoted string
			var message string
			if s, ok := args[0].(starlark.String); ok {
				message = string(s)
			} else {
				message = args[0].String()
			}

			// Mock log output
			fmt.Printf("Log [%s] (%s format) [%s]: %s\n",
				level, format, configLevel, message)

			return starlark.None, nil
		}),
	}

	// Create module loaders
	dbLoader := dbModule.LoadModule("db", dbFuncs)
	logLoader := logModule.LoadModule("log", logFuncs)

	// Script that uses both modules
	script := `
load("db", "get_host", "set_host", "get_database", "query")
load("log", "get_level", "set_level", "log")

# Log the startup
log("Application starting")

# Get and update database config
print("Database:", get_host() + "/" + get_database())
set_host("db.production.example.com")

# Run a query and log the result
result = query("SELECT count(*) FROM users")
log(result, "debug")

# Change log level and log again
set_level("verbose")
log("Log level changed", "info")
`

	// Create a starlet machine with both modules
	machine := starlet.NewDefault()

	// Set up the module loaders
	loaders := make(map[string]starlet.ModuleLoader)
	loaders["db"] = dbLoader
	loaders["log"] = logLoader
	machine.SetLazyloadModules(loaders)

	// Set and run the script
	machine.SetScriptContent([]byte(script))
	_, _ = machine.Run()

	// Check the final configuration
	dbHost, _ := base.GetConfigValue[string](dbModule, "host")
	logLevel, _ := base.GetConfigValue[string](logModule, "level")

	fmt.Println("\nVerified multi-module configuration:")
	fmt.Printf("  DB Host: %s\n", dbHost)
	fmt.Printf("  Log Level: %s\n", logLevel)

	// Output:
	// Log [info] (json format) [info]: Application starting
	// Log [debug] (json format) [info]: Query result from db.production.example.com/myapp: SELECT count(*) FROM users
	// Log [info] (json format) [verbose]: Log level changed
	//
	// Verified multi-module configuration:
	//   DB Host: db.production.example.com
	//   Log Level: verbose
}

// TestExamples ensures the examples compile
func TestExamples(t *testing.T) {
	// This is just a placeholder test to ensure the examples compile
	// The actual examples are run by the go test tool when it's called with the -v flag
}
