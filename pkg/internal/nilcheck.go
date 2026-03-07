// Package internal provides shared utilities for rig's internal packages.
package internal

import "reflect"

// IsNilInterface reports whether v is nil or a typed-nil interface value
// (e.g., (*T)(nil) stored in an interface). Use this to guard against the
// common Go pitfall where interface != nil is true for typed-nil pointers.
func IsNilInterface(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Ptr && rv.IsNil()
}
