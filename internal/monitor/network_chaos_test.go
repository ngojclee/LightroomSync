package monitor

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestShareHealthMonitor_ProbeTimeoutPreventsLongStall(t *testing.T) {
	lostCh := make(chan error, 1)
	probe := func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	}

	monitor := NewShareHealthMonitor(ShareHealthConfig{
		CheckInterval:    1 * time.Second,
		ProbeTimeout:     35 * time.Millisecond,
		FailureThreshold: 1,
		OpenTimeout:      20 * time.Millisecond,
	}, probe, ShareHealthHooks{
		OnNetworkLost: func(err error) {
			select {
			case lostCh <- err:
			default:
			}
		},
	})

	start := time.Now()
	monitor.checkOnce(context.Background())
	elapsed := time.Since(start)
	if elapsed > 300*time.Millisecond {
		t.Fatalf("probe check stalled too long: %v", elapsed)
	}

	select {
	case err := <-lostCh:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("lost error = %v, want deadline exceeded", err)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected network lost callback for timeout")
	}
}

func TestShareHealthMonitor_Run_HandlesDisconnectReconnectMidOperation(t *testing.T) {
	lostCh := make(chan error, 2)
	recoveredCh := make(chan struct{}, 2)

	var disconnected atomic.Bool
	probe := func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(8 * time.Millisecond):
		}

		if disconnected.Load() {
			return errors.New("simulated network disconnect")
		}
		return nil
	}

	monitor := NewShareHealthMonitor(ShareHealthConfig{
		CheckInterval:    12 * time.Millisecond,
		ProbeTimeout:     80 * time.Millisecond,
		FailureThreshold: 2,
		OpenTimeout:      20 * time.Millisecond,
	}, probe, ShareHealthHooks{
		OnNetworkLost: func(err error) {
			select {
			case lostCh <- err:
			default:
			}
		},
		OnNetworkRecovered: func() {
			select {
			case recoveredCh <- struct{}{}:
			default:
			}
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go monitor.Run(ctx)

	time.Sleep(40 * time.Millisecond)
	disconnected.Store(true)

	select {
	case <-lostCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected network lost callback")
	}

	disconnected.Store(false)

	select {
	case <-recoveredCh:
	case <-time.After(700 * time.Millisecond):
		t.Fatal("expected network recovered callback")
	}
}
