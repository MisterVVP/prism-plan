package api

import (
	"context"
	"os"
	"prism-api/domain"
	"strconv"
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
	bg             = context.Background()
	globalStore    Storage
	globalDeduper  Deduper
	globalLog      *log.Logger
)

// Call lazily from handler (safe for Azure Functions cold starts)
func initCommandSender(store Storage, deduper Deduper, log *log.Logger) {
	once.Do(func() {
		globalStore = store
		globalDeduper = deduper
		if log == nil {
			panic("Logger is not initialized")
		}
		globalLog = log

		workerCount = envInt("ENQUEUE_WORKERS", 16)
		jobBuf = envInt("ENQUEUE_BUFFER", 1024)
		enqueueTimeout = envDur("ENQUEUE_TIMEOUT", 60*time.Second)

		jobs = make(chan enqueueJob, jobBuf)
		for i := 0; i < workerCount; i++ {
			go worker(i)
		}
		globalLog.Infof("command sender started, workers: %d, buffer: %d, timeout: %v", workerCount, jobBuf, enqueueTimeout)
	})
}

func worker(id int) {
	for j := range jobs {
		ctx, cancel := context.WithTimeout(bg, enqueueTimeout)
		err := globalStore.EnqueueCommands(ctx, j.userID, j.cmds)
		cancel()

		if err != nil {
			// Best-effort rollback of dedupe entries we just marked as added
			for _, k := range j.added {
				if rerr := globalDeduper.Remove(bg, j.userID, k); rerr != nil {
					globalLog.Errorf("dedupe rollback failed, err : %v, key: %s, user: %s", err, k, j.userID)
				}
			}
			globalLog.Errorf("enqueue failed, err: %v, user: %s, count: %d, worker: %d", err, j.userID, len(j.cmds), id)
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
