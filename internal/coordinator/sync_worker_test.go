package coordinator

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSyncWorker_ProcessesJobsOneAtATime(t *testing.T) {
	bus := NewEventBus(16)
	state := NewAppState()
	worker := NewSyncWorker(16, state, bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go worker.Run(ctx)

	var wg sync.WaitGroup
	var running int32
	var maxRunning int32

	for i := 0; i < 3; i++ {
		wg.Add(1)
		err := worker.Enqueue(SyncJob{
			Name: "job",
			Execute: func(ctx context.Context) error {
				defer wg.Done()
				cur := atomic.AddInt32(&running, 1)
				for {
					prev := atomic.LoadInt32(&maxRunning)
					if cur <= prev || atomic.CompareAndSwapInt32(&maxRunning, prev, cur) {
						break
					}
				}

				select {
				case <-ctx.Done():
					atomic.AddInt32(&running, -1)
					return ctx.Err()
				case <-time.After(30 * time.Millisecond):
				}

				atomic.AddInt32(&running, -1)
				return nil
			},
		})
		if err != nil {
			t.Fatalf("enqueue failed: %v", err)
		}
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("jobs did not complete in time")
	}

	if got := atomic.LoadInt32(&maxRunning); got != 1 {
		t.Fatalf("max concurrent running jobs = %d, want 1", got)
	}

	snap := state.Snapshot()
	if snap.SyncInProgress {
		t.Fatal("sync state should be false after all jobs completed")
	}
}

func TestSyncWorker_RejectsInvalidJobs(t *testing.T) {
	worker := NewSyncWorker(1, NewAppState(), NewEventBus(4))

	if err := worker.Enqueue(SyncJob{}); !errors.Is(err, ErrInvalidJob) {
		t.Fatalf("err = %v, want %v", err, ErrInvalidJob)
	}
}

func TestSyncWorker_ReturnsQueueFull(t *testing.T) {
	worker := NewSyncWorker(1, NewAppState(), NewEventBus(4))

	// Fill queue without starting the worker run loop.
	if err := worker.Enqueue(SyncJob{Name: "a", Execute: func(context.Context) error { return nil }}); err != nil {
		t.Fatalf("first enqueue failed: %v", err)
	}
	if err := worker.Enqueue(SyncJob{Name: "b", Execute: func(context.Context) error { return nil }}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("err = %v, want %v", err, ErrQueueFull)
	}
}
