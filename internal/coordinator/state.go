package coordinator

import (
	"sync"

	"github.com/ngojclee/lightroom-sync/internal/ipc"
)

// AppState is the single authoritative state cache for the Agent.
// Both tray and IPC reads come from here — no direct filesystem queries from UI.
type AppState struct {
	mu sync.RWMutex

	lightroomRunning bool
	syncInProgress   bool
	syncPaused       bool
	trayColor        string // green, blue, orange, red
	statusText       string
	lastBackup       string
	lockMachine      string
	lockStatus       string
	autoSync         bool
}

func NewAppState() *AppState {
	return &AppState{
		trayColor:  "green",
		statusText: "Sẵn sàng",
	}
}

// Snapshot returns a copy of the current state for IPC responses.
func (s *AppState) Snapshot() ipc.AppStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ipc.AppStatus{
		TrayColor:        s.trayColor,
		StatusText:       s.statusText,
		LightroomRunning: s.lightroomRunning,
		SyncInProgress:   s.syncInProgress,
		SyncPaused:       s.syncPaused,
		LastBackup:       s.lastBackup,
		LockMachine:      s.lockMachine,
		LockStatus:       s.lockStatus,
		AutoSync:         s.autoSync,
	}
}

func (s *AppState) SetLightroomRunning(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lightroomRunning = v
	if v {
		s.trayColor = "blue"
		s.statusText = "Lightroom đang chạy"
	} else {
		s.trayColor = "green"
		s.statusText = "Sẵn sàng"
	}
}

func (s *AppState) SetSyncing(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncInProgress = v
	if v {
		s.trayColor = "red"
		s.statusText = "Đang đồng bộ..."
		return
	}

	if s.syncPaused {
		s.trayColor = "orange"
		s.statusText = "Đã tạm dừng đồng bộ"
		return
	}

	if s.lightroomRunning {
		s.trayColor = "blue"
		s.statusText = "Lightroom đang chạy"
		return
	}

	s.trayColor = "green"
	s.statusText = "Sẵn sàng"
}

func (s *AppState) SetSyncPaused(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncPaused = v
	if v {
		s.statusText = "Đã tạm dừng đồng bộ"
	}
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

func (s *AppState) SetAutoSync(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autoSync = v
}

// TrayColor returns the current tray icon color.
func (s *AppState) TrayColor() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.trayColor
}
