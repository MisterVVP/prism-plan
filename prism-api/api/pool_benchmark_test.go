package api

import (
	"testing"
	"time"

	"prism-api/domain"
)

func BenchmarkTryEnqueueJob(b *testing.B) {
	job := enqueueJob{
		userID: "user",
		cmds: []domain.Command{{
			ID:         "cmd-1",
			Type:       "create-task",
			EntityType: "task",
		}},
	}

	b.Run("Buffered", func(b *testing.B) {
		resetCommandSenderForTests()
		defer resetCommandSenderForTests()

		jobs = make(chan enqueueJob, 1024)
		handoffTimeout = 0

		b.ReportAllocs()
		for b.Loop() {
			if !tryEnqueueJob(job) {
				b.Fatal("expected buffered enqueue to succeed")
			}
			select {
			case <-jobs:
			default:
				b.Fatal("expected buffered job to be queued")
			}
		}
	})

	b.Run("BufferFull", func(b *testing.B) {
		resetCommandSenderForTests()
		defer resetCommandSenderForTests()

		jobs = make(chan enqueueJob, 1)
		handoffTimeout = 0
		jobs <- job

		b.ReportAllocs()
		for b.Loop() {
			if tryEnqueueJob(job) {
				b.Fatal("expected enqueue to fail when buffer is saturated")
			}
		}
	})

	b.Run("HandoffTimeout", func(b *testing.B) {
		resetCommandSenderForTests()
		defer resetCommandSenderForTests()

		jobs = make(chan enqueueJob, 1)
		handoffTimeout = time.Nanosecond
		jobs <- job

		b.ReportAllocs()
		for b.Loop() {
			if tryEnqueueJob(job) {
				b.Fatal("expected enqueue to fail after handoff timeout")
			}
		}
	})
}
