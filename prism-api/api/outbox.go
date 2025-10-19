package api

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"

	"prism-api/domain"
)

type enqueueJob struct {
	userID string
	cmds   []domain.Command
	added  []string
}

type outboxConfig struct {
	bufferSize      int
	workerCount     int
	batchSize       int
	flushInterval   time.Duration
	enqueueTimeout  time.Duration
	handoffTimeout  time.Duration
	retryInitial    time.Duration
	retryMax        time.Duration
	walDir          string
	walSegmentSize  int64
	walSyncEvery    int
	walSyncInterval time.Duration
}

type commandOutbox struct {
	cfg      outboxConfig
	store    Storage
	logger   *log.Logger
	wal      *wal
	workCh   chan *walRecord
	stopCh   chan struct{}
	workerWG sync.WaitGroup
	retryWG  sync.WaitGroup

	mu        sync.Mutex
	inflight  map[uint64]*walRecord
	acked     map[uint64]struct{}
	nextAck   uint64
	closing   bool
	delivered atomic.Uint64
	started   time.Time
}

var (
	globalOutbox *commandOutbox
	outboxOnce   sync.Once
)

var errOutboxSaturated = errors.New("command outbox is saturated")

func initCommandSender(store Storage, deduper Deduper, logger *log.Logger) {
	outboxOnce.Do(func() {
		if store == nil {
			panic("storage is required")
		}
		if logger == nil {
			panic("logger is required")
		}
		_ = deduper

		cfg := outboxConfig{
			bufferSize:      envInt("OUTBOX_BUFFER", 4096),
			workerCount:     envInt("OUTBOX_WORKERS", 16),
			batchSize:       envInt("OUTBOX_BATCH", 32),
			flushInterval:   envDur("OUTBOX_FLUSH_INTERVAL", 5*time.Millisecond),
			enqueueTimeout:  envDur("OUTBOX_ENQUEUE_TIMEOUT", 60*time.Second),
			handoffTimeout:  envDur("OUTBOX_HANDOFF_TIMEOUT", 25*time.Millisecond),
			retryInitial:    envDur("OUTBOX_RETRY_INITIAL", 250*time.Millisecond),
			retryMax:        envDur("OUTBOX_RETRY_MAX", 30*time.Second),
			walDir:          envString("OUTBOX_DIR", filepath.Join(os.TempDir(), "prism-outbox")),
			walSegmentSize:  int64(envInt("OUTBOX_SEGMENT_MB", 128)) * 1024 * 1024,
			walSyncEvery:    envInt("OUTBOX_SYNC_EVERY", 1),
			walSyncInterval: envDur("OUTBOX_SYNC_INTERVAL", 2*time.Millisecond),
		}
		if cfg.workerCount <= 0 {
			cfg.workerCount = 1
		}
		if cfg.batchSize <= 0 {
			cfg.batchSize = 1
		}
		if cfg.bufferSize <= 0 {
			cfg.bufferSize = cfg.workerCount * cfg.batchSize * 2
		}
		if cfg.walSegmentSize <= 0 {
			cfg.walSegmentSize = 64 * 1024 * 1024
		}
		if cfg.walSyncEvery <= 0 {
			cfg.walSyncEvery = 1
		}

		walCfg := walConfig{
			dir:          cfg.walDir,
			segmentBytes: cfg.walSegmentSize,
			syncEvery:    cfg.walSyncEvery,
			syncInterval: cfg.walSyncInterval,
			logger:       logger,
		}

		w, pending, err := openWAL(walCfg)
		if err != nil {
			logger.Fatalf("failed to initialize command outbox WAL: %v", err)
		}

		globalOutbox = newCommandOutbox(cfg, store, logger, w, pending)
		globalOutbox.start()
	})
}

func newCommandOutbox(cfg outboxConfig, store Storage, logger *log.Logger, w *wal, pending []*walRecord) *commandOutbox {
	ob := &commandOutbox{
		cfg:      cfg,
		store:    store,
		logger:   logger,
		wal:      w,
		workCh:   make(chan *walRecord, cfg.bufferSize),
		stopCh:   make(chan struct{}),
		inflight: make(map[uint64]*walRecord),
		acked:    make(map[uint64]struct{}),
		nextAck:  w.committedOffset,
		started:  time.Now().UTC(),
	}

	sort.Slice(pending, func(i, j int) bool { return pending[i].Offset < pending[j].Offset })
	for _, rec := range pending {
		cpy := rec
		ob.inflight[rec.Offset] = cpy
	}

	go func() {
		for _, rec := range pending {
			ob.enqueueRecovered(rec)
		}
	}()

	return ob
}

