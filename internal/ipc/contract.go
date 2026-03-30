// Package ipc defines the contract between Agent and UI processes.
// Communication is over Windows named pipes.
package ipc

import "time"

// PipeName is the named pipe address for Agent ↔ UI communication.
const PipeName = `\\.\pipe\LightroomSyncIPC`

// IPC defaults.
const (
	DefaultConnectTimeout = 1500 * time.Millisecond
	DefaultRequestTimeout = 3 * time.Second
)

// Command types sent from UI to Agent.
type CommandType string

const (
	CmdPing           CommandType = "ping"
	CmdGetStatus      CommandType = "get_status"
	CmdGetConfig      CommandType = "get_config"
	CmdSaveConfig     CommandType = "save_config"
	CmdSyncNow        CommandType = "sync_now"
	CmdSyncBackup     CommandType = "sync_backup"
	CmdGetBackups     CommandType = "get_backups"
	CmdSubscribeLogs  CommandType = "subscribe_logs"
	CmdPauseSync      CommandType = "pause_sync"
	CmdResumeSync     CommandType = "resume_sync"
	CmdCheckUpdate    CommandType = "check_update"
	CmdDownloadUpdate CommandType = "download_update"
)

// Request is a message from UI to Agent.
type Request struct {
	ID      string      `json:"id"`
	Command CommandType `json:"command"`
	Payload any         `json:"payload,omitempty"`
}

// Response is a message from Agent to UI.
type Response struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Code    string `json:"code,omitempty"`
}

// Event is a push message from Agent to UI (for log stream, status changes, etc.).
type Event struct {
	Type    EventType `json:"type"`
	Payload any       `json:"payload,omitempty"`
}

// EventType for pushed events.
type EventType string

const (
	EventStatusChanged  EventType = "status:changed"
	EventLogEntry       EventType = "log:entry"
	EventSyncStarted    EventType = "sync:started"
	EventSyncCompleted  EventType = "sync:completed"
	EventSyncFailed     EventType = "sync:failed"
	EventUpdateProgress EventType = "update:progress"
)

// Standard response codes used by Agent/UI contract.
const (
	CodeOK            = "ok"
	CodeBadRequest    = "bad_request"
	CodeTimeout       = "timeout"
	CodeUnknownCmd    = "unknown_command"
	CodeInternalError = "internal_error"
	CodeAgentOffline  = "agent_offline"
)

// --- Payload types ---

// AppStatus represents the full application state snapshot.
type AppStatus struct {
	TrayColor              string `json:"tray_color"` // green, blue, orange, red
	StatusText             string `json:"status_text"`
	LightroomRunning       bool   `json:"lightroom_running"`
	SyncInProgress         bool   `json:"sync_in_progress"`
	SyncPaused             bool   `json:"sync_paused"`
	LastBackup             string `json:"last_backup,omitempty"`
	LockMachine            string `json:"lock_machine,omitempty"`
	LockStatus             string `json:"lock_status,omitempty"`
	MigrationHint          string `json:"migration_hint,omitempty"`
	LightroomMonitorErrors int    `json:"lightroom_monitor_errors"`
	BackupMonitorErrors    int    `json:"backup_monitor_errors"`
	NetworkMonitorErrors   int    `json:"network_monitor_errors"`
	LockMonitorErrors      int    `json:"lock_monitor_errors"`
	LastResumeGapSeconds   int    `json:"last_resume_gap_seconds"`
	AutoSync               bool   `json:"auto_sync"`
}

// BackupInfo describes a single backup zip file.
type BackupInfo struct {
	Path        string    `json:"path"`
	CatalogName string    `json:"catalog_name"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"mod_time"`
}

// LogEntry represents a single log line.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // INFO, WARNING, ERROR, DEBUG
	Message   string    `json:"message"`
}

// StreamLogEntry is a buffered log line with monotonically increasing ID.
type StreamLogEntry struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // INFO, WARN, ERROR, DEBUG
	Message   string    `json:"message"`
}

// SubscribeLogsPayload requests buffered logs after a cursor ID.
type SubscribeLogsPayload struct {
	AfterID int64 `json:"after_id,omitempty"`
	Limit   int   `json:"limit,omitempty"`
}

// SubscribeLogsResult is the subscribe_logs IPC response payload.
type SubscribeLogsResult struct {
	Entries []StreamLogEntry `json:"entries"`
	LastID  int64            `json:"last_id"`
}

// SyncBackupPayload is the payload for CmdSyncBackup.
type SyncBackupPayload struct {
	ZipPath string `json:"zip_path"`
}

// ConfigSnapshot is the Agent config returned to UI callers.
type ConfigSnapshot struct {
	BackupFolder        string   `json:"backup_folder"`
	CatalogPath         string   `json:"catalog_path"`
	StartWithWindows    bool     `json:"start_with_windows"`
	StartMinimized      bool     `json:"start_minimized"`
	MinimizeToTray      bool     `json:"minimize_to_tray"`
	AutoSync            bool     `json:"auto_sync"`
	HeartbeatInterval   int      `json:"heartbeat_interval"`
	CheckInterval       int      `json:"check_interval"`
	LockTimeout         int      `json:"lock_timeout"`
	MaxCatalogBackups   int      `json:"max_catalog_backups"`
	PresetSyncEnabled   bool     `json:"preset_sync_enabled"`
	PresetCategories    []string `json:"preset_categories"`
	LastSyncedTimestamp string   `json:"last_synced_timestamp"`
}

// SaveConfigPayload is a partial config update payload.
// nil fields mean "keep existing value".
type SaveConfigPayload struct {
	BackupFolder        *string   `json:"backup_folder,omitempty"`
	CatalogPath         *string   `json:"catalog_path,omitempty"`
	StartWithWindows    *bool     `json:"start_with_windows,omitempty"`
	StartMinimized      *bool     `json:"start_minimized,omitempty"`
	MinimizeToTray      *bool     `json:"minimize_to_tray,omitempty"`
	AutoSync            *bool     `json:"auto_sync,omitempty"`
	HeartbeatInterval   *int      `json:"heartbeat_interval,omitempty"`
	CheckInterval       *int      `json:"check_interval,omitempty"`
	LockTimeout         *int      `json:"lock_timeout,omitempty"`
	MaxCatalogBackups   *int      `json:"max_catalog_backups,omitempty"`
	PresetSyncEnabled   *bool     `json:"preset_sync_enabled,omitempty"`
	PresetCategories    *[]string `json:"preset_categories,omitempty"`
	LastSyncedTimestamp *string   `json:"last_synced_timestamp,omitempty"`
}
