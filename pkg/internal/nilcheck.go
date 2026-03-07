// Package internal provides shared utilities for rig's internal packages.
package internal

import "reflect"

// IsNilInterface reports whether v is nil or a typed-nil value stored in an
// interface (e.g., (*T)(nil), (map[K]V)(nil), ([]T)(nil)). Use this to guard
// against the common Go pitfall where interface != nil is true for typed-nil values.
func IsNilInterface(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return rv.IsNil()
	default:
		return false
	}
}
