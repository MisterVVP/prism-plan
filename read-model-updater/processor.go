package main

import (
	"context"

	"read-model-updater/domain"

	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

type eventApplier interface {
	Apply(ctx context.Context, ev domain.Event) error
}

func processEvent(ctx context.Context, h eventApplier, cache cacheRefresher, rc *redis.Client, taskChannel, settingsChannel string, ev domain.Event, payload string) error {
	if err := h.Apply(ctx, ev); err != nil {
		return err
	}
	if cache != nil {
		switch ev.EntityType {
		case "task":
			cache.RefreshTasks(ctx, ev.UserID, ev.EntityID, ev.Timestamp)
		case "user-settings":
			cache.RefreshSettings(ctx, ev.UserID, ev.Timestamp)
		}
	}
	channel := taskChannel
	if ev.EntityType == "user-settings" {
		channel = settingsChannel
	}
	if err := rc.Publish(ctx, channel, payload).Err(); err != nil {
		log.Errorf("Unable to publish updates for %s to %s", ev.EntityType, channel)
	}
	return nil
}
