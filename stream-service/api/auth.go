package api

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc"
	"github.com/golang-jwt/jwt/v4"
)

// Auth validates incoming JWT tokens.
type Auth struct {
	JWKS       *keyfunc.JWKS
	Audience   string
	Issuer     string
	TestMode   bool
	TestSecret []byte
}

// NewAuth creates a new Auth instance.
func NewAuth(jwks *keyfunc.JWKS, audience, issuer string) *Auth {
	a := &Auth{JWKS: jwks, Audience: audience, Issuer: issuer}
	if os.Getenv("AUTH0_TEST_MODE") == "1" {
		secret := os.Getenv("TEST_JWT_SECRET")
		if secret == "" {
			panic("TEST_JWT_SECRET must be set when AUTH0_TEST_MODE=1")
		}
		a.TestMode = true
		a.TestSecret = []byte(secret)
	}
	return a
}

// UserIDFromAuthHeader extracts the user identifier from the Authorization header.
func (a *Auth) UserIDFromAuthHeader(h string) (string, error) {
	if h == "" {
		return "", errors.New("missing authorization header")
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 {
		return "", errors.New("bad auth header")
	}

	tokenStr := parts[1]
	if strings.Count(tokenStr, ".") != 2 {
		return "", errors.New("bad auth header")
	}

	if a.TestMode {
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("invalid signing method")
			}
			return a.TestSecret, nil
		})
		if err != nil {
			return "", err
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return "", errors.New("invalid claims")
		}
		sub, ok := claims["sub"].(string)
		if !ok || sub == "" {
			return "", errors.New("missing sub")
		}
		return sub, nil
	}

	parser := jwt.NewParser(jwt.WithValidMethods([]string{"RS256"}))
	token, err := parser.Parse(tokenStr, a.JWKS.Keyfunc)
	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid claims")
	}

	now := time.Now().Add(time.Minute).Unix()
	if !claims.VerifyExpiresAt(now, true) {
		return "", errors.New("token expired")
	}
	if !claims.VerifyNotBefore(now, false) {
		return "", errors.New("token not valid yet")
	}
	if !claims.VerifyAudience(a.Audience, false) {
		return "", errors.New("invalid audience")
	}
	if !claims.VerifyIssuer(a.Issuer, false) {
		return "", errors.New("invalid issuer")
	}
	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", errors.New("missing sub")
	}

	return sub, nil
}
