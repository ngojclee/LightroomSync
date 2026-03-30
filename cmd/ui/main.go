package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/ngojclee/lightroom-sync/internal/ipc"
)

var Version = "dev"

const uiHarnessWindowTitle = "Lightroom Sync - Temporary GUI Test"

type actionEnvelope struct {
	OK      bool   `json:"ok"`
	ID      string `json:"id,omitempty"`
	Success bool   `json:"success"`
	Code    string `json:"code,omitempty"`
	Error   string `json:"error,omitempty"`
	Data    any    `json:"data,omitempty"`
	Server  string `json:"server_ts,omitempty"`
}

func main() {
	action := flag.String("action", "", "Run one IPC action and print JSON result (ping|status|get-config|save-config|get-backups|sync-now|sync-backup|pause-sync|resume-sync|subscribe-logs|check-update|download-update)")
	payload := flag.String("payload", "", "Optional JSON payload or value for action commands")
	pipeName := flag.String("pipe", ipc.PipeName, "Named pipe path for Agent IPC")
	runtimeMode := flag.String("runtime", uiRuntimeHarness, "UI runtime mode: harness|wails")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(Version)
		return
	}

	log.Printf("[INFO] LightroomSync UI %s", Version)

	if *action != "" {
		env := runAction(*action, *payload, *pipeName)
		printJSON(env)
		if env.OK {
			return
		}
		os.Exit(1)
	}

	mode := normalizeRuntimeMode(*runtimeMode)
	if mode == uiRuntimeWails {
		if err := launchWailsRuntime(*pipeName); err != nil {
			log.Printf("[ERROR] Failed to launch Wails runtime: %v", err)
			os.Exit(1)
		}
		return
	}
	if mode != uiRuntimeHarness {
		log.Printf("[WARN] Unknown runtime mode %q, falling back to %q.", mode, uiRuntimeHarness)
	}

	if runtime.GOOS != "windows" {
		log.Printf("[WARN] Temporary GUI harness currently supports Windows only. Use --action for headless checks.")
		return
	}

	guard, acquired, err := acquireUISingleInstance()
	if err != nil {
		log.Fatalf("Failed to acquire UI single-instance guard: %v", err)
	}
	if !acquired {
		if err := focusExistingUIWindow(); err != nil {
			log.Printf("[WARN] Another UI instance is running but could not be focused: %v", err)
		} else {
			log.Println("[INFO] Existing UI instance focused.")
		}
		return
	}
	defer guard.Release()

	waitCtx, cancelWait := context.WithTimeout(context.Background(), 2200*time.Millisecond)
	defer cancelWait()
	if err := ipc.WaitForAgent(waitCtx, *pipeName, 150*time.Millisecond); err != nil {
		log.Printf("[WARN] Agent not reachable at startup: %v", err)
	} else {
		log.Println("[INFO] Agent reachable. Opening temporary GUI harness...")
	}

	if err := launchWindowsHarness(*pipeName); err != nil {
		log.Printf("[ERROR] Failed to launch temporary GUI harness: %v", err)
		os.Exit(1)
	}
}

func runAction(action, payload, pipeName string) actionEnvelope {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "ping":
		return actionPing(pipeName)
	case "status":
		return actionStatus(pipeName)
	case "get-config":
		return actionGetConfig(pipeName)
	case "save-config":
		return actionSaveConfig(pipeName, payload)
	case "sync-now":
		return actionSyncNow(pipeName)
	case "get-backups":
		return actionGetBackups(pipeName)
	case "sync-backup":
		return actionSyncBackup(pipeName, payload)
	case "pause-sync":
		return actionPauseSync(pipeName)
	case "resume-sync":
		return actionResumeSync(pipeName)
	case "subscribe-logs":
		return actionSubscribeLogs(pipeName, payload)
	case "check-update":
		return actionCheckUpdate(pipeName)
	case "download-update":
		return actionDownloadUpdate(pipeName, payload)
	default:
		return actionEnvelope{
			OK:      false,
			Success: false,
			Code:    ipc.CodeBadRequest,
			Error:   fmt.Sprintf("unsupported action: %s", action),
			Server:  time.Now().Format(time.RFC3339),
		}
	}
}

func actionPing(pipeName string) actionEnvelope {
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()

	ok, err := ipc.Ping(ctx, pipeName)
	if err != nil {
		return actionEnvelope{
			OK:      false,
			Success: false,
			Code:    ipc.CodeAgentOffline,
			Error:   err.Error(),
			Server:  time.Now().Format(time.RFC3339),
		}
	}

	return actionEnvelope{
		OK:      true,
		Success: ok,
		Code:    ipc.CodeOK,
		Server:  time.Now().Format(time.RFC3339),
	}
}

func actionStatus(pipeName string) actionEnvelope {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := ipc.Call(ctx, pipeName, ipc.Request{Command: ipc.CmdGetStatus})
	if err != nil {
		return actionEnvelope{
			OK:      false,
			Success: false,
			Code:    ipc.CodeAgentOffline,
			Error:   err.Error(),
			Server:  time.Now().Format(time.RFC3339),
		}
	}

	return actionEnvelope{
		OK:      true,
		ID:      resp.ID,
		Success: resp.Success,
		Code:    resp.Code,
		Error:   resp.Error,
		Data:    resp.Data,
		Server:  time.Now().Format(time.RFC3339),
	}
}

func actionSyncNow(pipeName string) actionEnvelope {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := ipc.Call(ctx, pipeName, ipc.Request{Command: ipc.CmdSyncNow})
	if err != nil {
		return actionEnvelope{
			OK:      false,
			Success: false,
			Code:    ipc.CodeAgentOffline,
			Error:   err.Error(),
			Server:  time.Now().Format(time.RFC3339),
		}
	}

	return actionEnvelope{
		OK:      true,
		ID:      resp.ID,
		Success: resp.Success,
		Code:    resp.Code,
		Error:   resp.Error,
		Data:    resp.Data,
		Server:  time.Now().Format(time.RFC3339),
	}
}

