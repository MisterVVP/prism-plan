package api

import (
	"strings"
	"testing"
)

func TestUserIDFromAuthHeaderManyPeriods(t *testing.T) {
	a := &Auth{}
	header := "Bearer " + strings.Repeat(".", 10000)
	if _, err := a.UserIDFromAuthHeader(header); err == nil || err.Error() != "bad auth header" {
		t.Fatalf("expected bad auth header error, got %v", err)
	}
}
