package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
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
	e.POST("/api/commands", postCommands(store, auth, deduper, log))
	e.GET("/healthz", healthz(store))
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
		token, tokenErr := bearerTokenFromHeader(c.Request().Header)
		if tokenErr != nil {
			metrics.ObserveAuth(time.Since(authStart))
			metrics.SetErrorStage("auth")
			err = c.String(http.StatusUnauthorized, tokenErr.Error())
			return err
		}
		userID, authErr := auth.UserIDFromBearer(token)
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
		token, tokenErr := bearerTokenFromHeader(c.Request().Header)
		if tokenErr != nil {
			return c.String(http.StatusUnauthorized, tokenErr.Error())
		}
		userID, err := auth.UserIDFromBearer(token)
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

func postCommands(store Storage, auth Authenticator, deduper Deduper, log *log.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()

		initCommandSender(store, deduper, log)

		token, tokenErr := bearerTokenFromHeader(c.Request().Header)
		if tokenErr != nil {
			return c.String(http.StatusUnauthorized, tokenErr.Error())
		}
		userID, err := auth.UserIDFromBearer(token)
		if err != nil {
			return c.String(http.StatusUnauthorized, err.Error())
		}

		lr := io.LimitReader(c.Request().Body, postCommandMaxSize)
		dec := json.NewDecoder(lr)
		dec.DisallowUnknownFields()

		raw := make([]postCommandRequest, 0, 4)
		if err := dec.Decode(&raw); err != nil {
			return c.String(http.StatusBadRequest, "invalid body")
		}

		cmds := make([]domain.Command, len(raw))
		for i := range raw {
			cmds[i] = domain.Command{
				IdempotencyKey: raw[i].IdempotencyKey,
				EntityType:     raw[i].EntityType,
				Type:           raw[i].Type,
				Data:           raw[i].Data,
			}
		}

		keys := make([]string, len(cmds))
		filtered := cmds[:0]
		added := make([]string, 0, len(cmds))
		addedIdxs := make([]int, 0, len(cmds))

		for i := range cmds {
			if cmds[i].IdempotencyKey == "" {
				cmds[i].IdempotencyKey = uuid.NewString()
			}
			cmds[i].ID = cmds[i].IdempotencyKey
			keys[i] = cmds[i].IdempotencyKey
		}

		var addedMask []bool
		var addErr error
		failedIndex := -1
		usedBatch := false
		if batch, ok := deduper.(batchDeduper); ok && len(cmds) > 0 {
			addedMask, addErr = batch.AddMany(ctx, userID, keys)
			usedBatch = true
			for i, addedNow := range addedMask {
				if addedNow {
					addedIdxs = append(addedIdxs, i)
				}
			}
		} else if len(cmds) > 0 {
			for i := range cmds {
				var addedNow bool
				addedNow, addErr = deduper.Add(ctx, userID, cmds[i].IdempotencyKey)
				if addErr != nil {
					failedIndex = i
					break
				}
				if addedNow {
					addedIdxs = append(addedIdxs, i)
				}
			}
		}

		if addErr != nil {
			rollback := make(map[string]struct{}, len(keys))
			for _, idx := range addedIdxs {
				if idx >= 0 && idx < len(keys) {
					rollback[keys[idx]] = struct{}{}
				}
			}
			if !usedBatch && failedIndex >= 0 && failedIndex < len(keys) {
				rollback[keys[failedIndex]] = struct{}{}
			}
			if usedBatch {
				var idxErr interface{ RollbackIndexes() []int }
				if errors.As(addErr, &idxErr) {
					for _, idx := range idxErr.RollbackIndexes() {
						if idx >= 0 && idx < len(keys) {
							rollback[keys[idx]] = struct{}{}
						}
					}
				} else if len(rollback) == 0 || len(addedMask) != len(cmds) {
					for _, key := range keys {
						rollback[key] = struct{}{}
					}
				}
			} else if len(rollback) == 0 {
				for _, key := range keys {
					rollback[key] = struct{}{}
				}
			}
			for key := range rollback {
				if err := deduper.Remove(ctx, userID, key); err != nil {
					c.Logger().Errorf("dedupe rollback failed, err: %v, key: %s", err, key)
				}
			}
			return c.String(http.StatusInternalServerError, addErr.Error())
		}

		if usedBatch && len(addedMask) != len(cmds) {
			for _, key := range keys {
				if err := deduper.Remove(ctx, userID, key); err != nil {
					c.Logger().Errorf("dedupe rollback failed, err: %v, key: %s", err, key)
				}
			}
			return c.String(http.StatusInternalServerError, "failed to reserve idempotency keys")
		}

		if len(addedIdxs) == 0 {
			return c.JSON(http.StatusAccepted, postCommandResponse{IdempotencyKeys: keys})
		}

		filtered = filtered[:0]
		added = added[:0]
		for _, idx := range addedIdxs {
			if idx < 0 || idx >= len(cmds) {
				continue
			}
			cmds[idx].Timestamp = nextTimestamp()
			filtered = append(filtered, cmds[idx])
			added = append(added, keys[idx])
		}

		job := enqueueJob{
			userID: userID,
			cmds:   filtered,
			added:  added,
		}

		if tryEnqueueJob(job) {
			return c.JSON(http.StatusAccepted, postCommandResponse{IdempotencyKeys: keys})
		}

		globalLog.Warn("enqueue buffer saturated; processing inline")

		enqueueCtx, cancel := context.WithTimeout(bg, enqueueTimeout)
		enqueueErr := store.EnqueueCommands(enqueueCtx, userID, job.cmds)
		cancel()

		if enqueueErr != nil {
			for _, k := range added {
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