func actionPauseSync(pipeName string) actionEnvelope {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := ipc.Call(ctx, pipeName, ipc.Request{Command: ipc.CmdPauseSync})
	if err != nil {
		return actionEnvelope{
			OK:      false,
			Success: false,
			Code:    ipc.CodeAgentOffline,
			Error:   err.Error(),
			Server:  time.Now().Format(time.RFC3339),
		}
	}

	return actionEnvelope{
		OK:      true,
		ID:      resp.ID,
		Success: resp.Success,
		Code:    resp.Code,
		Error:   resp.Error,
		Data:    resp.Data,
		Server:  time.Now().Format(time.RFC3339),
	}
}

func actionResumeSync(pipeName string) actionEnvelope {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := ipc.Call(ctx, pipeName, ipc.Request{Command: ipc.CmdResumeSync})
	if err != nil {
		return actionEnvelope{
			OK:      false,
			Success: false,
			Code:    ipc.CodeAgentOffline,
			Error:   err.Error(),
			Server:  time.Now().Format(time.RFC3339),
		}
	}

	return actionEnvelope{
		OK:      true,
		ID:      resp.ID,
		Success: resp.Success,
		Code:    resp.Code,
		Error:   resp.Error,
		Data:    resp.Data,
		Server:  time.Now().Format(time.RFC3339),
	}
}

