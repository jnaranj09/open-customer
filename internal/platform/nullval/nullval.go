// Package nullval converts between nullable.Nullable (a PATCH or response field that can be
// absent, null or a value) and the *T the modules store.
package nullval

import "github.com/oapi-codegen/nullable"

// Ptr is the value of a specified nullable, or nil for an explicit null or an unspecified one.
// Callers check IsSpecified first when absent and null mean different things.
func Ptr[T any](n nullable.Nullable[T]) *T {
	v, err := n.Get()
	if err != nil {
		return nil
	}
	return &v
}

// Of is a nullable response field: a nil p is an explicit null, so the JSON always has the key.
func Of[T any](p *T) nullable.Nullable[T] {
	if p == nil {
		return nullable.NewNullNullable[T]()
	}
	return nullable.NewNullableWithValue(*p)
}
