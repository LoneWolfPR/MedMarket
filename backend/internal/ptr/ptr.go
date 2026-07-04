// Package ptr provides small, mechanical helpers for working with pointers.
//
// These are deliberately convention-free: To/Deref do not special-case zero
// values. Policies like "an empty string means absent" belong at the boundary
// that owns that convention (e.g. DTO mapping in an adapter), not here.
package ptr

// To returns a pointer to v. Useful for populating optional (pointer) fields
// from a plain value.
func To[T any](v T) *T {
	return &v
}

// Deref returns the value p points to, or the zero value of T if p is nil.
func Deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}
