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
	bg             = context.Background()
	globalStore    Storage
	globalDeduper  Deduper
	globalLog      *log.Logger
	workerWG       sync.WaitGroup
)

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
			workerWG.Add(1)
			go worker(i)
		}
		globalLog.Infof("command sender started, workers: %d, buffer: %d, timeout: %v", workerCount, jobBuf, enqueueTimeout)
	})
}

func worker(id int) {
	defer workerWG.Done()
	for j := range jobs {
		ctx, cancel := context.WithTimeout(bg, enqueueTimeout)
		err := globalStore.EnqueueCommands(ctx, j.userID, j.cmds)
		cancel()

		if err != nil {
			for _, k := range j.added {
				if rerr := globalDeduper.Remove(bg, j.userID, k); rerr != nil {
					globalLog.Errorf("dedupe rollback failed, err : %v, key: %s, user: %s", err, k, j.userID)
				}
			}
			globalLog.Errorf("enqueue failed, err: %v, user: %s, count: %d, worker: %d", err, j.userID, len(j.cmds), id)
		}
	}
}
