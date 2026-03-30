package monitor

import (
	"context"
	"sync"
	"testing"
	"time"
)

type blockingHeartbeatWriter struct {
	mu             sync.Mutex
	blockFirstCall bool
	release        chan struct{}
	writes         []LockInfo
}

func (w *blockingHeartbeatWriter) WriteLock(ctx context.Context, info LockInfo) error {
	w.mu.Lock()
	shouldBlock := w.blockFirstCall
	if shouldBlock {
		w.blockFirstCall = false
	}
	release := w.release
	w.mu.Unlock()

	if shouldBlock {
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	w.mu.Lock()
	w.writes = append(w.writes, info)
	w.mu.Unlock()
	return nil
}

func (w *blockingHeartbeatWriter) statuses() []LockStatus {
	w.mu.Lock()
	defer w.mu.Unlock()

	out := make([]LockStatus, 0, len(w.writes))
	for _, item := range w.writes {
		out = append(out, item.Status)
	}
	return out
}

func TestHeartbeat_Run_RecoversAfterTemporaryStall(t *testing.T) {
	writer := &blockingHeartbeatWriter{
		blockFirstCall: true,
		release:        make(chan struct{}),
	}

	hb := NewHeartbeatManager(writer, "TEST-PC", HeartbeatConfig{
		Interval:        20 * time.Millisecond,
		RetryBase:       2 * time.Millisecond,
		RetryMax:        10 * time.Millisecond,
		MaxRetries:      2,
		ShutdownTimeout: 100 * time.Millisecond,
	}, HeartbeatHooks{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		hb.Run(ctx)
		close(done)
	}()

	time.Sleep(60 * time.Millisecond)
	if got := len(writer.statuses()); got != 0 {
		t.Fatalf("expected stalled first write before resume, got %d writes", got)
	}

	close(writer.release)

	deadline := time.Now().Add(600 * time.Millisecond)
	for time.Now().Before(deadline) {
		statuses := writer.statuses()
		if len(statuses) >= 1 && statuses[0] == LockOnline {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()

	select {
	case <-done:
	case <-time.After(900 * time.Millisecond):
		t.Fatal("heartbeat loop did not stop after cancellation")
	}

	statuses := writer.statuses()
	if len(statuses) < 2 {
		t.Fatalf("expected at least ONLINE then OFFLINE writes, got %v", statuses)
	}
	if statuses[0] != LockOnline {
		t.Fatalf("first status = %q, want %q", statuses[0], LockOnline)
	}
	if statuses[len(statuses)-1] != LockOffline {
		t.Fatalf("last status = %q, want %q", statuses[len(statuses)-1], LockOffline)
	}
}
