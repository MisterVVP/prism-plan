package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
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
}

type tasksResponse struct {
	Tasks         []domain.Task `json:"tasks"`
	NextPageToken string        `json:"nextPageToken,omitempty"`
}

func getTasks(store Storage, auth Authenticator, logger *log.Logger) echo.HandlerFunc {
	return func(c echo.Context) (err error) {
		ctx := c.Request().Context()
		metrics := newTaskRequestMetrics(logger)
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

		fetchStart := time.Now()
		tasks, nextToken, fetchErr := store.FetchTasks(ctx, userID, pageToken)
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

func postCommands(store Storage, auth Authenticator, deduper Deduper, log *log.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()

		initCommandSender(store, deduper, log)

		userID, err := auth.UserIDFromAuthHeader(c.Request().Header.Get("Authorization"))
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
		filtered := make([]domain.Command, 0, len(cmds))
		added := make([]string, 0, len(cmds))

		for i := range cmds {
			if cmds[i].IdempotencyKey == "" {
				cmds[i].IdempotencyKey = uuid.NewString()
			}
			cmds[i].ID = cmds[i].IdempotencyKey
			keys[i] = cmds[i].IdempotencyKey

			addedNow, err := deduper.Add(ctx, userID, cmds[i].IdempotencyKey)
			if err != nil {
				for _, key := range added {
					_ = deduper.Remove(ctx, userID, key)
				}
				_ = deduper.Remove(ctx, userID, cmds[i].IdempotencyKey)
				return c.String(http.StatusInternalServerError, err.Error())
			}
			if !addedNow {
				continue
			}
			added = append(added, cmds[i].IdempotencyKey)
			cmds[i].Timestamp = nextTimestamp()
			filtered = append(filtered, cmds[i])
		}

		if len(filtered) == 0 {
			return c.JSON(http.StatusAccepted, postCommandResponse{IdempotencyKeys: keys})
		}

		job := enqueueJob{
			userID: userID,
			cmds:   append([]domain.Command(nil), filtered...),
			added:  append([]string(nil), added...),
		}

		select {
		case jobs <- job:
			return c.JSON(http.StatusAccepted, postCommandResponse{IdempotencyKeys: keys})
		default:
			globalLog.Warn("enqueue buffer full; processing inline")

			enqueueCtx, cancel := context.WithTimeout(bg, enqueueTimeout)
			err := store.EnqueueCommands(enqueueCtx, userID, job.cmds)
			cancel()

			if err != nil {
				for _, k := range added {
					if rerr := deduper.Remove(ctx, userID, k); rerr != nil {
						c.Logger().Errorf("dedupe rollback failed, err: %v, key: %s", rerr, k)
					}
				}
				c.Logger().Errorf("enqueue inline failed: %v", err)
				return c.String(http.StatusInternalServerError, "failed to enqueue commands")
			}

			return c.JSON(http.StatusAccepted, postCommandResponse{IdempotencyKeys: keys})
		}
	}
}
