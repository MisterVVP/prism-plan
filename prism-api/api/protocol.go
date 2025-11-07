package api

const postCommandMaxSize = 64 * 1024 // 64 KiB

// /POST /api/command response body
type postCommandResponse struct {
	IdempotencyKeys []string `json:"idempotencyKeys,omitempty"`
	Error           string   `json:"error,omitempty"`
}
