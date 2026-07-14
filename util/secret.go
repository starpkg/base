package util

import "crypto/subtle"

// SecretEqual reports whether a and b are equal, in constant time relative to
// their contents — use it to compare a caller-supplied token, password, or
// signature against a host secret so the comparison does not leak the secret
// via timing. A plain a == b (or bytes.Equal) short-circuits on the first
// differing byte and is timing-attackable. (Length inequality is not hidden,
// which matches crypto/subtle and is the standard trade-off.)
func SecretEqual(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// SecretEqualString is SecretEqual for strings.
func SecretEqualString(a, b string) bool {
	return SecretEqual([]byte(a), []byte(b))
}
