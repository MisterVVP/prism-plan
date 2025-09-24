package api

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

// GzipRequestMiddleware decompresses gzip-encoded request bodies so handlers can
// work with plain JSON payloads. If upstream infrastructure already decoded the
// body it gracefully falls back to the raw payload.
func GzipRequestMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			if !hasGzipEncoding(req.Header.Get(echo.HeaderContentEncoding)) {
				return next(c)
			}

			compressed, err := io.ReadAll(req.Body)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
			}
			_ = req.Body.Close()

			if len(compressed) == 0 {
				resetBody(req, compressed)
				req.Header.Del(echo.HeaderContentEncoding)
				return next(c)
			}

			gz, err := gzip.NewReader(bytes.NewReader(compressed))
			if err != nil {
				// If infrastructure already removed compression, fall back to the raw body.
				if errors.Is(err, gzip.ErrHeader) {
					if len(compressed) > postCommandMaxSize {
						return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "request body too large")
					}
					resetBody(req, compressed)
					req.Header.Del(echo.HeaderContentEncoding)
					return next(c)
				}
				return echo.NewHTTPError(http.StatusBadRequest, "invalid gzip body")
			}
			decompressed, err := io.ReadAll(io.LimitReader(gz, postCommandMaxSize+1))
			_ = gz.Close()
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid gzip body")
			}

			if len(decompressed) > postCommandMaxSize {
				return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "request body too large")
			}

			resetBody(req, decompressed)
			req.Header.Del(echo.HeaderContentEncoding)

			return next(c)
		}
	}
}

func resetBody(req *http.Request, payload []byte) {
	req.Body = io.NopCloser(bytes.NewReader(payload))
	req.ContentLength = int64(len(payload))
	if len(payload) == 0 {
		req.Header.Set(echo.HeaderContentLength, "0")
		return
	}
	req.Header.Set(echo.HeaderContentLength, strconv.Itoa(len(payload)))
}

func hasGzipEncoding(header string) bool {
	if header == "" {
		return false
	}
	for _, enc := range strings.Split(header, ",") {
		if strings.EqualFold(strings.TrimSpace(enc), "gzip") {
			return true
		}
	}
	return false
}
