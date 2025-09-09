package main

import (
	"context"

	"github.com/redis/go-redis/v9"

	"read-model-updater/domain"
)

type eventApplier interface {
	Apply(ctx context.Context, ev domain.Event) error
}

func processEvent(ctx context.Context, h eventApplier, rc *redis.Client, taskChannel, settingsChannel string, ev domain.Event, payload string) error {
	if err := h.Apply(ctx, ev); err != nil {
		return err
	}
	channel := taskChannel
	if ev.EntityType == "user-settings" {
		channel = settingsChannel
	}
	return rc.Publish(ctx, channel, payload).Err()
}
