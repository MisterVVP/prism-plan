package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	log "github.com/sirupsen/logrus"

	"github.com/labstack/echo/v4"

	"prism-api/domain"
)

// Register wires up all API routes on the provided Echo instance.
func Register(e *echo.Echo, store Storage, auth Authenticator, log *log.Logger) {
	e.GET("/api/tasks", getTasks(store, auth, log))
	e.GET("/api/settings", getSettings(store, auth))
	e.POST("/api/commands", postCommands(store, auth))
	e.GET("/healthz", healthz(store))

	initCommandSender(store, log)
}

type tasksResponse struct {
	Tasks         []domain.Task `json:"tasks"`
	NextPageToken string        `json:"nextPageToken,omitempty"`
}

func healthz(_ Storage) echo.HandlerFunc {
	return func(c echo.Context) error {
		//TODO: implement healthcheck
		return c.NoContent(http.StatusOK)
	}
}

func getTasks(store Storage, auth Authenticator, logger *log.Logger) echo.HandlerFunc {
	return func(c echo.Context) (err error) {
		ctx := c.Request().Context()
		metrics, spanCtx := newTaskRequestMetrics(ctx, logger)
		if spanCtx != nil {
			req := c.Request().WithContext(spanCtx)
			c.SetRequest(req)
			ctx = spanCtx
		}
		defer func() {
			metrics.Log(c.Response().Status, err)
		}()

		authStart := time.Now()
		userID, authErr := auth.UserIDFromAuthHeader(c.Request().Header.Get("Authorization"))
		metrics.ObserveAuth(time.Since(authStart))
		if authErr != nil {
			metrics.SetErrorStage("auth")
			err = c.String(http.StatusUnauthorized, authErr.Error())
			return err
		}
		pageToken := c.QueryParam("pageToken")
		metrics.SetPageTokenProvided(pageToken != "")

		pageSizeParam := strings.TrimSpace(c.QueryParam("pageSize"))
		pageSize := 0
		if pageSizeParam != "" {
			var parseErr error
			pageSize, parseErr = strconv.Atoi(pageSizeParam)
			if parseErr != nil || pageSize <= 0 {
				metrics.SetErrorStage("invalid_page_size")
				err = c.String(http.StatusBadRequest, "invalid page size")
				return err
			}
		}

		fetchStart := time.Now()
		tasks, nextToken, fetchErr := store.FetchTasks(ctx, userID, pageToken, pageSize)
		metrics.ObserveFetch(time.Since(fetchStart))
		if fetchErr != nil {
			var invalidTokenErr InvalidContinuationTokenError
			if errors.As(fetchErr, &invalidTokenErr) {
				metrics.SetErrorStage("invalid_page_token")
				err = c.String(http.StatusBadRequest, "invalid page token")
				return err
			}
			metrics.SetErrorStage("storage")
			c.Logger().Error(fetchErr)
			err = c.String(http.StatusInternalServerError, fetchErr.Error())
			return err
		}
		metrics.SetTasksReturned(len(tasks))
		resp := tasksResponse{Tasks: tasks}
		if nextToken != "" {
			metrics.SetHasNextPage(true)
			resp.NextPageToken = nextToken
		}
		encodeStart := time.Now()
		err = c.JSON(http.StatusOK, resp)
		metrics.ObserveEncode(time.Since(encodeStart))
		if err != nil {
			metrics.SetErrorStage("encode_response")
		}
		return err
	}
}

func getSettings(store Storage, auth Authenticator) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		userID, err := auth.UserIDFromAuthHeader(c.Request().Header.Get("Authorization"))
		if err != nil {
			return c.String(http.StatusUnauthorized, err.Error())
		}
		settings, err := store.FetchSettings(ctx, userID)
		if err != nil {
			c.Logger().Error(err)
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, settings)
	}
}

func postCommands(store Storage, auth Authenticator) echo.HandlerFunc {
	return func(c echo.Context) error {
		userID, err := auth.UserIDFromAuthHeader(c.Request().Header.Get("Authorization"))
		if err != nil {
			return c.String(http.StatusUnauthorized, err.Error())
		}

		lr := io.LimitReader(c.Request().Body, postCommandMaxSize)
		dec := sonic.ConfigStd.NewDecoder(lr)
		dec.DisallowUnknownFields()

		cmds := make([]domain.Command, 0, 4)
		if err := dec.Decode(&cmds); err != nil {
			return c.String(http.StatusBadRequest, "invalid body")
		}

		keys := finalizeCommands(cmds)

		job := enqueueJob{
			userID: userID,
			cmds:   cmds,
		}

		if tryEnqueueJob(job) {
			return respondJSON(c, http.StatusAccepted, postCommandResponse{IdempotencyKeys: keys})
		}

		if globalLog != nil {
			globalLog.Warn("enqueue buffer saturated; processing inline")
		}

		enqueueCtx := bg
		var cancel context.CancelFunc
		if enqueueTimeout > 0 {
			enqueueCtx, cancel = context.WithTimeout(bg, enqueueTimeout)
		}
		enqueueErr := store.EnqueueCommands(enqueueCtx, userID, job.cmds)
		if cancel != nil {
			cancel()
		}

		if enqueueErr != nil {
			c.Logger().Errorf("enqueue inline failed: %v", enqueueErr)
			return c.String(http.StatusInternalServerError, "failed to enqueue commands")
		}

		return respondJSON(c, http.StatusAccepted, postCommandResponse{IdempotencyKeys: keys})
	}
}

func finalizeCommands(cmds []domain.Command) []string {
	keys := make([]string, len(cmds))
	if len(cmds) == 0 {
		return keys
	}

	start := nextTimestampRange(len(cmds))
	for i := range cmds {
		ts := start + int64(i)
		keys[i] = applyCommandMetadata(&cmds[i], ts)
	}

	return keys
}

func applyCommandMetadata(cmd *domain.Command, ts int64) string {
	if cmd.IdempotencyKey == "" {
		cmd.IdempotencyKey = strconv.FormatInt(ts, 36)
	}
	cmd.ID = cmd.IdempotencyKey
	cmd.Timestamp = ts
	return cmd.IdempotencyKey
}

func respondJSON(c echo.Context, status int, payload any) error {
	data, err := sonic.Marshal(payload)
	if err != nil {
		return c.JSON(status, payload)
	}
	return c.Blob(status, echo.MIMEApplicationJSONCharsetUTF8, data)
}
