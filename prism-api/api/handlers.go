package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"prism-api/domain"
)

// Register wires up all API routes on the provided Echo instance.
func Register(e *echo.Echo, store Storage, auth Authenticator, deduper Deduper, log *log.Logger) {
	e.GET("/api/tasks", getTasks(store, auth, log))
	e.GET("/api/settings", getSettings(store, auth))
	e.POST("/api/commands", postCommands(store, auth, deduper))
	e.GET("/healthz", healthz(store))

	initCommandSender(store, deduper, log)
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

type batchDeduper interface {
	AddMany(ctx context.Context, userID string, keys []string) ([]bool, error)
}

var commandSlicePool = sync.Pool{
	New: func() any {
		return make([]domain.Command, 0, 4)
	},
}

type commandEntry struct {
	key string
	cmd domain.Command
}

var commandEntryPool = sync.Pool{
	New: func() any {
		return make([]commandEntry, 0, 4)
	},
}

func postCommands(store Storage, auth Authenticator, deduper Deduper) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()

		userID, err := auth.UserIDFromAuthHeader(c.Request().Header.Get("Authorization"))
		if err != nil {
			return c.String(http.StatusUnauthorized, err.Error())
		}

		lr := io.LimitReader(c.Request().Body, postCommandMaxSize)
		dec := json.NewDecoder(lr)
		dec.DisallowUnknownFields()

		start, err := dec.Token()
		if err != nil {
			return c.String(http.StatusBadRequest, "invalid body")
		}
		delim, ok := start.(json.Delim)
		if !ok || delim != '[' {
			return c.String(http.StatusBadRequest, "invalid body")
		}

		keys := make([]string, 0, 4)
		entries := commandEntryPool.Get().([]commandEntry)
		if entries == nil {
			entries = make([]commandEntry, 0, 4)
		}
		entries = entries[:0]
		defer func() {
			commandEntryPool.Put(entries[:0])
		}()

		var seen map[string]struct{}
		for dec.More() {
			var cmd domain.Command
			if err := dec.Decode(&cmd); err != nil {
				return c.String(http.StatusBadRequest, "invalid body")
			}
			if cmd.IdempotencyKey == "" {
				cmd.IdempotencyKey = uuid.NewString()
			}
			cmd.ID = cmd.IdempotencyKey
			keys = append(keys, cmd.IdempotencyKey)

			entry := commandEntry{key: cmd.IdempotencyKey, cmd: cmd}
			if len(entries) == 0 {
				entries = append(entries, entry)
				continue
			}
			if seen == nil {
				seen = make(map[string]struct{}, 4)
				seen[entries[0].key] = struct{}{}
			}
			if _, exists := seen[entry.key]; exists {
				continue
			}
			seen[entry.key] = struct{}{}
			entries = append(entries, entry)
		}

		if _, err := dec.Token(); err != nil {
			return c.String(http.StatusBadRequest, "invalid body")
		}

		if len(keys) == 0 {
			return c.JSON(http.StatusAccepted, postCommandResponse{IdempotencyKeys: []string{}})
		}

		if len(entries) == 1 {
			entry := entries[0]
			addedNow, addErr := deduper.Add(ctx, userID, entry.key)
			if addErr != nil {
				if addedNow {
					if rerr := deduper.Remove(ctx, userID, entry.key); rerr != nil {
						c.Logger().Errorf("dedupe rollback failed, err: %v, key: %s", rerr, entry.key)
					}
				}
				return c.String(http.StatusInternalServerError, addErr.Error())
			}
			if !addedNow {
				return c.JSON(http.StatusAccepted, postCommandResponse{IdempotencyKeys: keys})
			}

			entry.cmd.Timestamp = nextTimestamp()
			job := enqueueJob{
				userID: userID,
				cmds:   []domain.Command{entry.cmd},
				added:  append([]string(nil), entry.key),
			}

			if tryEnqueueJob(job) {
				return c.JSON(http.StatusAccepted, postCommandResponse{IdempotencyKeys: keys})
			}

			globalLog.Warn("enqueue buffer saturated; processing inline")

			enqueueCtx, cancel := context.WithTimeout(bg, enqueueTimeout)
			enqueueErr := store.EnqueueCommands(enqueueCtx, userID, job.cmds)
			cancel()

			if enqueueErr != nil {
				if rerr := deduper.Remove(ctx, userID, entry.key); rerr != nil {
					c.Logger().Errorf("dedupe rollback failed, err: %v, key: %s", rerr, entry.key)
				}
				c.Logger().Errorf("enqueue inline failed: %v", enqueueErr)
				return c.String(http.StatusInternalServerError, "failed to enqueue commands")
			}

			return c.JSON(http.StatusAccepted, postCommandResponse{IdempotencyKeys: keys})
		}

		uniqueKeys := make([]string, len(entries))
		for i := range entries {
			uniqueKeys[i] = entries[i].key
		}

		addedCmdsBuf := commandSlicePool.Get().([]domain.Command)
		if addedCmdsBuf == nil {
			addedCmdsBuf = make([]domain.Command, 0, len(uniqueKeys))
		} else {
			addedCmdsBuf = addedCmdsBuf[:0]
		}
		defer func() {
			commandSlicePool.Put(addedCmdsBuf[:0])
		}()

		addedKeys := make([]string, 0, len(uniqueKeys))

		var addErr error
		if batch, ok := deduper.(batchDeduper); ok && len(uniqueKeys) > 0 {
			addedMask, err := batch.AddMany(ctx, userID, uniqueKeys)
			if err != nil {
				rollbackKeys := collectRollbackKeys(uniqueKeys, addedMask, err)
				for _, key := range rollbackKeys {
					if rerr := deduper.Remove(ctx, userID, key); rerr != nil {
						c.Logger().Errorf("dedupe rollback failed, err: %v, key: %s", rerr, key)
					}
				}
				return c.String(http.StatusInternalServerError, err.Error())
			}
			if len(addedMask) != len(uniqueKeys) {
				for _, key := range uniqueKeys {
					if rerr := deduper.Remove(ctx, userID, key); rerr != nil {
						c.Logger().Errorf("dedupe rollback failed, err: %v, key: %s", rerr, key)
					}
				}
				return c.String(http.StatusInternalServerError, "failed to reserve idempotency keys")
			}
			for i, addedNow := range addedMask {
				if !addedNow {
					continue
				}
				entry := entries[i]
				entry.cmd.Timestamp = nextTimestamp()
				addedCmdsBuf = append(addedCmdsBuf, entry.cmd)
				addedKeys = append(addedKeys, entry.key)
			}
		} else if len(uniqueKeys) > 0 {
			for _, entry := range entries {
				var addedNow bool
				addedNow, addErr = deduper.Add(ctx, userID, entry.key)
				if addErr != nil {
					if addedNow {
						addedKeys = append(addedKeys, entry.key)
					}
					break
				}
				if addedNow {
					entry.cmd.Timestamp = nextTimestamp()
					addedCmdsBuf = append(addedCmdsBuf, entry.cmd)
					addedKeys = append(addedKeys, entry.key)
				}
			}
			if addErr != nil {
				for _, key := range addedKeys {
					if rerr := deduper.Remove(ctx, userID, key); rerr != nil {
						c.Logger().Errorf("dedupe rollback failed, err: %v, key: %s", rerr, key)
					}
				}
				return c.String(http.StatusInternalServerError, addErr.Error())
			}
		}

		if len(addedCmdsBuf) == 0 {
			return c.JSON(http.StatusAccepted, postCommandResponse{IdempotencyKeys: keys})
		}

		job := enqueueJob{
			userID: userID,
			cmds:   append([]domain.Command(nil), addedCmdsBuf...),
			added:  append([]string(nil), addedKeys...),
		}

		if tryEnqueueJob(job) {
			return c.JSON(http.StatusAccepted, postCommandResponse{IdempotencyKeys: keys})
		}

		globalLog.Warn("enqueue buffer saturated; processing inline")

		enqueueCtx, cancel := context.WithTimeout(bg, enqueueTimeout)
		enqueueErr := store.EnqueueCommands(enqueueCtx, userID, job.cmds)
		cancel()

		if enqueueErr != nil {
			for _, k := range addedKeys {
				if rerr := deduper.Remove(ctx, userID, k); rerr != nil {
					c.Logger().Errorf("dedupe rollback failed, err: %v, key: %s", rerr, k)
				}
			}
			c.Logger().Errorf("enqueue inline failed: %v", enqueueErr)
			return c.String(http.StatusInternalServerError, "failed to enqueue commands")
		}

		return c.JSON(http.StatusAccepted, postCommandResponse{IdempotencyKeys: keys})
	}
}

func collectRollbackKeys(uniqueKeys []string, addedMask []bool, addErr error) []string {
	rollback := make(map[int]struct{}, len(uniqueKeys))
	if len(addedMask) == len(uniqueKeys) {
		for i, addedNow := range addedMask {
			if addedNow {
				rollback[i] = struct{}{}
			}
		}
	}
	var idxErr interface{ RollbackIndexes() []int }
	if errors.As(addErr, &idxErr) {
		for _, idx := range idxErr.RollbackIndexes() {
			if idx >= 0 && idx < len(uniqueKeys) {
				rollback[idx] = struct{}{}
			}
		}
	}
	if len(rollback) == 0 {
		for i := range uniqueKeys {
			rollback[i] = struct{}{}
		}
	}
	keys := make([]string, 0, len(rollback))
	for idx := range rollback {
		if idx >= 0 && idx < len(uniqueKeys) {
			keys = append(keys, uniqueKeys[idx])
		}
	}
	return keys
}
