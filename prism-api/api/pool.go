package api

import (
	"context"
	"os"
	"prism-api/domain"
	"runtime"
	"strconv"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type enqueueJob struct {
	userID string
	cmds   []domain.Command
}

const (
	minEnqueueWorkers     = 32
	maxEnqueueWorkers     = 192
	workersPerCPU         = 32
	workersPerQueueUnit   = 4
	bufferPerWorker       = 128
	minEnqueueBuffer      = minEnqueueWorkers * bufferPerWorker
	maxEnqueueBuffer      = 64 * 1024
	defaultHandoffTimeout = 50 * time.Millisecond
	defaultQueueParallel  = 8
)

var (
	once           sync.Once
	jobs           chan enqueueJob
	workerCount    int
	jobBuf         int
	enqueueTimeout time.Duration
	handoffTimeout time.Duration
	bg             = context.Background()
	globalStore    Storage
	globalLog      *log.Logger
	workerWG       sync.WaitGroup
	timerPool      = sync.Pool{
		New: func() any {
			t := time.NewTimer(time.Hour)
			if !t.Stop() {
				<-t.C
			}
			return t
		},
	}
)

// shutdownCommandSender stops worker goroutines and clears shared state. It is intended for tests.
func shutdownCommandSender() {
	if jobs != nil {
		close(jobs)
		jobs = nil
	}

	workerWG.Wait()

	globalStore = nil
	globalLog = nil
	workerCount = 0
	jobBuf = 0
	enqueueTimeout = 0
	handoffTimeout = 0
	once = sync.Once{}
	workerWG = sync.WaitGroup{}
}

func initCommandSender(store Storage, log *log.Logger) {
	once.Do(func() {
		globalStore = store
		if log == nil {
			panic("Logger is not initialized")
		}
		globalLog = log

		queueParallel := deriveQueueConcurrency()
		autoWorkers, autoBuf := computeWorkerDefaults(queueParallel, runtime.NumCPU())

		workerCount = lookupPositiveInt("ENQUEUE_WORKERS", autoWorkers)
		jobBuf = lookupPositiveInt("ENQUEUE_BUFFER", autoBuf)
		if jobBuf < workerCount {
			jobBuf = workerCount
		}

		enqueueTimeout = envDur("ENQUEUE_TIMEOUT", 60*time.Second)
		handoffTimeout = envDur("ENQUEUE_HANDOFF_TIMEOUT", defaultHandoffTimeout)

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

	timer := acquireTimer(handoffTimeout)
	ok, closed := sendWithTimer(jobs, job, timer.C)
	releaseTimer(timer)
	if closed {
		return false
	}
	return ok
}

func deriveQueueConcurrency() int {
	if v := os.Getenv("COMMAND_QUEUE_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultQueueParallel
}

func computeWorkerDefaults(queueConcurrency, numCPU int) (workers int, buffer int) {
	if queueConcurrency <= 0 {
		queueConcurrency = defaultQueueParallel
	}
	if numCPU <= 0 {
		numCPU = 1
	}

	queueScaled := queueConcurrency * workersPerQueueUnit
	cpuScaled := numCPU * workersPerCPU
	workers = queueScaled
	if cpuScaled > workers {
		workers = cpuScaled
	}
	if workers < minEnqueueWorkers {
		workers = minEnqueueWorkers
	} else if workers > maxEnqueueWorkers {
		workers = maxEnqueueWorkers
	}

	buffer = workers * bufferPerWorker
	if buffer < minEnqueueBuffer {
		buffer = minEnqueueBuffer
	} else if buffer > maxEnqueueBuffer {
		buffer = maxEnqueueBuffer
	}

	return workers, buffer
}

func lookupPositiveInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
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

func acquireTimer(d time.Duration) *time.Timer {
	t := timerPool.Get().(*time.Timer)
	t.Reset(d)
	return t
}

func releaseTimer(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	timerPool.Put(t)
}
