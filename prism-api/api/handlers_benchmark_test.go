package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

func BenchmarkPostCommands(b *testing.B) {
	payloads := []struct {
		name     string
		commands int
	}{
		{name: "Single", commands: 1},
		{name: "Batch4", commands: 4},
	}

	for _, payload := range payloads {
		payload := payload

		b.Run("Async/"+payload.name, func(b *testing.B) {
			resetCommandSenderForTests()
			defer resetCommandSenderForTests()

			store := noopStore{}
			initCommandSender(store, log.New())
			handler := postCommands(store, mockAuth{})
			body := buildCommandPayload(payload.commands)

			runPostCommandsBenchmark(b, handler, body)
		})

		b.Run("Inline/"+payload.name, func(b *testing.B) {
			resetCommandSenderForTests()
			defer resetCommandSenderForTests()

			store := noopStore{}
			handler := postCommands(store, mockAuth{})
			body := buildCommandPayload(payload.commands)

			runPostCommandsBenchmark(b, handler, body)
		})
	}
}

func runPostCommandsBenchmark(b *testing.B, handler echo.HandlerFunc, payload []byte) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		e := echo.New()
		for pb.Next() {
			req := httptest.NewRequest(http.MethodPost, "/api/commands", bytes.NewReader(payload))
			req.Header.Set(echo.HeaderAuthorization, "Bearer token")
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if err := handler(c); err != nil {
				b.Fatalf("handler returned error: %v", err)
			}
			if rec.Code != http.StatusAccepted {
				b.Fatalf("unexpected status code: %d", rec.Code)
			}
		}
	})
}

func buildCommandPayload(n int) []byte {
	if n <= 0 {
		return []byte("[]")
	}

	const template = `{"entityType":"task","type":"create-task"}`
	bufSize := len(template)*n + (n - 1) + 2
	buf := make([]byte, 0, bufSize)
	buf = append(buf, '[')
	for i := 0; i < n; i++ {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, template...)
	}
	buf = append(buf, ']')
	return buf
}
