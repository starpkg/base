package base

import (
	"testing"
)

func TestConfigurableModuleExt(t *testing.T) {
	// Test setup
	module := NewConfigurableModule()

	// Add various types of configs to test
	module.SetConfigOption("string_config", NewConfigOption("test_value"))
	module.SetConfigOption("int_config", NewConfigOption(42))
	module.SetConfigOption("uint_config", NewConfigOption(uint(24)))
	module.SetConfigOption("bool_config", NewConfigOption(true))
	module.SetConfigOption("float_config", NewConfigOption(3.14159))

	// Test non-existent config with default
	module.Initialize()

	// Get extension wrapper
	ext := module.Extend()

	// Test GetString with existing config
	if v := ext.GetString("string_config"); v != "test_value" {
		t.Errorf("Expected GetString to return 'test_value', got '%s'", v)
	}

	// Test GetString with non-existent config and default
	if v := ext.GetString("non_existent", "default"); v != "default" {
		t.Errorf("Expected GetString to return default 'default', got '%s'", v)
	}

	// Test GetInt with existing config
	if v := ext.GetInt("int_config"); v != 42 {
		t.Errorf("Expected GetInt to return 42, got %d", v)
	}

	// Test GetInt with non-existent config and default
	if v := ext.GetInt("non_existent", 100); v != 100 {
		t.Errorf("Expected GetInt to return default 100, got %d", v)
	}

	// Test GetUint with existing config
	if v := ext.GetUint("uint_config"); v != 24 {
		t.Errorf("Expected GetUint to return 24, got %d", v)
	}

	// Test GetUint with non-existent config and default
	if v := ext.GetUint("non_existent", 50); v != 50 {
		t.Errorf("Expected GetUint to return default 50, got %d", v)
	}

	// Test GetBool with existing config
	if v := ext.GetBool("bool_config"); v != true {
		t.Errorf("Expected GetBool to return true, got %v", v)
	}

	// Test GetBool with non-existent config and default
	if v := ext.GetBool("non_existent", false); v != false {
		t.Errorf("Expected GetBool to return default false, got %v", v)
	}

	// Test GetFloat with existing config
	if v := ext.GetFloat("float_config"); v != 3.14159 {
		t.Errorf("Expected GetFloat to return 3.14159, got %f", v)
	}

	// Test GetFloat with non-existent config and default
	if v := ext.GetFloat("non_existent", 2.5); v != 2.5 {
		t.Errorf("Expected GetFloat to return default 2.5, got %f", v)
	}
}

func TestConfigurableModuleExtErrors(t *testing.T) {
	// Test error cases for extension methods
	// Test with non-existent config
	module := NewConfigurableModule()
	module.Initialize()

	ext := module.Extend()

	// Test with non-existent configs
	if v := ext.GetString("non_existent", "default_str"); v != "default_str" {
		t.Errorf("Expected GetString for non-existent config to return default, got %s", v)
	}

	if v := ext.GetInt("non_existent", 42); v != 42 {
		t.Errorf("Expected GetInt for non-existent config to return default, got %d", v)
	}

	if v := ext.GetUint("non_existent", 42); v != 42 {
		t.Errorf("Expected GetUint for non-existent config to return default, got %d", v)
	}

	// Test with secret config (secret values should now be accessible)
	secretModule := NewConfigurableModule()
	secretModule.SetConfigOption("api_key", NewConfigOption("secret_key").SetSecret(true))
	secretModule.Initialize()

	secretExt := secretModule.Extend()

	// Since we changed the behavior, secret values should now be retrievable
	if v := secretExt.GetString("api_key", "default_key"); v != "secret_key" {
		t.Errorf("Expected GetString on secret config to return the secret value 'secret_key', got '%s'", v)
	}
}

func TestConfigurableModuleExtWithFunctionalConfigs(t *testing.T) {
	module := NewConfigurableModule()

	// Add a dynamic getter config
	dynamicValue := "initial"
	module.SetConfigOption("dynamic_config", NewConfigOption("").WithGetter(func() string {
		return dynamicValue
	}))

	module.Initialize()
	ext := module.Extend()

	// Test that we get the initial dynamic value
	if v := ext.GetString("dynamic_config"); v != "initial" {
		t.Errorf("Expected GetString for dynamic config to return 'initial', got '%s'", v)
	}

	// Change the dynamic value and verify we get the new value
	dynamicValue = "updated"
	if v := ext.GetString("dynamic_config"); v != "updated" {
		t.Errorf("Expected GetString for dynamic config to return 'updated', got '%s'", v)
	}

	// Test with non-existent dynamic config
	if v := ext.GetString("non_existent_dynamic", "fallback"); v != "fallback" {
		t.Errorf("Expected GetString for non-existent dynamic config to return 'fallback', got '%s'", v)
	}
}
