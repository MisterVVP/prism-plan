package api

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisDeduperAddMany(t *testing.T) {
	m, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(m.Close)

	client := redis.NewClient(&redis.Options{Addr: m.Addr()})
	t.Cleanup(func() {
		if cerr := client.Close(); cerr != nil {
			t.Logf("redis close: %v", cerr)
		}
	})

	deduper := NewRedisDeduper(client, time.Minute)
	ctx := context.Background()
	keys := []string{"k1", "k2", "k3"}

	first, err := deduper.AddMany(ctx, "user", keys)
	if err != nil {
		t.Fatalf("add many: %v", err)
	}
	if len(first) != len(keys) {
		t.Fatalf("unexpected results length: %d", len(first))
	}
	for i, added := range first {
		if !added {
			t.Fatalf("expected key %d to be added", i)
		}
	}

	second, err := deduper.AddMany(ctx, "user", keys)
	if err != nil {
		t.Fatalf("second add many: %v", err)
	}
	for i, added := range second {
		if added {
			t.Fatalf("expected key %d to be duplicate on second call", i)
		}
	}
}

func TestRedisDeduperKeyNamespacing(t *testing.T) {
	m, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(m.Close)

	client := redis.NewClient(&redis.Options{Addr: m.Addr()})
	t.Cleanup(func() {
		if cerr := client.Close(); cerr != nil {
			t.Logf("redis close: %v", cerr)
		}
	})

	deduper := NewRedisDeduper(client, time.Minute)
	ctx := context.Background()
	const (
		userID = "user"
		key    = "k1"
	)

	added, err := deduper.Add(ctx, userID, key)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !added {
		t.Fatalf("expected key to be added")
	}

	expectedKey := userID + ":" + dedupeKeyPrefix + ":" + key
	exists, err := client.Exists(ctx, expectedKey).Result()
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if exists != 1 {
		t.Fatalf("expected redis key %q to exist", expectedKey)
	}
}
