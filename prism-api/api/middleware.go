package api

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// GzipRequestMiddleware decompresses gzip-encoded request bodies so handlers can
// work with plain JSON payloads. Requests with invalid gzip payloads are
// rejected with a 400 response.
func GzipRequestMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			if !hasGzipEncoding(req.Header.Get(echo.HeaderContentEncoding)) {
				return next(c)
			}

			body := req.Body
			gr, err := gzip.NewReader(body)
			if err != nil {
				_ = body.Close()
				return echo.NewHTTPError(http.StatusBadRequest, "invalid gzip body")
			}

			req.Body = &gzipReadCloser{Reader: gr, body: body}
			req.ContentLength = -1
			req.Header.Del(echo.HeaderContentEncoding)
			req.Header.Del(echo.HeaderContentLength)

			return next(c)
		}
	}
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

type gzipReadCloser struct {
	*gzip.Reader
	body io.Closer
}

func (g *gzipReadCloser) Close() error {
	var err error
	if g.Reader != nil {
		err = g.Reader.Close()
	}
	if g.body != nil {
		if cerr := g.body.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}
