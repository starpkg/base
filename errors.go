// Package base provides a generic configuration module with inline support for Starlark integration.
// This package defines types and helpers for creating configurable options that can be integrated with a Starlark runtime.
package base

import (
	"errors"
)

// Common errors returned by the configuration module.
var (
	// ErrConfigNotSet is the error when the configuration is not set.
	ErrConfigNotSet = errors.New("config not set")
	// ErrConfigRequired is the error when a required configuration is not set.
	ErrConfigRequired = errors.New("required config not set")
	// ErrConfigInvalidValue is the error when a configuration value is invalid.
	ErrConfigInvalidValue = errors.New("invalid config value")
	// ErrModuleAlreadyInitialized is the error when trying to modify a module after it's initialized.
	ErrModuleAlreadyInitialized = errors.New("module already initialized")
	// ErrSecretConfigNotRetrievable is the error when attempting to retrieve a secret configuration value.
	ErrSecretConfigNotRetrievable = errors.New("secret configuration is not retrievable")
	// ErrModuleNotInitialized is the error when trying to access a module that's not initialized.
	ErrModuleNotInitialized = errors.New("module not initialized")
	// ErrConfigOptionNotFound is the error when a configuration option is not found.
	ErrConfigOptionNotFound = errors.New("config option not found")
	// ErrConfigGetterPanic is the error when a config getter function panics.
	ErrConfigGetterPanic = errors.New("config getter panicked")
)
