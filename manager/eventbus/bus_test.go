package eventbus

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"sso-server/common/event"
)

func Test_Bus_QueueFullAndDrain(t *testing.T) {
	started := make(chan struct{}, workerCount)
	release := make(chan struct{})
	var consumed atomic.Int64
	b := New(Subscription{Type: event.LoginSucceededType, Handle: func(ctx context.Context, _ event.Event) error {
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-release:
			consumed.Add(1)
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}})
	t.Cleanup(func() { _ = b.Shutdown(t.Context()) })
	for range workerCount {
		require.True(t, b.TryPublish(event.LoginSucceeded{}))
	}
	for range workerCount {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("workers did not start")
		}
	}
	for range queueCapacity {
		require.True(t, b.TryPublish(event.LoginSucceeded{}))
	}
	require.False(t, b.TryPublish(event.LoginSucceeded{}))
	close(release)
	require.NoError(t, b.Shutdown(t.Context()))
	require.EqualValues(t, queueCapacity+workerCount, consumed.Load())
	require.False(t, b.TryPublish(event.LoginSucceeded{}))
}

func Test_Bus_FailedAndPanickingSubscribersAreIsolated(t *testing.T) {
	var received atomic.Int64
	b := New(
		Subscription{Type: event.LoginSucceededType, Handle: func(context.Context, event.Event) error { panic("private contents") }},
		Subscription{Type: event.LoginSucceededType, Handle: func(context.Context, event.Event) error { return errors.New("private email body") }},
		Subscription{Type: event.LoginSucceededType, Handle: func(context.Context, event.Event) error { received.Add(1); return nil }},
	)
	for range 4 {
		require.True(t, b.TryPublish(event.LoginSucceeded{}))
	}
	require.NoError(t, b.Shutdown(t.Context()))
	require.EqualValues(t, 4, received.Load())
}

func Test_Bus_ShutdownCancelsConsumerAndConcurrentPublishers(t *testing.T) {
	started := make(chan struct{}, workerCount)
	finished := make(chan struct{}, workerCount)
	b := New(Subscription{Type: event.LoginSucceededType, Handle: func(ctx context.Context, _ event.Event) error {
		started <- struct{}{}
		<-ctx.Done()
		finished <- struct{}{}
		return ctx.Err()
	}})
	for range workerCount {
		require.True(t, b.TryPublish(event.LoginSucceeded{}))
	}
	for range workerCount {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("workers did not start")
		}
	}
	var producers sync.WaitGroup
	producers.Add(1)
	go func() {
		defer producers.Done()
		for range 10 {
			b.TryPublish(event.LoginSucceeded{})
		}
	}()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, b.Shutdown(ctx), context.Canceled)
	producers.Wait()
	for range workerCount {
		select {
		case <-finished:
		case <-time.After(time.Second):
			t.Fatal("consumer was not canceled")
		}
	}
	require.NoError(t, b.Shutdown(t.Context()))
}

func Test_Bus_ConsumerDeadline(t *testing.T) {
	deadline := make(chan time.Duration, 1)
	b := New(Subscription{Type: event.LoginSucceededType, Handle: func(ctx context.Context, _ event.Event) error {
		value, ok := ctx.Deadline()
		if !ok {
			return errors.New("missing deadline")
		}
		deadline <- time.Until(value)
		return nil
	}})
	require.True(t, b.TryPublish(event.LoginSucceeded{}))
	require.NoError(t, b.Shutdown(t.Context()))
	select {
	case remaining := <-deadline:
		require.InDelta(t, consumerTimeout.Seconds(), remaining.Seconds(), 1)
	default:
		t.Fatal("consumer did not run")
	}
}
