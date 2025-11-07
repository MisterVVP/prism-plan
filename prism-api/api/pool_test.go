package api

import (
	"sync"
	"testing"
	"time"
)

func TestTryEnqueueJobWaitsForCapacity(t *testing.T) {
	resetCommandSenderForTests()
	t.Cleanup(resetCommandSenderForTests)

	jobs = make(chan enqueueJob, 1)
	handoffTimeout = 50 * time.Millisecond

	jobs <- enqueueJob{}

	done := make(chan bool, 1)
	go func() {
		done <- tryEnqueueJob(enqueueJob{})
	}()

	select {
	case <-done:
		t.Fatal("tryEnqueueJob returned before capacity was freed")
	case <-time.After(20 * time.Millisecond):
	}

	<-jobs

	select {
	case ok := <-done:
		if !ok {
			t.Fatal("expected successful enqueue after capacity freed")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for enqueue completion")
	}
}

func TestTryEnqueueJobTimesOut(t *testing.T) {
	resetCommandSenderForTests()
	t.Cleanup(resetCommandSenderForTests)

	jobs = make(chan enqueueJob, 1)
	handoffTimeout = 30 * time.Millisecond

	jobs <- enqueueJob{}

	ok := tryEnqueueJob(enqueueJob{})
	if ok {
		t.Fatal("expected enqueue to fail when timeout elapsed")
	}

	select {
	case <-jobs:
	default:
		t.Fatal("expected channel to remain full after timeout")
	}
}

func TestTryEnqueueJobReturnsFalseWhenClosed(t *testing.T) {
	resetCommandSenderForTests()
	t.Cleanup(resetCommandSenderForTests)
	t.Cleanup(func() { jobs = nil })

	jobs = make(chan enqueueJob)
	close(jobs)

	if tryEnqueueJob(enqueueJob{}) {
		t.Fatal("expected enqueue to fail when channel is closed")
	}
}

func TestTryEnqueueJobNoWaitWhenZeroTimeout(t *testing.T) {
	resetCommandSenderForTests()
	t.Cleanup(resetCommandSenderForTests)

	jobs = make(chan enqueueJob, 1)
	handoffTimeout = 0

	jobs <- enqueueJob{}

	if tryEnqueueJob(enqueueJob{}) {
		t.Fatal("expected enqueue to fail when buffer full and no timeout")
	}

	<-jobs

	if !tryEnqueueJob(enqueueJob{}) {
		t.Fatal("expected enqueue to succeed when buffer has capacity")
	}
}

func TestTryEnqueueJobConcurrentWriters(t *testing.T) {
	resetCommandSenderForTests()
	t.Cleanup(resetCommandSenderForTests)

	jobs = make(chan enqueueJob, 2)
	handoffTimeout = 100 * time.Millisecond

	jobs <- enqueueJob{}
	jobs <- enqueueJob{}

	var wg sync.WaitGroup
	wg.Add(2)
	results := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			results <- tryEnqueueJob(enqueueJob{})
		}()
	}

	time.Sleep(20 * time.Millisecond)

	<-jobs
	<-jobs

	wg.Wait()
	close(results)

	successCount := 0
	for r := range results {
		if r {
			successCount++
		}
	}

	if successCount != 2 {
		t.Fatalf("expected both enqueues to succeed after capacity freed, got %d", successCount)
	}
}
