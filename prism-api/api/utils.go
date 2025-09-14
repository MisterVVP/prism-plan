package api

import (
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
