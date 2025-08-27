package assertx

import "testing"

// Equal fails if want != got.
func Equal[T comparable](t *testing.T, want, got T) {
	if want != got {
		t.Fatalf("want %v, got %v", want, got)
	}
}
