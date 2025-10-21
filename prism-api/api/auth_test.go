package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"
)

func TestBearerTokenFromHeaderSuccess(t *testing.T) {
	header := make(http.Header)
	header.Set(echo.HeaderAuthorization, "Bearer header.payload.signature")

	token, err := bearerTokenFromHeader(header)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(token) != "header.payload.signature" {
		t.Fatalf("unexpected token content: %s", string(token))
	}
}

func TestBearerTokenFromHeaderMissing(t *testing.T) {
	header := make(http.Header)
	if _, err := bearerTokenFromHeader(header); err == nil || err.Error() != "missing authorization header" {
		t.Fatalf("expected missing header error, got %v", err)
	}
}

func TestBearerTokenFromStringManyPeriods(t *testing.T) {
	header := "Bearer " + strings.Repeat(".", 1000)
	if _, err := bearerTokenFromString(header); err == nil || err.Error() != "bad auth header" {
		t.Fatalf("expected bad auth header error, got %v", err)
	}
}

func TestUserIDFromBearerHS256(t *testing.T) {
	secret := []byte("test-secret")
	claims := jwt.MapClaims{
		"sub": "user-123",
		"aud": "api://aud",
		"iss": "https://issuer/",
		"exp": time.Now().Add(5 * time.Minute).Unix(),
		"nbf": time.Now().Add(-time.Minute).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	auth := &Auth{
		Audience:   "api://aud",
		Issuer:     "https://issuer/",
		TestMode:   true,
		TestSecret: secret,
		parser:     jwt.NewParser(jwt.WithValidMethods([]string{"HS256"})),
	}

	userID, err := auth.UserIDFromBearer([]byte(signed))
	if err != nil {
		t.Fatalf("unexpected error verifying token: %v", err)
	}
	if userID != "user-123" {
		t.Fatalf("unexpected user id: %s", userID)
	}
}
