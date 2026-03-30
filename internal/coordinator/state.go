package coordinator

import (
	"sync"

	"github.com/ngojclee/lightroom-sync/internal/ipc"
)

// AppState is the single authoritative state cache for the Agent.
// Both tray and IPC reads come from here — no direct filesystem queries from UI.
type AppState struct {
	mu sync.RWMutex

	lightroomRunning       bool
	syncInProgress         bool
	syncPaused             bool
	trayColor              string // green, blue, orange, red
	statusText             string
	lastBackup             string
	lockMachine            string
	lockStatus             string
	migrationHint          string
	lightroomMonitorErrors int
	backupMonitorErrors    int
	networkMonitorErrors   int
	lockMonitorErrors      int
	lastResumeGapSeconds   int
	autoSync               bool
}

func NewAppState() *AppState {
	return &AppState{
		trayColor:  "green",
		statusText: "Ready",
	}
}

// Snapshot returns a copy of the current state for IPC responses.
func (s *AppState) Snapshot() ipc.AppStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ipc.AppStatus{
		TrayColor:              s.trayColor,
		StatusText:             s.statusText,
		LightroomRunning:       s.lightroomRunning,
		SyncInProgress:         s.syncInProgress,
		SyncPaused:             s.syncPaused,
		LastBackup:             s.lastBackup,
		LockMachine:            s.lockMachine,
		LockStatus:             s.lockStatus,
		MigrationHint:          s.migrationHint,
		LightroomMonitorErrors: s.lightroomMonitorErrors,
		BackupMonitorErrors:    s.backupMonitorErrors,
		NetworkMonitorErrors:   s.networkMonitorErrors,
		LockMonitorErrors:      s.lockMonitorErrors,
		LastResumeGapSeconds:   s.lastResumeGapSeconds,
		AutoSync:               s.autoSync,
	}
}

func (s *AppState) SetLightroomRunning(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lightroomRunning = v
	s.recomputeDerivedStatusLocked()
}

func (s *AppState) SetSyncing(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncInProgress = v
	s.recomputeDerivedStatusLocked()
}

func (s *AppState) SetSyncPaused(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncPaused = v
	s.recomputeDerivedStatusLocked()
}

func (s *AppState) SetWarning(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trayColor = "orange"
	s.statusText = text
}

func (s *AppState) SetLock(machine, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lockMachine = machine
	s.lockStatus = status
}

func (s *AppState) SetLastBackup(info string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastBackup = info
}

func (s *AppState) SetMigrationHint(hint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.migrationHint = hint
}

func (s *AppState) SetAutoSync(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autoSync = v
}

func (s *AppState) IncLightroomMonitorError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lightroomMonitorErrors++
}

func (s *AppState) IncBackupMonitorError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backupMonitorErrors++
}

func (s *AppState) IncNetworkMonitorError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.networkMonitorErrors++
}

func (s *AppState) IncLockMonitorError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lockMonitorErrors++
}

func (s *AppState) SetLastResumeGapSeconds(seconds int) {
	if seconds < 0 {
		seconds = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastResumeGapSeconds = seconds
}

// RefreshDerivedStatus clears temporary warning state and restores status from current flags.
func (s *AppState) RefreshDerivedStatus() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recomputeDerivedStatusLocked()
}

// TrayColor returns the current tray icon color.
func (s *AppState) TrayColor() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.trayColor
}

func (s *AppState) recomputeDerivedStatusLocked() {
	switch {
	case s.syncInProgress:
		s.trayColor = "red"
		s.statusText = "Syncing..."
	case s.syncPaused:
		s.trayColor = "orange"
		s.statusText = "Sync Paused"
	case s.lightroomRunning:
		s.trayColor = "blue"
		s.statusText = "Lightroom Running"
	default:
		s.trayColor = "green"
		s.statusText = "Ready"
	}
}
