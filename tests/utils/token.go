package testutil

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// TestToken returns a signed JWT suitable for test mode authentication.
func TestToken(userID string) (string, error) {
	secret := os.Getenv("TEST_JWT_SECRET")
	if secret == "" {
		return "", errors.New("TEST_JWT_SECRET must be set")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	return token.SignedString([]byte(secret))
}