func (o *commandOutbox) start() {
	for i := 0; i < o.cfg.workerCount; i++ {
		o.workerWG.Add(1)
		go o.worker(i)
	}
	if o.wal.cfg.syncInterval > 0 {
		go o.syncLoop()
	}
}

func (o *commandOutbox) syncLoop() {
	ticker := time.NewTicker(o.wal.cfg.syncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			o.wal.mu.Lock()
			if err := o.wal.syncLocked(); err != nil {
				if errors.Is(err, errWALClosed) {
					o.wal.mu.Unlock()
					return
				}
				o.logger.WithError(err).Error("outbox wal sync failed")
			}
			o.wal.mu.Unlock()
		case <-o.stopCh:
			return
		}
	}
}

func (o *commandOutbox) shutdown() {
	o.mu.Lock()
	if o.closing {
		o.mu.Unlock()
		return
	}
	o.closing = true
	close(o.stopCh)
	o.mu.Unlock()

	close(o.workCh)
	o.workerWG.Wait()
	o.retryWG.Wait()
	o.wal.close()
}

func (o *commandOutbox) enqueueRecovered(rec *walRecord) {
	select {
	case o.workCh <- rec:
	case <-o.stopCh:
	}
}

func (o *commandOutbox) enqueue(job enqueueJob) error {
	if len(job.cmds) == 0 {
		return nil
	}

	rec := &walRecord{
		UserID:    job.userID,
		Commands:  cloneCommands(job.cmds),
		AddedKeys: append([]string(nil), job.added...),
		Timestamp: time.Now().UTC(),
	}

	o.wal.mu.Lock()
	if err := o.wal.appendRecordLocked(rec); err != nil {
		o.wal.mu.Unlock()
		return err
	}
	if err := o.wal.syncIfNeededLocked(); err != nil {
		if rbErr := o.wal.rollbackRecordLocked(rec); rbErr != nil {
			o.logger.WithError(rbErr).Error("wal rollback failed")
		}
		o.wal.mu.Unlock()
		return err
	}

	o.mu.Lock()
	o.inflight[rec.Offset] = rec
	o.mu.Unlock()

	if err := o.dispatchLocked(rec); err != nil {
		o.mu.Lock()
		delete(o.inflight, rec.Offset)
		o.mu.Unlock()
		if rbErr := o.wal.rollbackRecordLocked(rec); rbErr != nil {
			o.logger.WithError(rbErr).Error("wal rollback failed")
		}
		if syncErr := o.wal.syncLocked(); syncErr != nil {
			o.logger.WithError(syncErr).Error("wal sync after rollback failed")
		}
		o.wal.mu.Unlock()
		return err
	}
	o.wal.mu.Unlock()

	return nil
}

func (o *commandOutbox) dispatchLocked(rec *walRecord) error {
	if o.cfg.handoffTimeout <= 0 {
		select {
		case o.workCh <- rec:
			return nil
		default:
			return errOutboxSaturated
		}
	}

	timer := time.NewTimer(o.cfg.handoffTimeout)
	defer timer.Stop()

	select {
	case o.workCh <- rec:
		return nil
	case <-timer.C:
		return errOutboxSaturated
	case <-o.stopCh:
		return errors.New("outbox shutting down")
	}
}

func (o *commandOutbox) worker(id int) {
	defer o.workerWG.Done()

	batch := make([]*walRecord, 0, o.cfg.batchSize)
	timer := time.NewTimer(o.cfg.flushInterval)
	defer timer.Stop()
	for {
		if len(batch) == 0 {
			select {
			case rec, ok := <-o.workCh:
				if !ok {
					return
				}
				if rec == nil {
					continue
				}
				batch = append(batch, rec)
				timer.Reset(o.cfg.flushInterval)
			case <-o.stopCh:
				return
			}
		}

	gather:
		for len(batch) < o.cfg.batchSize {
			select {
			case rec, ok := <-o.workCh:
				if !ok {
					break gather
				}
				if rec == nil {
					continue
				}
				batch = append(batch, rec)
			case <-timer.C:
				timer.Reset(o.cfg.flushInterval)
				break gather
			case <-o.stopCh:
				return
			}
		}

		o.flushBatch(batch, id)
		batch = batch[:0]
	}
}

