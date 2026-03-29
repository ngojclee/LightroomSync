package monitor

import (
	"context"
	"log"
	"time"

	"github.com/ngojclee/lightroom-sync/internal/platform/common"
)

// LightroomHooks receives monitor transition/error callbacks.
type LightroomHooks struct {
	OnStarted func()
	OnStopped func()
	OnError   func(err error)
}

// LightroomMonitor detects Lightroom process start/stop edges.
type LightroomMonitor struct {
	detector     common.ProcessDetector
	interval     time.Duration
	processNames []string
	hooks        LightroomHooks
}

// NewLightroomMonitor creates a Lightroom process monitor.
func NewLightroomMonitor(detector common.ProcessDetector, interval time.Duration, processNames []string, hooks LightroomHooks) *LightroomMonitor {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if len(processNames) == 0 {
		processNames = []string{"Lightroom.exe"}
	}
	return &LightroomMonitor{
		detector:     detector,
		interval:     interval,
		processNames: processNames,
		hooks:        hooks,
	}
}

// Run blocks until context cancellation.
func (m *LightroomMonitor) Run(ctx context.Context) {
	var known bool
	hasKnown := false

	// Immediate probe on startup.
	m.probeAndDispatch(&known, &hasKnown)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.probeAndDispatch(&known, &hasKnown)
		}
	}
}

func (m *LightroomMonitor) probeAndDispatch(known *bool, hasKnown *bool) {
	running, err := m.detector.IsRunning(m.processNames)
	if err != nil {
		if m.hooks.OnError != nil {
			m.hooks.OnError(err)
		} else {
			log.Printf("[WARN] lightroom monitor check failed: %v", err)
		}
		return
	}

	if !*hasKnown {
		*known = running
		*hasKnown = true
		if running {
			m.onStarted()
		}
		return
	}

	if running == *known {
		return
	}

	*known = running
	if running {
		m.onStarted()
		return
	}
	m.onStopped()
}

func (m *LightroomMonitor) onStarted() {
	if m.hooks.OnStarted != nil {
		m.hooks.OnStarted()
	}
}

func (m *LightroomMonitor) onStopped() {
	if m.hooks.OnStopped != nil {
		m.hooks.OnStopped()
	}
}
