// Package util provides small, hardened safety primitives shared across the
// starpkg modules: bounded I/O, constant-time secret comparison, float-safe
// duration parsing, and panic recovery. Each centralizes a guard that domain
// modules previously re-implemented (often subtly wrong), so a fix lives in one
// place and new modules are safe by construction.
package util
