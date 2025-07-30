package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

type Event struct {
	ID     string          `json:"id"`
	TaskID string          `json:"taskId"`
	Type   string          `json:"type"`
	Data   json.RawMessage `json:"data,omitempty"`
	Time   int64           `json:"time"`
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
	token := parts[1]
	segments := strings.Split(token, ".")
	if len(segments) < 2 {
		return "", errors.New("invalid token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(segments[1])
	if err != nil {
		return "", err
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", err
	}
	if claims.Sub == "" {
		return "", errors.New("missing sub")
	}
	return claims.Sub, nil
}
