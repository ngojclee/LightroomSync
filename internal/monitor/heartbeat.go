package monitor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

// LockWriter is the minimal dependency required by HeartbeatManager.
type LockWriter interface {
	WriteLock(ctx context.Context, info LockInfo) error
}

// HeartbeatConfig controls write frequency and retry behavior.
type HeartbeatConfig struct {
	Interval        time.Duration
	RetryBase       time.Duration
	RetryMax        time.Duration
	MaxRetries      int
	ShutdownTimeout time.Duration
}

// HeartbeatHooks allows the caller to observe heartbeat outcomes.
type HeartbeatHooks struct {
	OnHeartbeat func(info LockInfo)
	OnError     func(err error)
}

// HeartbeatManager writes ONLINE lock heartbeats periodically and OFFLINE on shutdown.
type HeartbeatManager struct {
	writer  LockWriter
	machine string
	cfg     HeartbeatConfig
	hooks   HeartbeatHooks
}

// NewHeartbeatManager creates a heartbeat loop writer for lock status.
func NewHeartbeatManager(writer LockWriter, machine string, cfg HeartbeatConfig, hooks HeartbeatHooks) *HeartbeatManager {
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.RetryBase <= 0 {
		cfg.RetryBase = 500 * time.Millisecond
	}
	if cfg.RetryMax <= 0 {
		cfg.RetryMax = 5 * time.Second
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 2 * time.Second
	}

	return &HeartbeatManager{
		writer:  writer,
		machine: machine,
		cfg:     cfg,
		hooks:   hooks,
	}
}

// Run blocks until ctx is cancelled.
func (h *HeartbeatManager) Run(ctx context.Context) {
	// Initial ONLINE heartbeat immediately on start.
	h.writeAndReport(ctx, LockOnline)

	ticker := time.NewTicker(h.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Best-effort OFFLINE write on shutdown.
			shutdownCtx, cancel := context.WithTimeout(context.Background(), h.cfg.ShutdownTimeout)
			defer cancel()
			h.writeAndReport(shutdownCtx, LockOffline)
			return
		case <-ticker.C:
			h.writeAndReport(ctx, LockOnline)
		}
	}
}

func (h *HeartbeatManager) writeAndReport(ctx context.Context, status LockStatus) {
	err := h.writeWithRetry(ctx, status)
	if err == nil {
		if h.hooks.OnHeartbeat != nil {
			h.hooks.OnHeartbeat(LockInfo{
				Status:    status,
				Machine:   h.machine,
				Timestamp: time.Now().UTC(),
			})
		}
		return
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}

	if h.hooks.OnError != nil {
		h.hooks.OnError(err)
		return
	}
	log.Printf("[WARN] heartbeat write failed: %v", err)
}

func (h *HeartbeatManager) writeWithRetry(ctx context.Context, status LockStatus) error {
	var lastErr error
	delay := h.cfg.RetryBase

	for attempt := 1; attempt <= h.cfg.MaxRetries; attempt++ {
		info := LockInfo{
			Status:    status,
			Machine:   h.machine,
			Timestamp: time.Now().UTC(),
		}
		if err := h.writer.WriteLock(ctx, info); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if errors.Is(lastErr, context.Canceled) || errors.Is(lastErr, context.DeadlineExceeded) {
			return lastErr
		}
		if attempt == h.cfg.MaxRetries {
			break
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}

		delay *= 2
		if delay > h.cfg.RetryMax {
			delay = h.cfg.RetryMax
		}
	}

	return fmt.Errorf("heartbeat write failed after %d attempts: %w", h.cfg.MaxRetries, lastErr)
}