func (o *commandOutbox) flushBatch(batch []*walRecord, workerID int) {
	if len(batch) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), o.cfg.enqueueTimeout)
	defer cancel()

	successes := make([]*walRecord, 0, len(batch))
	failures := make([]*walRecord, 0)
	for _, rec := range batch {
		if err := o.store.EnqueueCommands(ctx, rec.UserID, rec.Commands); err != nil {
			rec.Attempt++
			rec.LastErr = err.Error()
			failures = append(failures, rec)
			o.logger.WithError(err).Errorf("command outbox enqueue failed, worker=%d, user=%s, cmds=%d, offset=%d, attempt=%d", workerID, rec.UserID, len(rec.Commands), rec.Offset, rec.Attempt)
		} else {
			rec.Attempt = 0
			rec.LastErr = ""
			successes = append(successes, rec)
		}
	}

	if len(successes) > 0 {
		o.markDelivered(successes)
	}
	for _, rec := range failures {
		o.scheduleRetry(rec)
	}
}

func (o *commandOutbox) markDelivered(records []*walRecord) {
	var maxCommit uint64

	o.mu.Lock()
	for _, rec := range records {
		delete(o.inflight, rec.Offset)
		o.acked[rec.Offset] = struct{}{}
	}
	o.delivered.Add(uint64(len(records)))

	for {
		next := o.nextAck + 1
		if _, ok := o.acked[next]; ok {
			delete(o.acked, next)
			o.nextAck = next
			maxCommit = next
		} else {
			break
		}
	}
	o.mu.Unlock()

	if maxCommit > 0 {
		o.wal.mu.Lock()
		if err := o.wal.commitLocked(maxCommit); err != nil {
			o.logger.WithError(err).Error("failed to commit outbox WAL")
		}
		o.wal.mu.Unlock()
	}
}

func (o *commandOutbox) scheduleRetry(rec *walRecord) {
	delay := exponentialBackoff(rec.Attempt, o.cfg.retryInitial, o.cfg.retryMax)
	o.retryWG.Add(1)
	timer := time.NewTimer(delay)
	go func(r *walRecord) {
		defer o.retryWG.Done()
		defer timer.Stop()
		select {
		case <-timer.C:
			select {
			case o.workCh <- r:
			case <-o.stopCh:
			}
		case <-o.stopCh:
		}
	}(rec)
}

func exponentialBackoff(attempt int, initial, max time.Duration) time.Duration {
	if attempt <= 0 {
		if initial <= 0 {
			return time.Second
		}
		return initial
	}
	if initial <= 0 {
		initial = time.Second
	}
	if max <= 0 {
		max = 10 * time.Second
	}
	backoff := float64(initial) * math.Pow(2, float64(attempt-1))
	if backoff > float64(max) {
		backoff = float64(max)
	}
	jitter := 0.2 * backoff
	return time.Duration(backoff + (rand.Float64()-0.5)*2*jitter)
}

func cloneCommands(cmds []domain.Command) []domain.Command {
	if len(cmds) == 0 {
		return nil
	}
	out := make([]domain.Command, len(cmds))
	copy(out, cmds)
	return out
}

func enqueueCommands(job enqueueJob) error {
	if globalOutbox == nil {
		return errors.New("command outbox unavailable")
	}
	return globalOutbox.enqueue(job)
}

func (o *commandOutbox) stats() outboxStats {
	o.mu.Lock()
	defer o.mu.Unlock()

	depth := len(o.inflight)
	buffered := len(o.workCh)
	var oldest time.Duration
	now := time.Now()
	for _, rec := range o.inflight {
		age := now.Sub(rec.Timestamp)
		if age < 0 {
			age = 0
		}
		if age > oldest {
			oldest = age
		}
	}

	delivered := o.delivered.Load()
	elapsed := time.Since(o.started)
	rps := 0.0
	if elapsed > 0 {
		rps = float64(delivered) / elapsed.Seconds()
	}

	return outboxStats{
		QueueDepth: depth,
		Buffered:   buffered,
		OldestAge:  oldest,
		Delivered:  delivered,
		StartedAt:  o.started,
		DrainRate:  rps,
	}
}

type outboxStats struct {
	QueueDepth int           `json:"queueDepth"`
	Buffered   int           `json:"buffered"`
	OldestAge  time.Duration `json:"oldestAge"`
	Delivered  uint64        `json:"delivered"`
	StartedAt  time.Time     `json:"startedAt"`
	DrainRate  float64       `json:"drainRatePerSecond"`
}

func getOutboxStats() (outboxStats, error) {
	if globalOutbox == nil {
		return outboxStats{}, errors.New("command outbox unavailable")
	}
	return globalOutbox.stats(), nil
}

func shutdownCommandSender() {
	if globalOutbox != nil {
		globalOutbox.shutdown()
	}
	globalOutbox = nil
	outboxOnce = sync.Once{}
}
