package main

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func userIDFromAuthHeader(h string) (string, error) {
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

	parser := jwt.NewParser(jwt.WithValidMethods([]string{"RS256"}), jwt.WithoutClaimsValidation())
	token, err := parser.Parse(tokenStr, jwtJWKS.Keyfunc)
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
	if !claims.VerifyAudience(jwtAudience, false) {
		return "", errors.New("invalid audience")
	}
	if !claims.VerifyIssuer(jwtIssuer, false) {
		return "", errors.New("invalid issuer")
	}
	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", errors.New("missing sub")
	}

	return sub, nil
}
