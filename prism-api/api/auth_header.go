package api

import (
	"errors"
	"net/http"
	"unsafe"

	"github.com/labstack/echo/v4"
)

var (
	errMissingAuthorization = errors.New("missing authorization header")
	errBadAuthorization     = errors.New("bad auth header")
)

var bearerPrefix = [...]byte{'B', 'e', 'a', 'r', 'e', 'r', ' '}

func bearerTokenFromHeader(header http.Header) ([]byte, error) {
	values := header.Values(echo.HeaderAuthorization)
	if len(values) == 0 {
		return nil, errMissingAuthorization
	}
	return bearerTokenFromString(values[0])
}

func bearerTokenFromString(raw string) ([]byte, error) {
	start := 0
	end := len(raw)
	for start < end && raw[start] == ' ' {
		start++
	}
	for end > start && raw[end-1] == ' ' {
		end--
	}
	if start >= end {
		return nil, errMissingAuthorization
	}
	trimmed := raw[start:end]
	tokenBytes := readOnlyBytes(trimmed)
	if len(tokenBytes) <= len(bearerPrefix) {
		return nil, errBadAuthorization
	}
	if !hasBearerPrefix(tokenBytes) {
		return nil, errBadAuthorization
	}
	tokenBytes = tokenBytes[len(bearerPrefix):]
	if countByte(tokenBytes, '.') != 2 {
		return nil, errBadAuthorization
	}
	return tokenBytes, nil
}

func hasBearerPrefix(value []byte) bool {
	if len(value) < len(bearerPrefix) {
		return false
	}
	for i := range bearerPrefix {
		if value[i] != bearerPrefix[i] {
			return false
		}
	}
	return true
}

func countByte(buf []byte, target byte) int {
	count := 0
	for _, b := range buf {
		if b == target {
			count++
		}
	}
	return count
}

func readOnlyBytes(s string) []byte {
	if s == "" {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

func readOnlyString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}
