package api

import (
	"context"
	"prism-api/domain"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type enqueueJob struct {
	userID string
	cmds   []domain.Command
	added  []string // keys added to deduper (for rollback on enqueue failure)
}

var (
	once           sync.Once
	jobs           chan enqueueJob
	workerCount    int
	jobBuf         int
	enqueueTimeout time.Duration
	handoffTimeout time.Duration
	bg             = context.Background()
	globalStore    Storage
	globalDeduper  Deduper
	globalLog      *log.Logger
	workerWG       sync.WaitGroup
)

// shutdownCommandSender stops worker goroutines and clears shared state. It is intended for tests.
func shutdownCommandSender() {
	if jobs != nil {
		close(jobs)
		jobs = nil
	}

	workerWG.Wait()

	globalStore = nil
	globalDeduper = nil
	globalLog = nil
	workerCount = 0
	jobBuf = 0
	enqueueTimeout = 0
	handoffTimeout = 0
	once = sync.Once{}
	workerWG = sync.WaitGroup{}
}

func initCommandSender(store Storage, deduper Deduper, log *log.Logger) {
	once.Do(func() {
		globalStore = store
		globalDeduper = deduper
		if log == nil {
			panic("Logger is not initialized")
		}
		globalLog = log

		workerCount = envInt("ENQUEUE_WORKERS", 32)
		jobBuf = envInt("ENQUEUE_BUFFER", 4096)
		enqueueTimeout = envDur("ENQUEUE_TIMEOUT", 60*time.Second)
		handoffTimeout = envDur("ENQUEUE_HANDOFF_TIMEOUT", 15*time.Millisecond)

		jobs = make(chan enqueueJob, jobBuf)
		for i := 0; i < workerCount; i++ {
			workerWG.Add(1)
			go worker(i, jobs)
		}
		globalLog.Infof("command sender started, workers: %d, buffer: %d, timeout: %v, handoff: %v", workerCount, jobBuf, enqueueTimeout, handoffTimeout)
	})
}

func worker(id int, jobCh <-chan enqueueJob) {
	defer workerWG.Done()
	for j := range jobCh {
		ctx, cancel := context.WithTimeout(bg, enqueueTimeout)
		err := globalStore.EnqueueCommands(ctx, j.userID, j.cmds)
		cancel()

		if err != nil {
			for _, k := range j.added {
				if rerr := globalDeduper.Remove(bg, j.userID, k); rerr != nil {
					globalLog.Errorf("dedupe rollback failed, err : %v, key: %s, user: %s", rerr, k, j.userID)
				}
			}
			globalLog.Errorf("enqueue failed, err: %v, user: %s, count: %d, worker: %d", err, j.userID, len(j.cmds), id)
		}
	}
}

func tryEnqueueJob(job enqueueJob) bool {
	if jobs == nil {
		return false
	}

	if ok, closed := trySendNonBlocking(jobs, job); closed {
		return false
	} else if ok {
		return true
	}

	if handoffTimeout <= 0 {
		return false
	}

	timer := time.NewTimer(handoffTimeout)
	defer timer.Stop()

	ok, closed := sendWithTimer(jobs, job, timer.C)
	if closed {
		return false
	}
	return ok
}

func trySendNonBlocking(ch chan enqueueJob, job enqueueJob) (ok bool, closed bool) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
			closed = true
		}
	}()

	select {
	case ch <- job:
		return true, false
	default:
		return false, false
	}
}

func sendWithTimer(ch chan enqueueJob, job enqueueJob, timer <-chan time.Time) (ok bool, closed bool) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
			closed = true
		}
	}()

	select {
	case ch <- job:
		return true, false
	case <-timer:
		return false, false
	}
}
