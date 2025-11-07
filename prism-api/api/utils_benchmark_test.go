package api

import (
	"sync/atomic"
	"testing"
	"time"
)

var legacyLastTimestamp int64

func legacyNextTimestamp() int64 {
	for {
		now := time.Now().UnixNano()
		last := atomic.LoadInt64(&legacyLastTimestamp)
		if now <= last {
			now = last + 1
		}
		if atomic.CompareAndSwapInt64(&legacyLastTimestamp, last, now) {
			return now
		}
	}
}

func BenchmarkNextTimestamp(b *testing.B) {
	benchmarkTimestamp(b, nextTimestamp, func() {
		atomic.StoreInt64(&lastTimestamp, 0)
	})
}

func BenchmarkNextTimestampLegacy(b *testing.B) {
	benchmarkTimestamp(b, legacyNextTimestamp, func() {
		atomic.StoreInt64(&legacyLastTimestamp, 0)
	})
}

func benchmarkTimestamp(b *testing.B, generator func() int64, reset func()) {
	if reset != nil {
		reset()
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			generator()
		}
	})
}
