package api

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestNextTimestampRangeSequential(t *testing.T) {
	t.Cleanup(func() {
		atomic.StoreInt64(&lastTimestamp, 0)
	})
	atomic.StoreInt64(&lastTimestamp, 0)

	start := nextTimestampRange(3)
	if start == 0 {
		t.Fatal("expected non-zero start timestamp")
	}

	wantLast := start + 2
	if got := atomic.LoadInt64(&lastTimestamp); got != wantLast {
		t.Fatalf("expected lastTimestamp=%d, got %d", wantLast, got)
	}

	second := start + 1
	third := start + 2
	if second-start != 1 || third-second != 1 {
		t.Fatalf("expected sequential timestamps, got start=%d second=%d third=%d", start, second, third)
	}
}

func TestNextTimestampRangeAdvancesPastLast(t *testing.T) {
	t.Cleanup(func() {
		atomic.StoreInt64(&lastTimestamp, 0)
	})

	base := time.Now().Add(time.Second).UnixNano()
	atomic.StoreInt64(&lastTimestamp, base)

	start := nextTimestampRange(2)
	if start != base+1 {
		t.Fatalf("expected range to start at %d, got %d", base+1, start)
	}

	wantLast := base + 2
	if got := atomic.LoadInt64(&lastTimestamp); got != wantLast {
		t.Fatalf("expected lastTimestamp=%d, got %d", wantLast, got)
	}
}

func TestNextTimestampRangeZeroCount(t *testing.T) {
	t.Cleanup(func() {
		atomic.StoreInt64(&lastTimestamp, 0)
	})
	atomic.StoreInt64(&lastTimestamp, 123)

	if start := nextTimestampRange(0); start != 0 {
		t.Fatalf("expected zero start for zero count, got %d", start)
	}
	if got := atomic.LoadInt64(&lastTimestamp); got != 123 {
		t.Fatalf("expected lastTimestamp unchanged, got %d", got)
	}
}
