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
	module := NewConfigurableModule()

	// Set a string config and try to retrieve it as an int
	module.SetConfigOption("string_config", NewConfigOption("test"))
	module.Initialize()

	ext := module.Extend()

	// This should return the default value because of type mismatch
	if v := ext.GetInt("string_config", 999); v != 999 {
		t.Errorf("Expected GetInt on string config to return default 999, got %d", v)
	}

	// Test with secret config
	secretModule := NewConfigurableModule()
	secretModule.SetConfigOption("api_key", NewConfigOption("secret_key").SetSecret(true))
	secretModule.Initialize()

	secretExt := secretModule.Extend()

	// This should return the default value because it's secret
	if v := secretExt.GetString("api_key", "default_key"); v != "default_key" {
		t.Errorf("Expected GetString on secret config to return default, got %s", v)
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
