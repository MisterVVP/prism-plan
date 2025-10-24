package api

import "github.com/bytedance/sonic"

const postCommandMaxSize = 64 * 1024 // 64 KiB

// /POST /api/command request body
type postCommandRequest struct {
	IdempotencyKey string                 `json:"idempotencyKey"`
	EntityType     string                 `json:"entityType"`
	Type           string                 `json:"type"`
	Data           sonic.NoCopyRawMessage `json:"data,omitempty"`
}

// /POST /api/command response body
type postCommandResponse struct {
	IdempotencyKeys []string `json:"idempotencyKeys,omitempty"`
	Error           string   `json:"error,omitempty"`
}
