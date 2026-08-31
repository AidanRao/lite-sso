// Package audit provides bounded, best-effort asynchronous operation auditing.
package audit

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"sso-server/conf"
	"sso-server/model"
)

const (
	retentionDays    = 90
	cleanupBatchSize = 500
	cleanupInterval  = time.Hour
	cleanupTimeout   = 30 * time.Second
)

var counterNames = [...]string{"queue_full", "write_failed", "cleanup_failed", "shutdown_timeout", "shutdown_discarded", "invalid_event", "closed"}

// Store is the persistence boundary; request contexts never cross this interface.
type Store interface {
	InsertBatch(context.Context, []model.AuditLog) error
	DeleteExpired(context.Context, time.Time, int) (int64, error)
}

// Recorder owns the queue and its two fixed background workers.
type Recorder struct {
	store         Store
	config        conf.AuditConfig
	queue         chan model.AuditLog
	mu            sync.RWMutex
	closed        bool
	writeCtx      context.Context
	cancelWrite   context.CancelFunc
	cancelCleanup context.CancelFunc
	done          chan struct{}
	counters      [7]atomic.Uint64
	reportMu      sync.Mutex
	lastReport    time.Time
	lastCounts    [7]uint64
}

// New validates settings and starts a writer and a retention worker.
func New(store Store, config conf.AuditConfig) (*Recorder, error) {
	if store == nil {
		return nil, fmt.Errorf("audit store is required")
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	writeCtx, cancelWrite := context.WithCancel(context.Background())
	cleanupCtx, cancelCleanup := context.WithCancel(context.Background())
	r := &Recorder{store: store, config: config.WithDefaults(), writeCtx: writeCtx, cancelWrite: cancelWrite, cancelCleanup: cancelCleanup, done: make(chan struct{})}
	r.queue = make(chan model.AuditLog, r.config.QueueCapacity)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() { defer workers.Done(); r.consume() }()
	go func() { defer workers.Done(); r.clean(cleanupCtx) }()
	go func() { workers.Wait(); r.cancelWrite(); r.report(true); close(r.done) }()
	return r, nil
}

// TryRecord sanitizes and snapshots an event, then attempts a nonblocking enqueue.
func (r *Recorder) TryRecord(event model.AuditLog) bool {
	event, ok := snapshot(event)
	if !ok {
		r.counters[5].Add(1)
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		r.counters[6].Add(1)
		return false
	}
	select {
	case r.queue <- event:
		return true
	default:
		r.counters[0].Add(1)
		return false
	}
}

// Stats returns aggregate counters, with no event content or identifiers.
func (r *Recorder) Stats() map[string]uint64 {
	result := make(map[string]uint64, len(counterNames))
	for i, name := range counterNames {
		result[name] = r.counters[i].Load()
	}
	return result
}

// Shutdown stops cleanup and drains accepted events until the deadline.
// It is safe to call concurrently with producers and with other shutdown calls.
func (r *Recorder) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	if !r.closed {
		r.closed = true
		close(r.queue)
		r.cancelCleanup()
	}
	r.mu.Unlock()
	select {
	case <-r.done:
		return nil
	default:
	}
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		r.counters[3].Add(1)
		r.cancelWrite()
		return ctx.Err()
	}
}

func (r *Recorder) consume() {
	ticker := time.NewTicker(r.config.FlushInterval)
	defer ticker.Stop()
	batch := make([]model.AuditLog, 0, r.config.BatchSize)
	flush := func() {
		if len(batch) > 0 {
			r.write(batch)
			clear(batch)
			batch = batch[:0]
		}
		r.report(false)
	}
	for {
		if r.writeCtx.Err() != nil {
			discarded := len(batch)
			for range r.queue {
				discarded++
			}
			r.counters[4].Add(uint64(discarded))
			return
		}
		select {
		case event, ok := <-r.queue:
			if !ok {
				flush()
				return
			}
			batch = append(batch, event)
			if len(batch) >= r.config.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-r.writeCtx.Done():
		}
	}
}

func (r *Recorder) write(batch []model.AuditLog) {
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			delay := 100 * time.Millisecond
			if attempt == 2 {
				delay = 500 * time.Millisecond
			}
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-r.writeCtx.Done():
				timer.Stop()
			}
		}
		if r.writeCtx.Err() != nil {
			r.counters[4].Add(uint64(len(batch)))
			return
		}
		insertedAt := time.Now().UTC()
		for i := range batch {
			batch[i].CreatedAt = insertedAt
		}
		ctx, cancel := context.WithTimeout(r.writeCtx, r.config.WriteTimeout)
		err := r.store.InsertBatch(ctx, batch)
		cancel()
		if err == nil {
			return
		}
	}
	if r.writeCtx.Err() != nil {
		r.counters[4].Add(uint64(len(batch)))
	} else {
		r.counters[1].Add(uint64(len(batch)))
	}
}

func (r *Recorder) clean(ctx context.Context) {
	r.cleanup(ctx)
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.cleanup(ctx)
		}
	}
}

func (r *Recorder) cleanup(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, cleanupTimeout)
	defer cancel()
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	for ctx.Err() == nil {
		batchCtx, cancelBatch := context.WithTimeout(ctx, r.config.WriteTimeout)
		count, err := r.store.DeleteExpired(batchCtx, cutoff, cleanupBatchSize)
		cancelBatch()
		if err != nil {
			if parent.Err() == nil {
				r.counters[2].Add(1)
				r.report(false)
			}
			return
		}
		if count < cleanupBatchSize {
			return
		}
	}
}

func (r *Recorder) report(force bool) {
	r.reportMu.Lock()
	defer r.reportMu.Unlock()
	if !force && time.Since(r.lastReport) < 30*time.Second {
		return
	}
	var counts [7]uint64
	for i := range counts {
		counts[i] = r.counters[i].Load()
	}
	if counts == r.lastCounts {
		return
	}
	r.lastCounts, r.lastReport = counts, time.Now()
	log.Printf("audit counters: queue_full=%d write_failed=%d cleanup_failed=%d shutdown_timeout=%d shutdown_discarded=%d invalid_event=%d closed=%d", counts[0], counts[1], counts[2], counts[3], counts[4], counts[5], counts[6])
}
