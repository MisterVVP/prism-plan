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
	return nextTimestampRange(1)
}

func nextTimestampRange(n int) int64 {
	if n <= 0 {
		return 0
	}

	// Reserve a contiguous, monotonically increasing sequence of timestamps with a
	// single atomic update. This avoids calling time.Now for every element in the
	// batch and keeps timestamp assignment contention low under high concurrency.
	for {
		now := time.Now().UnixNano()
		last := atomic.LoadInt64(&lastTimestamp)

		start := now
		if now <= last {
			start = last + 1
		}

		end := start + int64(n-1)
		if atomic.CompareAndSwapInt64(&lastTimestamp, last, end) {
			return start
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
func envDur(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}
