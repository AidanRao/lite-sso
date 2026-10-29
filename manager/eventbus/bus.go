// Package eventbus dispatches best-effort business events in bounded memory.
package eventbus

import (
	"context"
	"log"
	"sync"
	"time"

	"sso-server/common/event"
)

const (
	queueCapacity   = 128
	workerCount     = 2
	consumerTimeout = 5 * time.Second
)

// Handler consumes an event using the supplied bounded context.
type Handler func(context.Context, event.Event) error

// Subscription binds one consumer to a business event type at startup.
type Subscription struct {
	Type   string
	Handle Handler
}

// Bus owns a fixed worker pool and immutable subscriptions.
type Bus struct {
	mu       sync.RWMutex
	closed   bool
	queue    chan event.Event
	handlers map[string][]Handler
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
}

// New starts workers after copying all subscriptions.
func New(subscriptions ...Subscription) *Bus {
	ctx, cancel := context.WithCancel(context.Background())
	b := &Bus{queue: make(chan event.Event, queueCapacity), handlers: make(map[string][]Handler), ctx: ctx, cancel: cancel, done: make(chan struct{})}
	for _, subscription := range subscriptions {
		if subscription.Handle != nil {
			b.handlers[subscription.Type] = append(b.handlers[subscription.Type], subscription.Handle)
		}
	}
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for range workerCount {
		go func() { defer workers.Done(); b.run() }()
	}
	go func() { workers.Wait(); close(b.done) }()
	return b
}

// TryPublish never blocks login on queue capacity or consumer work.
func (b *Bus) TryPublish(e event.Event) bool {
	if b == nil || e == nil {
		return false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		log.Printf("eventbus event_id=%s reason=closed", e.EventID())
		return false
	}
	if len(b.handlers[e.EventType()]) == 0 {
		return true
	}
	select {
	case b.queue <- e:
		return true
	default:
		log.Printf("eventbus event_id=%s reason=queue_full", e.EventID())
		return false
	}
}

// Shutdown stops admission and drains accepted events until the deadline.
func (b *Bus) Shutdown(ctx context.Context) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	if !b.closed {
		b.closed = true
		close(b.queue)
	}
	b.mu.Unlock()
	select {
	case <-b.done:
		b.cancel()
		return nil
	case <-ctx.Done():
		b.cancel()
		log.Print("eventbus reason=shutdown_timeout")
		return ctx.Err()
	}
}

func (b *Bus) run() {
	for {
		select {
		case <-b.ctx.Done():
			return
		case e, ok := <-b.queue:
			if !ok {
				return
			}
			for _, handler := range b.handlers[e.EventType()] {
				if b.ctx.Err() != nil {
					return
				}
				b.consume(e, handler)
			}
		}
	}
}

func (b *Bus) consume(e event.Event, handler Handler) {
	ctx, cancel := context.WithTimeout(b.ctx, consumerTimeout)
	defer cancel()
	defer func() {
		if recover() != nil {
			log.Printf("eventbus event_id=%s reason=consumer_panic", e.EventID())
		}
	}()
	if err := handler(ctx, e); err != nil {
		reason := "consumer_failed"
		if ctx.Err() == context.DeadlineExceeded {
			reason = "consumer_timeout"
		}
		if ctx.Err() == context.Canceled {
			reason = "consumer_canceled"
		}
		log.Printf("eventbus event_id=%s reason=%s", e.EventID(), reason)
	}
}
