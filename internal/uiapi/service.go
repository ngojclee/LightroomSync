package uiapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ngojclee/lightroom-sync/internal/ipc"
)

type callFunc func(context.Context, string, ipc.Request) (ipc.Response, error)
type pingFunc func(context.Context, string) (bool, error)

// Service provides command execution for CLI mode and Wails bindings.
type Service struct {
	PipeName string

	now    func() time.Time
	call   callFunc
	ping   pingFunc
	ctxFor func(time.Duration) (context.Context, context.CancelFunc)
}

// NewService creates a UI API service bound to a named pipe.
func NewService(pipeName string) *Service {
	return &Service{
		PipeName: pipeName,
		now:      time.Now,
		call:     ipc.Call,
		ping:     ipc.Ping,
		ctxFor: func(timeout time.Duration) (context.Context, context.CancelFunc) {
			return context.WithTimeout(context.Background(), timeout)
		},
	}
}

// ExecuteAction runs a CLI/Wails action while preserving the legacy envelope shape.
func (s *Service) ExecuteAction(action, payload string) ActionEnvelope {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "ping":
		return s.Ping()
	case "status":
		return s.GetStatus()
	case "get-config":
		return s.GetConfig()
	case "save-config":
		return s.SaveConfig(payload)
	case "sync-now":
		return s.SyncNow()
	case "get-backups":
		return s.GetBackups()
	case "sync-backup":
		return s.SyncBackup(payload)
	case "pause-sync":
		return s.PauseSync()
	case "resume-sync":
		return s.ResumeSync()
	case "subscribe-logs":
		return s.SubscribeLogs(payload)
	case "check-update":
		return s.CheckUpdate()
	case "download-update":
		return s.DownloadUpdate(payload)
	default:
		return badRequestEnvelope(s.now, fmt.Sprintf("unsupported action: %s", action))
	}
}

func (s *Service) Ping() ActionEnvelope {
	ctx, cancel := s.ctxFor(1200 * time.Millisecond)
	defer cancel()

	ok, err := s.ping(ctx, s.PipeName)
	if err != nil {
		return agentOfflineEnvelope(s.now, err)
	}

	return ActionEnvelope{
		OK:      true,
		Success: ok,
		Code:    ipc.CodeOK,
		Server:  nowRFC3339(s.now),
	}
}

func (s *Service) GetStatus() ActionEnvelope {
	return s.callCommand(2*time.Second, ipc.CmdGetStatus, nil)
}

func (s *Service) GetConfig() ActionEnvelope {
	return s.callCommand(2*time.Second, ipc.CmdGetConfig, nil)
}

func (s *Service) SaveConfig(payload string) ActionEnvelope {
	body, err := parsePayloadJSON(payload)
	if err != nil {
		return badRequestEnvelope(s.now, err.Error())
	}

	return s.callCommand(3*time.Second, ipc.CmdSaveConfig, body)
}

func (s *Service) SyncNow() ActionEnvelope {
	return s.callCommand(2*time.Second, ipc.CmdSyncNow, nil)
}

func (s *Service) SyncBackup(payload string) ActionEnvelope {
	zipPath := strings.TrimSpace(payload)
	if zipPath == "" {
		return badRequestEnvelope(s.now, "payload is required for sync-backup and must contain zip path")
	}

	return s.callCommand(3*time.Second, ipc.CmdSyncBackup, ipc.SyncBackupPayload{ZipPath: zipPath})
}

func (s *Service) GetBackups() ActionEnvelope {
	return s.callCommand(3*time.Second, ipc.CmdGetBackups, nil)
}

func (s *Service) PauseSync() ActionEnvelope {
	return s.callCommand(2*time.Second, ipc.CmdPauseSync, nil)
}

func (s *Service) ResumeSync() ActionEnvelope {
	return s.callCommand(2*time.Second, ipc.CmdResumeSync, nil)
}

func (s *Service) SubscribeLogs(payload string) ActionEnvelope {
	body := ipc.SubscribeLogsPayload{
		AfterID: 0,
		Limit:   120,
	}

	payload = strings.TrimSpace(payload)
	if payload != "" {
		if err := json.Unmarshal([]byte(payload), &body); err != nil {
			return badRequestEnvelope(s.now, fmt.Sprintf("invalid payload JSON: %v", err))
		}
	}
	if body.Limit <= 0 {
		body.Limit = 120
	}

	return s.callCommand(2*time.Second, ipc.CmdSubscribeLogs, body)
}

func (s *Service) CheckUpdate() ActionEnvelope {
	return s.callCommand(3*time.Second, ipc.CmdCheckUpdate, nil)
}

func (s *Service) DownloadUpdate(payload string) ActionEnvelope {
	body := ipc.DownloadUpdatePayload{}
	payload = strings.TrimSpace(payload)
	if payload != "" {
		if err := json.Unmarshal([]byte(payload), &body); err != nil {
			return badRequestEnvelope(s.now, fmt.Sprintf("invalid payload JSON: %v", err))
		}
	}

	return s.callCommand(2*time.Second, ipc.CmdDownloadUpdate, body)
}

func (s *Service) callCommand(timeout time.Duration, command ipc.CommandType, payload any) ActionEnvelope {
	ctx, cancel := s.ctxFor(timeout)
	defer cancel()

	req := ipc.Request{Command: command}
	if payload != nil {
		req.Payload = payload
	}

	resp, err := s.call(ctx, s.PipeName, req)
	if err != nil {
		return agentOfflineEnvelope(s.now, err)
	}
	return responseEnvelope(s.now, resp)
}

func parsePayloadJSON(payload string) (map[string]any, error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return nil, fmt.Errorf("payload JSON is required for save-config")
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return nil, fmt.Errorf("invalid payload JSON: %w", err)
	}
	return body, nil
}
