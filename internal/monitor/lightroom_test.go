package monitor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type detectorStep struct {
	running bool
	err     error
}

type sequenceDetector struct {
	mu    sync.Mutex
	steps []detectorStep
	idx   int
}

func (d *sequenceDetector) IsRunning(_ []string) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.steps) == 0 {
		return false, nil
	}
	if d.idx >= len(d.steps) {
		last := d.steps[len(d.steps)-1]
		return last.running, last.err
	}
	step := d.steps[d.idx]
	d.idx++
	return step.running, step.err
}

func TestLightroomMonitor_EmitsEdgeTransitions(t *testing.T) {
	detector := &sequenceDetector{
		steps: []detectorStep{
			{running: false},
			{running: true},
			{running: true},
			{running: false},
		},
	}

	var mu sync.Mutex
	events := make([]string, 0, 2)
	monitor := NewLightroomMonitor(detector, 20*time.Millisecond, []string{"Lightroom.exe"}, LightroomHooks{
		OnStarted: func() {
			mu.Lock()
			events = append(events, "started")
			mu.Unlock()
		},
		OnStopped: func() {
			mu.Lock()
			events = append(events, "stopped")
			mu.Unlock()
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		monitor.Run(ctx)
		close(done)
	}()

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		mu.Lock()
		if len(events) >= 2 {
			mu.Unlock()
			break
		}
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()
	if len(events) != 2 {
		t.Fatalf("events = %v, want [started stopped]", events)
	}
	if events[0] != "started" || events[1] != "stopped" {
		t.Fatalf("events order = %v, want [started stopped]", events)
	}
}

func TestLightroomMonitor_ReportsErrors(t *testing.T) {
	detector := &sequenceDetector{
		steps: []detectorStep{
			{running: false, err: errors.New("scan failed")},
		},
	}

	errCh := make(chan error, 1)
	monitor := NewLightroomMonitor(detector, 50*time.Millisecond, nil, LightroomHooks{
		OnError: func(err error) {
			select {
			case errCh <- err:
			default:
			}
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	go monitor.Run(ctx)
	defer cancel()

	select {
	case err := <-errCh:
		if err == nil || err.Error() != "scan failed" {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected monitor error callback")
	}
}
