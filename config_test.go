package base_test

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/starpkg/base"
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

		// Direct validation should still work and fail for the invalid value
		err = validatedOpt.Validate()
		if err == nil {
			t.Error("Expected Validate() to fail for invalid value, but it succeeded")
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
			SetRequired(true)

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

		// Test that getter value is used when no value is explicitly set
		val, err := opt.GetValue()
		if err != nil {
			t.Fatalf("GetValue failed: %v", err)
		}
		if val != dynamicValue {
			t.Errorf("Expected dynamic value '%s', got '%s'", dynamicValue, val)
		}

		// Set a value and verify it takes precedence over getter
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

		// Update dynamic value and verify it doesn't affect result
		// as explicit value takes precedence
		dynamicValue = "updated"
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
		opt := base.NewConfigOption("secret_value").SetSecret(true)

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
		opt := base.NewConfigOption("").SetRequired(true)

		// Check that required is set
		if !opt.IsRequired() {
			t.Error("IsRequired should return true for required configs")
		}
	})

	// Test GetInfo
	t.Run("GetInfo", func(t *testing.T) {
		opt := base.NewConfigOption("test_val").
			WithName("test_name").
			WithDescription("Test description").
			SetRequired(true)

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
		secretOpt := base.NewConfigOption("secret").SetSecret(true)
		secretInfo := secretOpt.GetInfo()

		if secretInfo["secret"] != true {
			t.Errorf("Expected secret true, got %v", secretInfo["secret"])
		}

		if _, exists := secretInfo["value"]; exists {
			t.Error("Secret config should not include value in info")
		}
	})

	// Test SetSecret method separately
	t.Run("SetSecretMethod", func(t *testing.T) {
		// Test that SetSecret works correctly
		opt := base.NewConfigOption("default")
		opt = opt.SetSecret(true)

		if !opt.IsSecret() {
			t.Error("SetSecret(true) should make the option secret")
		}

		// Test that we can toggle it back
		opt = opt.SetSecret(false)

		if opt.IsSecret() {
			t.Error("SetSecret(false) should make the option not secret")
		}

		// Verify we can get the value when not secret
		_, err := opt.GetValue()
		if err != nil {
			t.Errorf("Should be able to get value after SetSecret(false): %v", err)
		}
	})

	// Test priority order of value resolution
	t.Run("PriorityOrder", func(t *testing.T) {
		// Test that immediate value takes precedence over getter
		t.Run("ImmediateValueOverGetter", func(t *testing.T) {
			opt := base.NewConfigOption("default").
				WithGetter(func() string { return "getter_value" }).
				WithValue("immediate_value")

			val, err := opt.GetValue()
			if err != nil {
				t.Fatalf("GetValue failed: %v", err)
			}
			if val != "immediate_value" {
				t.Errorf("Expected immediate value 'immediate_value', got '%s'", val)
			}
		})

		// Test that getter takes precedence over environment variable
		t.Run("GetterOverEnvVar", func(t *testing.T) {
			opt := base.NewConfigOption("default").
				WithGetter(func() string { return "getter_value" }).
				WithEnvVar("TEST_ENV_VAR")

			os.Setenv("TEST_ENV_VAR", "env_value")
			defer os.Unsetenv("TEST_ENV_VAR")

			val, err := opt.GetValue()
			if err != nil {
				t.Fatalf("GetValue failed: %v", err)
			}
			if val != "getter_value" {
				t.Errorf("Expected getter value 'getter_value', got '%s'", val)
			}
		})

		// Test that environment variable takes precedence over default
		t.Run("EnvVarOverDefault", func(t *testing.T) {
			opt := base.NewConfigOption("default").
				WithEnvVar("TEST_ENV_VAR")

			os.Setenv("TEST_ENV_VAR", "env_value")
			defer os.Unsetenv("TEST_ENV_VAR")

			val, err := opt.GetValue()
			if err != nil {
				t.Fatalf("GetValue failed: %v", err)
			}
			if val != "env_value" {
				t.Errorf("Expected env value 'env_value', got '%s'", val)
			}
		})

		// Test that default value is used when no other sources are available
		t.Run("DefaultValue", func(t *testing.T) {
			opt := base.NewConfigOption("default")

			val, err := opt.GetValue()
			if err != nil {
				t.Fatalf("GetValue failed: %v", err)
			}
			if val != "default" {
				t.Errorf("Expected default value 'default', got '%s'", val)
			}
		})
	})

	// Test GetInfo deadlock prevention
	t.Run("GetInfoDeadlock", func(t *testing.T) {
		// Test with a slow getter that could cause deadlock
		t.Run("SlowGetter", func(t *testing.T) {
			opt := base.NewConfigOption("default").
				WithGetter(func() string {
					time.Sleep(100 * time.Millisecond)
					return "slow_value"
				})

			// Start multiple goroutines to call GetInfo concurrently
			var wg sync.WaitGroup
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					info := opt.GetInfo()
					if info["value"] != "slow_value" {
						t.Errorf("Expected value 'slow_value', got '%v'", info["value"])
					}
				}()
			}

			// Wait for all goroutines to complete
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			// Wait with timeout to detect deadlocks
			select {
			case <-done:
				// All goroutines completed successfully
			case <-time.After(2 * time.Second):
				t.Fatal("Potential deadlock detected in GetInfo")
			}
		})

		// Test with a getter that panics
		t.Run("PanickingGetter", func(t *testing.T) {
			opt := base.NewConfigOption("default").
				WithGetter(func() string {
					panic("getter panic")
				})

			// Call GetInfo and ensure it doesn't deadlock
			func() {
				defer func() {
					if r := recover(); r == nil {
						t.Error("Expected panic from getter")
					}
				}()
				opt.GetInfo()
			}()
		})

		// Test with concurrent access
		t.Run("ConcurrentAccess", func(t *testing.T) {
			opt := base.NewConfigOption("default")

			var wg sync.WaitGroup
			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					info := opt.GetInfo()
					if info["value"] != "default" {
						t.Errorf("Expected value 'default', got '%v'", info["value"])
					}
				}()
			}

			// Wait for all goroutines to complete
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			// Wait with timeout to detect deadlocks
			select {
			case <-done:
				// All goroutines completed successfully
			case <-time.After(2 * time.Second):
				t.Fatal("Potential deadlock detected in GetInfo")
			}
		})
	})

	// Test environment variable configuration
	t.Run("EnvironmentVariable", func(t *testing.T) {
		// Set up environment variables for testing
		os.Setenv("TEST_ENV_STRING", "env_value")
		os.Setenv("TEST_ENV_INT", "42")
		os.Setenv("TEST_ENV_BOOL", "true")
		os.Setenv("TEST_ENV_FLOAT", "3.14")
		os.Setenv("TEST_ENV_LIST", "[1, 2, 3]")
		os.Setenv("TEST_ENV_MAP", `{"key": "value"}`)
		defer func() {
			os.Unsetenv("TEST_ENV_STRING")
			os.Unsetenv("TEST_ENV_INT")
			os.Unsetenv("TEST_ENV_BOOL")
			os.Unsetenv("TEST_ENV_FLOAT")
			os.Unsetenv("TEST_ENV_LIST")
			os.Unsetenv("TEST_ENV_MAP")
		}()

		// Test basic environment variable configuration for string
		t.Run("StringEnvVar", func(t *testing.T) {
			opt := base.NewConfigOption("default").WithEnvVar("TEST_ENV_STRING")

			val, err := opt.GetValue()
			if err != nil {
				t.Fatalf("GetValue failed: %v", err)
			}
			if val != "env_value" {
				t.Errorf("Expected environment value 'env_value', got '%s'", val)
			}

			// HasEnvVar should return true
			if !opt.HasEnvVar() {
				t.Error("HasEnvVar should return true when environment variable is set")
			}
		})

		// Test environment variable with int type
		t.Run("IntEnvVar", func(t *testing.T) {
			opt := base.NewConfigOption(0).WithEnvVar("TEST_ENV_INT")

			val, err := opt.GetValue()
			if err != nil {
				t.Fatalf("GetValue failed: %v", err)
			}
			if val != 42 {
				t.Errorf("Expected environment value 42, got %d", val)
			}
		})

		// Test environment variable with boolean type
		t.Run("BoolEnvVar", func(t *testing.T) {
			opt := base.NewConfigOption(false).WithEnvVar("TEST_ENV_BOOL")

			val, err := opt.GetValue()
			if err != nil {
				t.Fatalf("GetValue failed: %v", err)
			}
			if val != true {
				t.Errorf("Expected environment value true, got %v", val)
			}
		})

		// Test environment variable with slice
		t.Run("ListEnvVar", func(t *testing.T) {
			opt := base.NewConfigOption([]int{}).WithEnvVar("TEST_ENV_LIST")

			val, err := opt.GetValue()
			if err != nil {
				t.Fatalf("GetValue failed: %v", err)
			}
			expected := []int{1, 2, 3}
			if len(val) != len(expected) {
				t.Fatalf("Expected environment value %v, got %v", expected, val)
			}
			for i, v := range val {
				if v != expected[i] {
					t.Errorf("Expected %d at index %d, got %d", expected[i], i, v)
				}
			}
		})

		// Test priority order (explicit value > env var)
		t.Run("PriorityOverEnvVar", func(t *testing.T) {
			opt := base.NewConfigOption("default").
				WithEnvVar("TEST_ENV_STRING").
				WithValue("explicit_value")

			val, err := opt.GetValue()
			if err != nil {
				t.Fatalf("GetValue failed: %v", err)
			}
			if val != "explicit_value" {
				t.Errorf("Expected explicit value 'explicit_value' to take precedence, got '%s'", val)
			}
		})

		// Test priority order (getter > env var)
		t.Run("GetterPriorityOverEnvVar", func(t *testing.T) {
			opt := base.NewConfigOption("default").
				WithEnvVar("TEST_ENV_STRING").
				WithGetter(func() string {
					return "getter_value"
				})

			val, err := opt.GetValue()
			if err != nil {
				t.Fatalf("GetValue failed: %v", err)
			}
			if val != "getter_value" {
				t.Errorf("Expected getter value 'getter_value' to take precedence over env var, got '%s'", val)
			}
		})

		// Test env var > default
		t.Run("EnvVarPriorityOverDefault", func(t *testing.T) {
			opt := base.NewConfigOption("default").WithEnvVar("TEST_ENV_STRING")

			val, err := opt.GetValue()
			if err != nil {
				t.Fatalf("GetValue failed: %v", err)
			}
			if val != "env_value" {
				t.Errorf("Expected env var 'env_value' to take precedence over default, got '%s'", val)
			}
		})

		// Test invalid env var format for the type
		t.Run("InvalidEnvVarFormat", func(t *testing.T) {
			// Set an env var with invalid format for int
			os.Setenv("TEST_ENV_INVALID", "not_an_int")
			defer os.Unsetenv("TEST_ENV_INVALID")

			opt := base.NewConfigOption(10).WithEnvVar("TEST_ENV_INVALID")

			val, err := opt.GetValue()
			if err != nil {
				t.Fatalf("GetValue failed: %v", err)
			}

			// Should fall back to default when env var format is invalid
			if val != 10 {
				t.Errorf("Expected fallback to default value 10, got %d", val)
			}
		})

		// Test non-existent env var
		t.Run("NonExistentEnvVar", func(t *testing.T) {
			opt := base.NewConfigOption("default").WithEnvVar("NON_EXISTENT_ENV_VAR")

			val, err := opt.GetValue()
			if err != nil {
				t.Fatalf("GetValue failed: %v", err)
			}

			// Should fall back to default when env var doesn't exist
			if val != "default" {
				t.Errorf("Expected fallback to default value 'default', got '%s'", val)
			}
		})
	})
}
