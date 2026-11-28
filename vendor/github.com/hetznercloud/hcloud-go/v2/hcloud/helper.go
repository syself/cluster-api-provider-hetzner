package hcloud

import "time"

// Ptr returns a pointer to p.
//
//go:fix inline
func Ptr[T any](p T) *T {
	return new(p)
}

// String returns a pointer to the passed string s.
//
// Deprecated: Use [Ptr] instead.
//
//go:fix inline
func String(s string) *string { return new(s) }

// Int returns a pointer to the passed integer i.
//
// Deprecated: Use [Ptr] instead.
//
//go:fix inline
func Int(i int) *int { return new(i) }

// Bool returns a pointer to the passed bool b.
//
// Deprecated: Use [Ptr] instead.
//
//go:fix inline
func Bool(b bool) *bool { return new(b) }

// Duration returns a pointer to the passed time.Duration d.
//
// Deprecated: Use [Ptr] instead.
//
//go:fix inline
func Duration(d time.Duration) *time.Duration { return new(d) }
