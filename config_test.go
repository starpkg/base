package base_test

import (
	"testing"

	"github.com/starpkg/base"
	"go.starlark.net/starlark"
)

func TestConfigOption(t *testing.T) {
	// Test basic config option with default value
	t.Run("Basic", func(t *testing.T) {
		opt := base.NewConfigOption("default")

		// Test default value
		val, err := opt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}
		if val != "default" {
			t.Errorf("Expected default value 'default', got '%s'", val)
		}

		// Test HasDefault
		if !opt.HasDefault() {
			t.Error("HasDefault should return true for non-zero default value")
		}

		// Test builder methods
		opt.WithName("test_option").WithDescription("A test option")
		if opt.GetName() != "test_option" {
			t.Errorf("Expected name 'test_option', got '%s'", opt.GetName())
		}

		// Test SetValue and GetValue
		err = opt.SetValue("new_value")
		if err != nil {
			t.Fatalf("SetValue failed: %v", err)
		}

		val, err = opt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}
		if val != "new_value" {
			t.Errorf("Expected 'new_value', got '%s'", val)
		}

		// Test HasValue
		if !opt.HasValue() {
			t.Error("HasValue should return true after setting a value")
		}
	})

	// Test validator
	t.Run("Validator", func(t *testing.T) {
		opt := base.NewConfigOption(0).WithValidator(func(val int) error {
			if val < 0 {
				return base.ErrConfigInvalidValue
			}
			return nil
		})

		// Valid value should succeed
		if err := opt.SetValue(10); err != nil {
			t.Errorf("Expected valid value to pass validation, got error: %v", err)
		}

		// Invalid value should fail
		if err := opt.SetValue(-5); err == nil {
			t.Error("Expected invalid value to fail validation")
		}
	})

	// Test WithValue
	t.Run("WithValue", func(t *testing.T) {
		// Test basic WithValue functionality
		opt := base.NewConfigOption("default").WithValue("initial_value")

		val, err := opt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}
		if val != "initial_value" {
			t.Errorf("Expected 'initial_value', got '%s'", val)
		}

		// Test WithValue with validator
		validatedOpt := base.NewConfigOption(0).
			WithValidator(func(val int) error {
				if val < 0 {
					return base.ErrConfigInvalidValue
				}
				return nil
			}).
			WithValue(-10) // Should pass even with invalid value since WithValue ignores validators

		valInt, err := validatedOpt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}
		if valInt != -10 {
			t.Errorf("Expected -10, got %d", valInt)
		}

		// Now try direct SetValue which should enforce validation
		err = validatedOpt.SetValue(-5)
		if err == nil {
			t.Error("Expected SetValue to fail validation with negative number, but it succeeded")
		}

		// Test chain of builder methods with WithValue
		chainedOpt := base.NewConfigOption("").
			WithName("option_name").
			WithDescription("An option with a value").
			WithValue("chain_value").
			Required()

		if !chainedOpt.HasValue() {
			t.Error("HasValue should return true after using WithValue")
		}

		chainVal, err := chainedOpt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}
		if chainVal != "chain_value" {
			t.Errorf("Expected 'chain_value', got '%s'", chainVal)
		}
	})

	// Test getter
	t.Run("Getter", func(t *testing.T) {
		dynamicValue := "initial"
		opt := base.NewConfigOption("default").WithGetter(func() string {
			return dynamicValue
		})

		// Test default priority (PrioritySetValue)
		val, err := opt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}
		if val != dynamicValue {
			t.Errorf("Expected dynamic value '%s', got '%s'", dynamicValue, val)
		}

		// Set a value and check priority
		err = opt.SetValue("explicit")
		if err != nil {
			t.Fatalf("SetValue failed: %v", err)
		}

		val, err = opt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}
		if val != "explicit" {
			t.Errorf("Expected explicit value 'explicit', got '%s'", val)
		}

		// Test PreferGetter
		opt.PreferGetter()

		dynamicValue = "updated"
		val, err = opt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}
		if val != "updated" {
			t.Errorf("Expected updated dynamic value 'updated', got '%s'", val)
		}

		// Test PreferSetValue again
		opt.PreferSetValue()

		val, err = opt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}
		if val != "explicit" {
			t.Errorf("Expected explicit value 'explicit', got '%s'", val)
		}
	})

	// Test secret
	t.Run("Secret", func(t *testing.T) {
		opt := base.NewConfigOption("secret_value").Secret()

		// Check that secret is set
		if !opt.IsSecret() {
			t.Error("IsSecret should return true for secret configs")
		}

		// GetValue should return an error for secret configs
		_, err := opt.GetValue()
		if err == nil {
			t.Error("GetValue should return error for secret configs")
		}

		// But we should still be able to set values
		err = opt.SetValue("new_secret")
		if err != nil {
			t.Fatalf("SetValue failed for secret config: %v", err)
		}
	})

	// Test required
	t.Run("Required", func(t *testing.T) {
		opt := base.NewConfigOption("").Required()

		// Check that required is set
		if !opt.IsRequired() {
			t.Error("IsRequired should return true for required configs")
		}
	})

	// Test Starlark integration
	t.Run("StarlarkIntegration", func(t *testing.T) {
		opt := base.NewConfigOption("default")

		// Set from Starlark string
		err := opt.SetValueFromStarlark(starlark.String("starlark_value"))
		if err != nil {
			t.Fatalf("SetValueFromStarlark failed: %v", err)
		}

		// Get value as Go string
		val, err := opt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}
		if val != "starlark_value" {
			t.Errorf("Expected 'starlark_value', got '%s'", val)
		}

		// Get value as Starlark value
		sval, err := opt.GetStarlarkValue()
		if err != nil {
			t.Fatalf("GetStarlarkValue failed: %v", err)
		}
		if sval.String() != `"starlark_value"` {
			t.Errorf("Expected Starlark string \"starlark_value\", got %s", sval.String())
		}

		// Test type mismatch
		err = opt.SetValueFromStarlark(starlark.MakeInt(42))
		if err == nil {
			t.Error("Expected type mismatch error, got nil")
		}
	})

	// Test GetInfo
	t.Run("GetInfo", func(t *testing.T) {
		opt := base.NewConfigOption("test_val").
			WithName("test_name").
			WithDescription("Test description").
			Required()

		info := opt.GetInfo()

		if info["name"] != "test_name" {
			t.Errorf("Expected name 'test_name', got '%v'", info["name"])
		}

		if info["description"] != "Test description" {
			t.Errorf("Expected description 'Test description', got '%v'", info["description"])
		}

		if info["required"] != true {
			t.Errorf("Expected required true, got %v", info["required"])
		}

		if info["value"] != "test_val" {
			t.Errorf("Expected value 'test_val', got '%v'", info["value"])
		}

		// Test that secret values don't expose their values
		secretOpt := base.NewConfigOption("secret").Secret()
		secretInfo := secretOpt.GetInfo()

		if secretInfo["secret"] != true {
			t.Errorf("Expected secret true, got %v", secretInfo["secret"])
		}

		if _, exists := secretInfo["value"]; exists {
			t.Error("Secret config should not include value in info")
		}
	})
}
