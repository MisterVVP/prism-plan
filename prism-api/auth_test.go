package main

import (
	"strings"
	"testing"
)

func TestUserIDFromAuthHeaderManyPeriods(t *testing.T) {
	header := "Bearer " + strings.Repeat(".", 10000)
	if _, err := userIDFromAuthHeader(header); err == nil || err.Error() != "bad auth header" {
		t.Fatalf("expected bad auth header error, got %v", err)
	}
}
