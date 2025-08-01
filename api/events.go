package main

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/golang-jwt/jwt/v4"
)

type Event struct {
	ID         string          `json:"id"`
	EntityID   string          `json:"entityId"`
	EntityType string          `json:"entityType"`
	Type       string          `json:"type"`
	Data       json.RawMessage `json:"data,omitempty"`
	Time       int64           `json:"time"`
}

type eventEntity struct {
	aztables.Entity
	Data string `json:"Data"`
}

func userIDFromAuthHeader(h string) (string, error) {
	if h == "" {
		return "", errors.New("missing authorization header")
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 {
		return "", errors.New("bad auth header")
	}
	token, err := jwt.Parse(parts[1], jwtJWKS.Keyfunc)
	if err != nil {
		return "", err
	}
	if !token.Valid {
		return "", errors.New("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid claims")
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
