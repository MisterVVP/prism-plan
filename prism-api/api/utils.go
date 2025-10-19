package api

import (
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

var (
	lastTimestamp int64
)

func nextTimestamp() int64 {
	for {
		now := time.Now().UnixNano()
		last := atomic.LoadInt64(&lastTimestamp)
		if now <= last {
			now = last + 1
		}
		if atomic.CompareAndSwapInt64(&lastTimestamp, last, now) {
			return now
		}
	}
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
func envDur(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}