func actionSubscribeLogs(pipeName, payload string) actionEnvelope {
	body := ipc.SubscribeLogsPayload{
		AfterID: 0,
		Limit:   120,
	}
	payload = strings.TrimSpace(payload)
	if payload != "" {
		if err := json.Unmarshal([]byte(payload), &body); err != nil {
			return actionEnvelope{
				OK:      false,
				Success: false,
				Code:    ipc.CodeBadRequest,
				Error:   fmt.Sprintf("invalid payload JSON: %v", err),
				Server:  time.Now().Format(time.RFC3339),
			}
		}
	}
	if body.Limit <= 0 {
		body.Limit = 120
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := ipc.Call(ctx, pipeName, ipc.Request{
		Command: ipc.CmdSubscribeLogs,
		Payload: body,
	})
	if err != nil {
		return actionEnvelope{
			OK:      false,
			Success: false,
			Code:    ipc.CodeAgentOffline,
			Error:   err.Error(),
			Server:  time.Now().Format(time.RFC3339),
		}
	}

	return actionEnvelope{
		OK:      true,
		ID:      resp.ID,
		Success: resp.Success,
		Code:    resp.Code,
		Error:   resp.Error,
		Data:    resp.Data,
		Server:  time.Now().Format(time.RFC3339),
	}
}

func actionCheckUpdate(pipeName string) actionEnvelope {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := ipc.Call(ctx, pipeName, ipc.Request{Command: ipc.CmdCheckUpdate})
	if err != nil {
		return actionEnvelope{
			OK:      false,
			Success: false,
			Code:    ipc.CodeAgentOffline,
			Error:   err.Error(),
			Server:  time.Now().Format(time.RFC3339),
		}
	}
	return actionEnvelope{
		OK:      true,
		ID:      resp.ID,
		Success: resp.Success,
		Code:    resp.Code,
		Error:   resp.Error,
		Data:    resp.Data,
		Server:  time.Now().Format(time.RFC3339),
	}
}

func actionDownloadUpdate(pipeName, payload string) actionEnvelope {
	body := ipc.DownloadUpdatePayload{}
	payload = strings.TrimSpace(payload)
	if payload != "" {
		if err := json.Unmarshal([]byte(payload), &body); err != nil {
			return actionEnvelope{
				OK:      false,
				Success: false,
				Code:    ipc.CodeBadRequest,
				Error:   fmt.Sprintf("invalid payload JSON: %v", err),
				Server:  time.Now().Format(time.RFC3339),
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := ipc.Call(ctx, pipeName, ipc.Request{
		Command: ipc.CmdDownloadUpdate,
		Payload: body,
	})
	if err != nil {
		return actionEnvelope{
			OK:      false,
			Success: false,
			Code:    ipc.CodeAgentOffline,
			Error:   err.Error(),
			Server:  time.Now().Format(time.RFC3339),
		}
	}
	return actionEnvelope{
		OK:      true,
		ID:      resp.ID,
		Success: resp.Success,
		Code:    resp.Code,
		Error:   resp.Error,
		Data:    resp.Data,
		Server:  time.Now().Format(time.RFC3339),
	}
}

func actionGetConfig(pipeName string) actionEnvelope {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := ipc.Call(ctx, pipeName, ipc.Request{Command: ipc.CmdGetConfig})
	if err != nil {
		return actionEnvelope{
			OK:      false,
			Success: false,
			Code:    ipc.CodeAgentOffline,
			Error:   err.Error(),
			Server:  time.Now().Format(time.RFC3339),
		}
	}
	return actionEnvelope{
		OK:      true,
		ID:      resp.ID,
		Success: resp.Success,
		Code:    resp.Code,
		Error:   resp.Error,
		Data:    resp.Data,
		Server:  time.Now().Format(time.RFC3339),
	}
}

func actionSaveConfig(pipeName, payload string) actionEnvelope {
	body, err := parsePayloadJSON(payload)
	if err != nil {
		return actionEnvelope{
			OK:      false,
			Success: false,
			Code:    ipc.CodeBadRequest,
			Error:   err.Error(),
			Server:  time.Now().Format(time.RFC3339),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := ipc.Call(ctx, pipeName, ipc.Request{
		Command: ipc.CmdSaveConfig,
		Payload: body,
	})
	if err != nil {
		return actionEnvelope{
			OK:      false,
			Success: false,
			Code:    ipc.CodeAgentOffline,
			Error:   err.Error(),
			Server:  time.Now().Format(time.RFC3339),
		}
	}

	return actionEnvelope{
		OK:      true,
		ID:      resp.ID,
		Success: resp.Success,
		Code:    resp.Code,
		Error:   resp.Error,
		Data:    resp.Data,
		Server:  time.Now().Format(time.RFC3339),
	}
}

func actionGetBackups(pipeName string) actionEnvelope {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := ipc.Call(ctx, pipeName, ipc.Request{Command: ipc.CmdGetBackups})
	if err != nil {
		return actionEnvelope{
			OK:      false,
			Success: false,
			Code:    ipc.CodeAgentOffline,
			Error:   err.Error(),
			Server:  time.Now().Format(time.RFC3339),
		}
	}
	return actionEnvelope{
		OK:      true,
		ID:      resp.ID,
		Success: resp.Success,
		Code:    resp.Code,
		Error:   resp.Error,
		Data:    resp.Data,
		Server:  time.Now().Format(time.RFC3339),
	}
}

func actionSyncBackup(pipeName, payload string) actionEnvelope {
	zipPath := strings.TrimSpace(payload)
	if zipPath == "" {
		return actionEnvelope{
			OK:      false,
			Success: false,
			Code:    ipc.CodeBadRequest,
			Error:   "payload is required for sync-backup and must contain zip path",
			Server:  time.Now().Format(time.RFC3339),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := ipc.Call(ctx, pipeName, ipc.Request{
		Command: ipc.CmdSyncBackup,
		Payload: ipc.SyncBackupPayload{ZipPath: zipPath},
	})
	if err != nil {
		return actionEnvelope{
			OK:      false,
			Success: false,
			Code:    ipc.CodeAgentOffline,
			Error:   err.Error(),
			Server:  time.Now().Format(time.RFC3339),
		}
	}
	return actionEnvelope{
		OK:      true,
		ID:      resp.ID,
		Success: resp.Success,
		Code:    resp.Code,
		Error:   resp.Error,
		Data:    resp.Data,
		Server:  time.Now().Format(time.RFC3339),
	}
}

func parsePayloadJSON(payload string) (map[string]any, error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return nil, errors.New("payload JSON is required for save-config")
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(payload), &body); err != nil {
		return nil, fmt.Errorf("invalid payload JSON: %w", err)
	}
	return body, nil
}

func printJSON(payload any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}

func launchWindowsHarness(pipeName string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve ui executable path: %w", err)
	}

	script := windowsHarnessScript(exePath, pipeName)
	var lastErr error
	for _, shellName := range []string{"pwsh", "powershell"} {
		cmd := exec.Command(shellName, "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err == nil {
			return nil
		}

		lastErr = err
		if errors.Is(err, exec.ErrNotFound) {
			continue
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no PowerShell host found (pwsh/powershell)")
}

func windowsHarnessScript(exePath, pipeName string) string {
	escapedExe := strings.ReplaceAll(exePath, "'", "''")
	escapedPipe := strings.ReplaceAll(pipeName, "'", "''")
	escapedVersion := strings.ReplaceAll(Version, "'", "''")

	return fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
[System.Windows.Forms.Application]::EnableVisualStyles()

$exe = '%s'
$pipe = '%s'
$currentVersion = '%s'

$form = New-Object System.Windows.Forms.Form
$form.Text = '%s'
$form.Width = 980
$form.Height = 720
$form.StartPosition = 'CenterScreen'
$form.BackColor = [System.Drawing.Color]::FromArgb(245, 248, 255)

$title = New-Object System.Windows.Forms.Label
$title.Text = 'Lightroom Sync - GUI Test Harness'
$title.Font = New-Object System.Drawing.Font('Segoe UI', 16, [System.Drawing.FontStyle]::Bold)
$title.AutoSize = $true
$title.Location = New-Object System.Drawing.Point(20, 16)
$form.Controls.Add($title)

$subtitle = New-Object System.Windows.Forms.Label
$subtitle.Text = 'Temporary Windows Forms GUI for testing Agent IPC before full Wails UI.'
$subtitle.Font = New-Object System.Drawing.Font('Segoe UI', 9)
$subtitle.AutoSize = $true
$subtitle.ForeColor = [System.Drawing.Color]::FromArgb(88, 114, 156)
$subtitle.Location = New-Object System.Drawing.Point(22, 46)
$form.Controls.Add($subtitle)

$btnPing = New-Object System.Windows.Forms.Button
$btnPing.Text = 'Ping Agent'
$btnPing.Width = 100
$btnPing.Height = 36
$btnPing.Location = New-Object System.Drawing.Point(22, 80)
$form.Controls.Add($btnPing)

$btnStatus = New-Object System.Windows.Forms.Button
$btnStatus.Text = 'Refresh Status'
$btnStatus.Width = 120
$btnStatus.Height = 36
$btnStatus.Location = New-Object System.Drawing.Point(128, 80)
$form.Controls.Add($btnStatus)

$btnGetConfig = New-Object System.Windows.Forms.Button
$btnGetConfig.Text = 'Get Config'
$btnGetConfig.Width = 110
$btnGetConfig.Height = 36
$btnGetConfig.Location = New-Object System.Drawing.Point(254, 80)
$form.Controls.Add($btnGetConfig)

$btnGetBackups = New-Object System.Windows.Forms.Button
$btnGetBackups.Text = 'Get Backups'
$btnGetBackups.Width = 115
$btnGetBackups.Height = 36
$btnGetBackups.Location = New-Object System.Drawing.Point(370, 80)
$form.Controls.Add($btnGetBackups)

$btnSyncNow = New-Object System.Windows.Forms.Button
$btnSyncNow.Text = 'Sync Now'
$btnSyncNow.Width = 100
$btnSyncNow.Height = 36
$btnSyncNow.Location = New-Object System.Drawing.Point(491, 80)
$btnSyncNow.BackColor = [System.Drawing.Color]::FromArgb(11, 138, 106)
$btnSyncNow.ForeColor = [System.Drawing.Color]::White
$form.Controls.Add($btnSyncNow)

$btnPauseSync = New-Object System.Windows.Forms.Button
$btnPauseSync.Text = 'Pause Sync'
$btnPauseSync.Width = 95
$btnPauseSync.Height = 36
$btnPauseSync.Location = New-Object System.Drawing.Point(597, 80)
$btnPauseSync.BackColor = [System.Drawing.Color]::FromArgb(209, 117, 41)
$btnPauseSync.ForeColor = [System.Drawing.Color]::White
$form.Controls.Add($btnPauseSync)

$btnResumeSync = New-Object System.Windows.Forms.Button
$btnResumeSync.Text = 'Resume'
$btnResumeSync.Width = 95
$btnResumeSync.Height = 36
$btnResumeSync.Location = New-Object System.Drawing.Point(698, 80)
$btnResumeSync.BackColor = [System.Drawing.Color]::FromArgb(52, 122, 189)
$btnResumeSync.ForeColor = [System.Drawing.Color]::White
$form.Controls.Add($btnResumeSync)

$btnClose = New-Object System.Windows.Forms.Button
$btnClose.Text = 'Close'
$btnClose.Width = 70
$btnClose.Height = 36
$btnClose.Location = New-Object System.Drawing.Point(799, 80)
$form.Controls.Add($btnClose)

$btnSaveConfig = New-Object System.Windows.Forms.Button
$btnSaveConfig.Text = 'Save Config'
$btnSaveConfig.Width = 95
$btnSaveConfig.Height = 30
$btnSaveConfig.Location = New-Object System.Drawing.Point(870, 84)
$form.Controls.Add($btnSaveConfig)

$btnResumeSync.Enabled = $false

$lblReachTitle = New-Object System.Windows.Forms.Label
$lblReachTitle.Text = 'Agent Reachable:'
$lblReachTitle.AutoSize = $true
$lblReachTitle.Location = New-Object System.Drawing.Point(22, 132)
$form.Controls.Add($lblReachTitle)

$lblReachValue = New-Object System.Windows.Forms.Label
$lblReachValue.Text = 'Unknown'
$lblReachValue.AutoSize = $true
$lblReachValue.Font = New-Object System.Drawing.Font('Segoe UI', 9, [System.Drawing.FontStyle]::Bold)
$lblReachValue.Location = New-Object System.Drawing.Point(130, 132)
$form.Controls.Add($lblReachValue)

$lblStatusTitle = New-Object System.Windows.Forms.Label
$lblStatusTitle.Text = 'Status:'
$lblStatusTitle.AutoSize = $true
$lblStatusTitle.Location = New-Object System.Drawing.Point(22, 156)
$form.Controls.Add($lblStatusTitle)

$lblStatusValue = New-Object System.Windows.Forms.Label
$lblStatusValue.Text = '-'
$lblStatusValue.AutoSize = $true
$lblStatusValue.Font = New-Object System.Drawing.Font('Segoe UI', 9, [System.Drawing.FontStyle]::Bold)
$lblStatusValue.Location = New-Object System.Drawing.Point(130, 156)
$form.Controls.Add($lblStatusValue)

$lblTrayTitle = New-Object System.Windows.Forms.Label
$lblTrayTitle.Text = 'Tray Color:'
$lblTrayTitle.AutoSize = $true
$lblTrayTitle.Location = New-Object System.Drawing.Point(22, 180)
$form.Controls.Add($lblTrayTitle)

$lblTrayValue = New-Object System.Windows.Forms.Label
$lblTrayValue.Text = '-'
$lblTrayValue.AutoSize = $true
$lblTrayValue.Font = New-Object System.Drawing.Font('Segoe UI', 9, [System.Drawing.FontStyle]::Bold)
$lblTrayValue.Location = New-Object System.Drawing.Point(130, 180)
$form.Controls.Add($lblTrayValue)

$lblCurrentVersionTitle = New-Object System.Windows.Forms.Label
$lblCurrentVersionTitle.Text = 'Current Version:'
$lblCurrentVersionTitle.AutoSize = $true
$lblCurrentVersionTitle.Location = New-Object System.Drawing.Point(22, 206)
$form.Controls.Add($lblCurrentVersionTitle)

$lblCurrentVersionValue = New-Object System.Windows.Forms.Label
$lblCurrentVersionValue.Text = $currentVersion
$lblCurrentVersionValue.AutoSize = $true
$lblCurrentVersionValue.Font = New-Object System.Drawing.Font('Segoe UI', 9, [System.Drawing.FontStyle]::Bold)
$lblCurrentVersionValue.Location = New-Object System.Drawing.Point(130, 206)
$form.Controls.Add($lblCurrentVersionValue)

$lblLatestVersionTitle = New-Object System.Windows.Forms.Label
$lblLatestVersionTitle.Text = 'Latest Version:'
$lblLatestVersionTitle.AutoSize = $true
$lblLatestVersionTitle.Location = New-Object System.Drawing.Point(22, 228)
$form.Controls.Add($lblLatestVersionTitle)

$lblLatestVersionValue = New-Object System.Windows.Forms.Label
$lblLatestVersionValue.Text = '-'
$lblLatestVersionValue.AutoSize = $true
$lblLatestVersionValue.Font = New-Object System.Drawing.Font('Segoe UI', 9, [System.Drawing.FontStyle]::Bold)
$lblLatestVersionValue.Location = New-Object System.Drawing.Point(130, 228)
$form.Controls.Add($lblLatestVersionValue)

$btnCheckUpdate = New-Object System.Windows.Forms.Button
$btnCheckUpdate.Text = 'Check Update'
$btnCheckUpdate.Width = 120
$btnCheckUpdate.Height = 28
$btnCheckUpdate.Location = New-Object System.Drawing.Point(22, 252)
$form.Controls.Add($btnCheckUpdate)

$btnDownloadUpdate = New-Object System.Windows.Forms.Button
$btnDownloadUpdate.Text = 'Download Update'
$btnDownloadUpdate.Width = 130
$btnDownloadUpdate.Height = 28
$btnDownloadUpdate.Location = New-Object System.Drawing.Point(148, 252)
$btnDownloadUpdate.Enabled = $false
$form.Controls.Add($btnDownloadUpdate)

$txtReleaseNotes = New-Object System.Windows.Forms.TextBox
$txtReleaseNotes.Multiline = $true
$txtReleaseNotes.ReadOnly = $true
$txtReleaseNotes.ScrollBars = 'Vertical'
$txtReleaseNotes.Font = New-Object System.Drawing.Font('Segoe UI', 8.5)
$txtReleaseNotes.Location = New-Object System.Drawing.Point(22, 282)
$txtReleaseNotes.Size = New-Object System.Drawing.Size(440, 42)
$form.Controls.Add($txtReleaseNotes)

$lblBackupRootTitle = New-Object System.Windows.Forms.Label
$lblBackupRootTitle.Text = 'Backup Folder:'
$lblBackupRootTitle.AutoSize = $true
$lblBackupRootTitle.Location = New-Object System.Drawing.Point(300, 132)
$form.Controls.Add($lblBackupRootTitle)

$txtBackupFolder = New-Object System.Windows.Forms.TextBox
$txtBackupFolder.Location = New-Object System.Drawing.Point(390, 128)
$txtBackupFolder.Width = 572
$form.Controls.Add($txtBackupFolder)

$lblCatalogTitle = New-Object System.Windows.Forms.Label
$lblCatalogTitle.Text = 'Catalog Path:'
$lblCatalogTitle.AutoSize = $true
$lblCatalogTitle.Location = New-Object System.Drawing.Point(300, 156)
$form.Controls.Add($lblCatalogTitle)

$txtCatalogPath = New-Object System.Windows.Forms.TextBox
$txtCatalogPath.Location = New-Object System.Drawing.Point(390, 152)
$txtCatalogPath.Width = 572
$form.Controls.Add($txtCatalogPath)

$chkStartWithWindows = New-Object System.Windows.Forms.CheckBox
$chkStartWithWindows.Text = 'Start with Windows'
$chkStartWithWindows.AutoSize = $true
$chkStartWithWindows.Location = New-Object System.Drawing.Point(390, 181)
$form.Controls.Add($chkStartWithWindows)

$chkStartMinimized = New-Object System.Windows.Forms.CheckBox
$chkStartMinimized.Text = 'Start Minimized'
$chkStartMinimized.AutoSize = $true
$chkStartMinimized.Location = New-Object System.Drawing.Point(526, 181)
$form.Controls.Add($chkStartMinimized)

$chkMinimizeToTray = New-Object System.Windows.Forms.CheckBox
$chkMinimizeToTray.Text = 'Minimize To Tray'
$chkMinimizeToTray.AutoSize = $true
$chkMinimizeToTray.Location = New-Object System.Drawing.Point(646, 181)
$form.Controls.Add($chkMinimizeToTray)

$chkAutoSync = New-Object System.Windows.Forms.CheckBox
$chkAutoSync.Text = 'Auto Sync'
$chkAutoSync.AutoSize = $true
$chkAutoSync.Location = New-Object System.Drawing.Point(766, 181)
$form.Controls.Add($chkAutoSync)

$chkPresetSyncEnabled = New-Object System.Windows.Forms.CheckBox
$chkPresetSyncEnabled.Text = 'Preset Sync'
$chkPresetSyncEnabled.AutoSize = $true
$chkPresetSyncEnabled.Location = New-Object System.Drawing.Point(850, 181)
$form.Controls.Add($chkPresetSyncEnabled)

$lblHeartbeatTitle = New-Object System.Windows.Forms.Label
$lblHeartbeatTitle.Text = 'Heartbeat:'
$lblHeartbeatTitle.AutoSize = $true
$lblHeartbeatTitle.Location = New-Object System.Drawing.Point(390, 206)
$form.Controls.Add($lblHeartbeatTitle)

$numHeartbeat = New-Object System.Windows.Forms.NumericUpDown
$numHeartbeat.Minimum = 1
$numHeartbeat.Maximum = 3600
$numHeartbeat.Location = New-Object System.Drawing.Point(460, 202)
$numHeartbeat.Width = 56
$form.Controls.Add($numHeartbeat)

$lblCheckIntervalTitle = New-Object System.Windows.Forms.Label
$lblCheckIntervalTitle.Text = 'Check:'
$lblCheckIntervalTitle.AutoSize = $true
$lblCheckIntervalTitle.Location = New-Object System.Drawing.Point(526, 206)
$form.Controls.Add($lblCheckIntervalTitle)

$numCheckInterval = New-Object System.Windows.Forms.NumericUpDown
$numCheckInterval.Minimum = 1
$numCheckInterval.Maximum = 3600
$numCheckInterval.Location = New-Object System.Drawing.Point(575, 202)
$numCheckInterval.Width = 56
$form.Controls.Add($numCheckInterval)

$lblLockTimeoutTitle = New-Object System.Windows.Forms.Label
$lblLockTimeoutTitle.Text = 'Lock Timeout:'
$lblLockTimeoutTitle.AutoSize = $true
$lblLockTimeoutTitle.Location = New-Object System.Drawing.Point(646, 206)
$form.Controls.Add($lblLockTimeoutTitle)

$numLockTimeout = New-Object System.Windows.Forms.NumericUpDown
$numLockTimeout.Minimum = 1
$numLockTimeout.Maximum = 7200
$numLockTimeout.Location = New-Object System.Drawing.Point(728, 202)
$numLockTimeout.Width = 56
$form.Controls.Add($numLockTimeout)

$lblMaxBackupsTitle = New-Object System.Windows.Forms.Label
$lblMaxBackupsTitle.Text = 'Max Backups:'
$lblMaxBackupsTitle.AutoSize = $true
$lblMaxBackupsTitle.Location = New-Object System.Drawing.Point(796, 206)
$form.Controls.Add($lblMaxBackupsTitle)

$numMaxBackups = New-Object System.Windows.Forms.NumericUpDown
$numMaxBackups.Minimum = 1
$numMaxBackups.Maximum = 500
$numMaxBackups.Location = New-Object System.Drawing.Point(884, 202)
$numMaxBackups.Width = 78
$form.Controls.Add($numMaxBackups)

$lblPresetCategoriesTitle = New-Object System.Windows.Forms.Label
$lblPresetCategoriesTitle.Text = 'Preset Categories:'
$lblPresetCategoriesTitle.AutoSize = $true
$lblPresetCategoriesTitle.Location = New-Object System.Drawing.Point(300, 231)
$form.Controls.Add($lblPresetCategoriesTitle)

$txtPresetCategories = New-Object System.Windows.Forms.TextBox
$txtPresetCategories.Location = New-Object System.Drawing.Point(410, 227)
$txtPresetCategories.Width = 552
$form.Controls.Add($txtPresetCategories)

$lblSyncPathTitle = New-Object System.Windows.Forms.Label
$lblSyncPathTitle.Text = 'Selected Zip:'
$lblSyncPathTitle.AutoSize = $true
$lblSyncPathTitle.Location = New-Object System.Drawing.Point(300, 257)
$form.Controls.Add($lblSyncPathTitle)

$txtSyncPath = New-Object System.Windows.Forms.TextBox
$txtSyncPath.Location = New-Object System.Drawing.Point(390, 253)
$txtSyncPath.Width = 462
$form.Controls.Add($txtSyncPath)

$btnSyncSelected = New-Object System.Windows.Forms.Button
$btnSyncSelected.Text = 'Sync Selected'
$btnSyncSelected.Width = 110
$btnSyncSelected.Height = 28
$btnSyncSelected.Location = New-Object System.Drawing.Point(854, 251)
$form.Controls.Add($btnSyncSelected)

$lstBackups = New-Object System.Windows.Forms.ListBox
$lstBackups.Font = New-Object System.Drawing.Font('Consolas', 8.5)
$lstBackups.Location = New-Object System.Drawing.Point(22, 332)
$lstBackups.Size = New-Object System.Drawing.Size(440, 312)
$form.Controls.Add($lstBackups)

$lblOutputTitle = New-Object System.Windows.Forms.Label
$lblOutputTitle.Text = 'Action Output'
$lblOutputTitle.AutoSize = $true
$lblOutputTitle.Location = New-Object System.Drawing.Point(474, 332)
$form.Controls.Add($lblOutputTitle)

$txtOutput = New-Object System.Windows.Forms.TextBox
$txtOutput.Multiline = $true
$txtOutput.ReadOnly = $true
$txtOutput.ScrollBars = 'Vertical'
$txtOutput.Font = New-Object System.Drawing.Font('Consolas', 9)
$txtOutput.Location = New-Object System.Drawing.Point(474, 350)
$txtOutput.Size = New-Object System.Drawing.Size(488, 110)
$form.Controls.Add($txtOutput)

$lblLogsTitle = New-Object System.Windows.Forms.Label
$lblLogsTitle.Text = 'Agent Logs (subscribe_logs)'
$lblLogsTitle.AutoSize = $true
$lblLogsTitle.Location = New-Object System.Drawing.Point(474, 468)
$form.Controls.Add($lblLogsTitle)

$lblLogLevel = New-Object System.Windows.Forms.Label
$lblLogLevel.Text = 'Level:'
$lblLogLevel.AutoSize = $true
$lblLogLevel.Location = New-Object System.Drawing.Point(760, 468)
$form.Controls.Add($lblLogLevel)

$cmbLogLevel = New-Object System.Windows.Forms.ComboBox
$cmbLogLevel.DropDownStyle = 'DropDownList'
[void]$cmbLogLevel.Items.Add('ALL')
[void]$cmbLogLevel.Items.Add('INFO')
[void]$cmbLogLevel.Items.Add('WARN')
[void]$cmbLogLevel.Items.Add('ERROR')
[void]$cmbLogLevel.Items.Add('DEBUG')
$cmbLogLevel.SelectedIndex = 0
$cmbLogLevel.Location = New-Object System.Drawing.Point(805, 464)
$cmbLogLevel.Width = 90
$form.Controls.Add($cmbLogLevel)

$btnClearLogs = New-Object System.Windows.Forms.Button
$btnClearLogs.Text = 'Clear'
$btnClearLogs.Width = 60
$btnClearLogs.Height = 26
$btnClearLogs.Location = New-Object System.Drawing.Point(902, 462)
$form.Controls.Add($btnClearLogs)

$txtLogs = New-Object System.Windows.Forms.TextBox
$txtLogs.Multiline = $true
$txtLogs.ReadOnly = $true
$txtLogs.ScrollBars = 'Vertical'
$txtLogs.Font = New-Object System.Drawing.Font('Consolas', 8.8)
$txtLogs.Location = New-Object System.Drawing.Point(474, 486)
$txtLogs.Size = New-Object System.Drawing.Size(488, 158)
$form.Controls.Add($txtLogs)

$script:lastLogID = 0
$script:updateAssetURL = ''
$script:updateAssetName = ''

function Set-Reachability([bool]$ok) {
    if ($ok) {
        $lblReachValue.Text = 'Yes'
        $lblReachValue.ForeColor = [System.Drawing.Color]::FromArgb(46, 139, 87)
    } else {
        $lblReachValue.Text = 'No'
        $lblReachValue.ForeColor = [System.Drawing.Color]::FromArgb(199, 59, 59)
    }
}

function Render-Config($cfg) {
    if (-not $cfg) { return }
    if ($null -ne $cfg.backup_folder) {
        $txtBackupFolder.Text = [string]$cfg.backup_folder
    } else {
        $txtBackupFolder.Text = ''
    }
    if ($null -ne $cfg.catalog_path) {
        $txtCatalogPath.Text = [string]$cfg.catalog_path
    } else {
        $txtCatalogPath.Text = ''
    }
    if ($null -ne $cfg.start_with_windows) {
        $chkStartWithWindows.Checked = [bool]$cfg.start_with_windows
    }
    if ($null -ne $cfg.start_minimized) {
        $chkStartMinimized.Checked = [bool]$cfg.start_minimized
    }
    if ($null -ne $cfg.minimize_to_tray) {
        $chkMinimizeToTray.Checked = [bool]$cfg.minimize_to_tray
    }
    if ($null -ne $cfg.auto_sync) {
        $chkAutoSync.Checked = [bool]$cfg.auto_sync
    }
    if ($null -ne $cfg.preset_sync_enabled) {
        $chkPresetSyncEnabled.Checked = [bool]$cfg.preset_sync_enabled
    }
    if ($cfg.preset_categories) {
        if ($cfg.preset_categories -is [System.Array]) {
            $txtPresetCategories.Text = [string]::Join(', ', [string[]]$cfg.preset_categories)
        } else {
            $txtPresetCategories.Text = [string]$cfg.preset_categories
        }
    } else {
        $txtPresetCategories.Text = ''
    }

    if ($null -ne $cfg.heartbeat_interval) {
        $hb = [int]$cfg.heartbeat_interval
        if ($hb -lt [int]$numHeartbeat.Minimum) { $hb = [int]$numHeartbeat.Minimum }
        if ($hb -gt [int]$numHeartbeat.Maximum) { $hb = [int]$numHeartbeat.Maximum }
        $numHeartbeat.Value = [decimal]$hb
    }
    if ($null -ne $cfg.check_interval) {
        $ci = [int]$cfg.check_interval
        if ($ci -lt [int]$numCheckInterval.Minimum) { $ci = [int]$numCheckInterval.Minimum }
        if ($ci -gt [int]$numCheckInterval.Maximum) { $ci = [int]$numCheckInterval.Maximum }
        $numCheckInterval.Value = [decimal]$ci
    }
    if ($null -ne $cfg.lock_timeout) {
        $lt = [int]$cfg.lock_timeout
        if ($lt -lt [int]$numLockTimeout.Minimum) { $lt = [int]$numLockTimeout.Minimum }
        if ($lt -gt [int]$numLockTimeout.Maximum) { $lt = [int]$numLockTimeout.Maximum }
        $numLockTimeout.Value = [decimal]$lt
    }
    if ($null -ne $cfg.max_catalog_backups) {
        $mb = [int]$cfg.max_catalog_backups
        if ($mb -lt [int]$numMaxBackups.Minimum) { $mb = [int]$numMaxBackups.Minimum }
        if ($mb -gt [int]$numMaxBackups.Maximum) { $mb = [int]$numMaxBackups.Maximum }
        $numMaxBackups.Value = [decimal]$mb
    }
}

function Render-Backups($items) {
    $lstBackups.Items.Clear()
    if (-not $items) { return }
    foreach ($item in $items) {
        if ($item.path) {
            [void]$lstBackups.Items.Add([string]$item.path)
        }
    }
}

function Render-LogEntries($items) {
    if (-not $items) { return }
    foreach ($item in $items) {
        $idText = '?'
        if ($null -ne $item.id) { $idText = [string]$item.id }
        $levelText = 'INFO'
        if ($item.level) { $levelText = [string]$item.level }
        $timestampText = '-'
        if ($item.timestamp) { $timestampText = [string]$item.timestamp }
        $messageText = ''
        if ($item.message) { $messageText = [string]$item.message }

        $line = ('#{0} [{1}] {2} {3}' -f $idText, $levelText, $timestampText, $messageText)
        $txtLogs.AppendText($line + [Environment]::NewLine)
    }

    $lines = $txtLogs.Lines
    if ($lines.Count -gt 500) {
        $start = $lines.Count - 400
        if ($start -lt 0) { $start = 0 }
        $txtLogs.Lines = $lines[$start..($lines.Count - 1)]
    }
    $txtLogs.SelectionStart = $txtLogs.TextLength
    $txtLogs.ScrollToCaret()
}

function Pull-Logs() {
    $payloadObj = @{
        after_id = [int64]$script:lastLogID
        limit = 120
    }
    $selectedLevel = [string]$cmbLogLevel.SelectedItem
    if (-not [string]::IsNullOrWhiteSpace($selectedLevel) -and $selectedLevel -ne 'ALL') {
        $payloadObj.level = $selectedLevel
    }
    $payloadJson = $payloadObj | ConvertTo-Json -Compress
    Invoke-Action 'subscribe-logs' $payloadJson $false
}

function Invoke-Action([string]$action, [string]$payload = '', [bool]$renderRaw = $true) {
    try {
        if ([string]::IsNullOrWhiteSpace($payload)) {
            $raw = & $exe --action $action --pipe $pipe 2>&1 | Out-String
        } else {
            $raw = & $exe --action $action --pipe $pipe --payload $payload 2>&1 | Out-String
        }
        if ($renderRaw) {
            $txtOutput.Text = $raw.Trim()
        }

        try {
            $obj = $raw | ConvertFrom-Json
            if ($obj -and $null -ne $obj.ok) {
                Set-Reachability([bool]$obj.ok)
            }

            if ($obj.data) {
                if ($obj.data.status_text) { $lblStatusValue.Text = [string]$obj.data.status_text }
                if ($obj.data.tray_color) { $lblTrayValue.Text = [string]$obj.data.tray_color }
                if ($null -ne $obj.data.sync_paused) {
                    $paused = [bool]$obj.data.sync_paused
                    $btnPauseSync.Enabled = -not $paused
                    $btnResumeSync.Enabled = $paused
                }
                if ($null -ne $obj.data.current_version) {
                    $lblCurrentVersionValue.Text = [string]$obj.data.current_version
                }
                if ($null -ne $obj.data.latest_version) {
                    $lblLatestVersionValue.Text = [string]$obj.data.latest_version
                }
                if ($null -ne $obj.data.release_notes) {
                    $txtReleaseNotes.Text = [string]$obj.data.release_notes
                }
                if ($null -ne $obj.data.asset_url) {
                    $script:updateAssetURL = [string]$obj.data.asset_url
                }
                if ($null -ne $obj.data.asset_name) {
                    $script:updateAssetName = [string]$obj.data.asset_name
                }
                if ($null -ne $obj.data.has_update) {
                    $hasUpdate = [bool]$obj.data.has_update
                    $canDownload = $hasUpdate -and (-not [string]::IsNullOrWhiteSpace($script:updateAssetURL))
                    if ($null -ne $obj.data.download_in_progress) {
                        $inProgress = [bool]$obj.data.download_in_progress
                        $btnDownloadUpdate.Enabled = $canDownload -and (-not $inProgress)
                    } else {
                        $btnDownloadUpdate.Enabled = $canDownload
                    }
                } elseif ($null -ne $obj.data.download_in_progress) {
                    $btnDownloadUpdate.Enabled = (-not [bool]$obj.data.download_in_progress) -and (-not [string]::IsNullOrWhiteSpace($script:updateAssetURL))
                }
                if (
                    $null -ne $obj.data.auto_sync -or
                    $null -ne $obj.data.backup_folder -or
                    $null -ne $obj.data.catalog_path -or
                    $null -ne $obj.data.heartbeat_interval -or
                    $null -ne $obj.data.check_interval -or
                    $null -ne $obj.data.lock_timeout -or
                    $null -ne $obj.data.max_catalog_backups
                ) {
                    Render-Config $obj.data
                }
                if ($obj.data -is [System.Array] -and $obj.data.Count -gt 0 -and $obj.data[0].path) {
                    Render-Backups $obj.data
                }
                if ($obj.data.entries) {
                    Render-LogEntries $obj.data.entries
                    if ($null -ne $obj.data.last_id) {
                        $script:lastLogID = [int64]$obj.data.last_id
                    }
                }
            } elseif ($obj.error) {
                $lblStatusValue.Text = [string]$obj.error
                $lblTrayValue.Text = '-'
            }
        } catch {
            if ($renderRaw) {
                Set-Reachability($false)
                $lblStatusValue.Text = 'Invalid JSON output'
            }
        }
    } catch {
        Set-Reachability($false)
        $lblStatusValue.Text = $_.Exception.Message
        if ($renderRaw) {
            $txtOutput.Text = $_.Exception.Message
        }
    }
}

function Parse-CategoryList([string]$raw) {
    $seen = @{}
    $list = New-Object System.Collections.Generic.List[string]
    if ([string]::IsNullOrWhiteSpace($raw)) {
        return ,$list.ToArray()
    }
    foreach ($part in ($raw -split ',')) {
        $name = [string]$part
        if ($null -eq $name) { continue }
        $name = $name.Trim()
        if ($name -eq '') { continue }
        $key = $name.ToLowerInvariant()
        if ($seen.ContainsKey($key)) { continue }
        $seen[$key] = $true
        $list.Add($name)
    }
    return ,$list.ToArray()
}

$btnPing.Add_Click({ Invoke-Action 'ping' })
$btnStatus.Add_Click({
    Invoke-Action 'status'
    Pull-Logs
})
$btnGetConfig.Add_Click({ Invoke-Action 'get-config' })
$btnGetBackups.Add_Click({ Invoke-Action 'get-backups' })
$btnSyncNow.Add_Click({
    Invoke-Action 'sync-now'
    Start-Sleep -Milliseconds 220
    Invoke-Action 'status'
    Pull-Logs
})
$btnPauseSync.Add_Click({
    Invoke-Action 'pause-sync'
    Start-Sleep -Milliseconds 120
    Invoke-Action 'status'
    Pull-Logs
})
$btnResumeSync.Add_Click({
    Invoke-Action 'resume-sync'
    Start-Sleep -Milliseconds 120
    Invoke-Action 'status'
    Pull-Logs
})
$btnCheckUpdate.Add_Click({
    Invoke-Action 'check-update'
    Pull-Logs
})
$btnDownloadUpdate.Add_Click({
    if ([string]::IsNullOrWhiteSpace($script:updateAssetURL)) {
        [System.Windows.Forms.MessageBox]::Show(
            'Chưa có asset URL. Hãy bấm Check Update trước.',
            'Download Update',
            'OK',
            'Information'
        ) | Out-Null
        return
    }
    $payloadObj = @{
        asset_url = [string]$script:updateAssetURL
        asset_name = [string]$script:updateAssetName
    }
    $payloadJson = $payloadObj | ConvertTo-Json -Compress
    Invoke-Action 'download-update' $payloadJson
    Pull-Logs
})
$btnSaveConfig.Add_Click({
    $heartbeat = [int]$numHeartbeat.Value
    $checkInterval = [int]$numCheckInterval.Value
    $lockTimeout = [int]$numLockTimeout.Value
    $maxBackups = [int]$numMaxBackups.Value
    if ($lockTimeout -lt $heartbeat) {
        [System.Windows.Forms.MessageBox]::Show(
            'Lock Timeout nên lớn hơn hoặc bằng Heartbeat.',
            'Validation',
            'OK',
            'Warning'
        ) | Out-Null
        return
    }

    $categories = Parse-CategoryList ([string]$txtPresetCategories.Text)
    if ($chkPresetSyncEnabled.Checked -and $categories.Count -eq 0) {
        [System.Windows.Forms.MessageBox]::Show(
            'Preset Sync đang bật, cần ít nhất 1 category.',
            'Validation',
            'OK',
            'Warning'
        ) | Out-Null
        return
    }

    $payloadObj = @{
        backup_folder = [string]$txtBackupFolder.Text.Trim()
        catalog_path = [string]$txtCatalogPath.Text.Trim()
        start_with_windows = [bool]$chkStartWithWindows.Checked
        start_minimized = [bool]$chkStartMinimized.Checked
        minimize_to_tray = [bool]$chkMinimizeToTray.Checked
        auto_sync = [bool]$chkAutoSync.Checked
        heartbeat_interval = $heartbeat
        check_interval = $checkInterval
        lock_timeout = $lockTimeout
        max_catalog_backups = $maxBackups
        preset_sync_enabled = [bool]$chkPresetSyncEnabled.Checked
        preset_categories = $categories
    }
    $payloadJson = $payloadObj | ConvertTo-Json -Compress
    Invoke-Action 'save-config' $payloadJson
    Start-Sleep -Milliseconds 120
    Invoke-Action 'status' '' $false
    Invoke-Action 'get-config' '' $false
    Pull-Logs
})
$btnSyncSelected.Add_Click({
    $zipPath = [string]$txtSyncPath.Text
    if ([string]::IsNullOrWhiteSpace($zipPath)) {
        [System.Windows.Forms.MessageBox]::Show('Please select or enter a backup zip path first.', 'Sync Selected', 'OK', 'Information') | Out-Null
        return
    }
    Invoke-Action 'sync-backup' $zipPath
    Start-Sleep -Milliseconds 220
    Invoke-Action 'status'
    Pull-Logs
})
$btnClose.Add_Click({ $form.Close() })
$btnClearLogs.Add_Click({
    $script:lastLogID = 0
    $txtLogs.Clear()
    Pull-Logs
})
$cmbLogLevel.Add_SelectedIndexChanged({
    $script:lastLogID = 0
    $txtLogs.Clear()
    Pull-Logs
})

$lstBackups.Add_SelectedIndexChanged({
    if ($lstBackups.SelectedItem) {
        $txtSyncPath.Text = [string]$lstBackups.SelectedItem
    }
})

$timer = New-Object System.Windows.Forms.Timer
$timer.Interval = 2500
$timer.Add_Tick({
    Invoke-Action 'status' '' $false
    Pull-Logs
})
$timer.Start()

$form.Add_Shown({
    Invoke-Action 'status'
    Invoke-Action 'get-config'
    Invoke-Action 'get-backups'
    Invoke-Action 'check-update' '' $false
    Pull-Logs
})
[void]$form.ShowDialog()
`, escapedExe, escapedPipe, escapedVersion, strings.ReplaceAll(uiHarnessWindowTitle, "'", "''"))
}
