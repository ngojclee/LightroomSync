package monitor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type stubLockWriter struct {
	mu         sync.Mutex
	failUntil  int
	attempts   int
	writtenLog []LockInfo
}

func (w *stubLockWriter) WriteLock(_ context.Context, info LockInfo) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.attempts++
	if w.attempts <= w.failUntil {
		return errors.New("simulated write failure")
	}

	w.writtenLog = append(w.writtenLog, info)
	return nil
}

func (w *stubLockWriter) attemptsCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.attempts
}

func (w *stubLockWriter) statuses() []LockStatus {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]LockStatus, 0, len(w.writtenLog))
	for _, info := range w.writtenLog {
		out = append(out, info.Status)
	}
	return out
}

func TestHeartbeat_WriteWithRetry_SucceedsWithinBudget(t *testing.T) {
	writer := &stubLockWriter{failUntil: 2}
	hb := NewHeartbeatManager(writer, "TEST-PC", HeartbeatConfig{
		Interval:        1 * time.Second,
		RetryBase:       1 * time.Millisecond,
		RetryMax:        4 * time.Millisecond,
		MaxRetries:      4,
		ShutdownTimeout: 20 * time.Millisecond,
	}, HeartbeatHooks{})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := hb.writeWithRetry(ctx, LockOnline); err != nil {
		t.Fatalf("writeWithRetry should succeed: %v", err)
	}
	if got := writer.attemptsCount(); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}
}

func TestHeartbeat_WriteWithRetry_FailsWhenBudgetExceeded(t *testing.T) {
	writer := &stubLockWriter{failUntil: 5}
	hb := NewHeartbeatManager(writer, "TEST-PC", HeartbeatConfig{
		Interval:        1 * time.Second,
		RetryBase:       1 * time.Millisecond,
		RetryMax:        4 * time.Millisecond,
		MaxRetries:      3,
		ShutdownTimeout: 20 * time.Millisecond,
	}, HeartbeatHooks{})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := hb.writeWithRetry(ctx, LockOnline); err == nil {
		t.Fatal("writeWithRetry should fail when retry budget exceeded")
	}
	if got := writer.attemptsCount(); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}
}

func TestHeartbeat_Run_WritesOfflineOnShutdown(t *testing.T) {
	writer := &stubLockWriter{}
	hb := NewHeartbeatManager(writer, "TEST-PC", HeartbeatConfig{
		Interval:        200 * time.Millisecond,
		RetryBase:       1 * time.Millisecond,
		RetryMax:        4 * time.Millisecond,
		MaxRetries:      2,
		ShutdownTimeout: 100 * time.Millisecond,
	}, HeartbeatHooks{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		hb.Run(ctx)
		close(done)
	}()

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		statuses := writer.statuses()
		if len(statuses) >= 1 && statuses[0] == LockOnline {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("heartbeat run did not stop after cancellation")
	}

	statuses := writer.statuses()
	if len(statuses) < 2 {
		t.Fatalf("written statuses = %v, want at least [ONLINE, OFFLINE]", statuses)
	}
	if statuses[0] != LockOnline {
		t.Fatalf("first status = %q, want %q", statuses[0], LockOnline)
	}
	if statuses[len(statuses)-1] != LockOffline {
		t.Fatalf("last status = %q, want %q", statuses[len(statuses)-1], LockOffline)
	}
}
