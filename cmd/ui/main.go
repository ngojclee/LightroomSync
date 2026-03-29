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
	action := flag.String("action", "", "Run one IPC action and print JSON result (ping|status|get-config|save-config|get-backups|sync-now|sync-backup)")
	payload := flag.String("payload", "", "Optional JSON payload or value for action commands")
	pipeName := flag.String("pipe", ipc.PipeName, "Named pipe path for Agent IPC")
	flag.Parse()

	log.Printf("[INFO] LightroomSync UI %s", Version)

	if *action != "" {
		env := runAction(*action, *payload, *pipeName)
		printJSON(env)
		if env.OK {
			return
		}
		os.Exit(1)
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

	return fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
[System.Windows.Forms.Application]::EnableVisualStyles()

$exe = '%s'
$pipe = '%s'

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
$btnPing.Width = 120
$btnPing.Height = 36
$btnPing.Location = New-Object System.Drawing.Point(22, 80)
$form.Controls.Add($btnPing)

$btnStatus = New-Object System.Windows.Forms.Button
$btnStatus.Text = 'Refresh Status'
$btnStatus.Width = 130
$btnStatus.Height = 36
$btnStatus.Location = New-Object System.Drawing.Point(150, 80)
$form.Controls.Add($btnStatus)

$btnGetConfig = New-Object System.Windows.Forms.Button
$btnGetConfig.Text = 'Get Config'
$btnGetConfig.Width = 120
$btnGetConfig.Height = 36
$btnGetConfig.Location = New-Object System.Drawing.Point(290, 80)
$form.Controls.Add($btnGetConfig)

$btnGetBackups = New-Object System.Windows.Forms.Button
$btnGetBackups.Text = 'Get Backups'
$btnGetBackups.Width = 120
$btnGetBackups.Height = 36
$btnGetBackups.Location = New-Object System.Drawing.Point(420, 80)
$form.Controls.Add($btnGetBackups)

$btnSyncNow = New-Object System.Windows.Forms.Button
$btnSyncNow.Text = 'Sync Now'
$btnSyncNow.Width = 120
$btnSyncNow.Height = 36
$btnSyncNow.Location = New-Object System.Drawing.Point(550, 80)
$btnSyncNow.BackColor = [System.Drawing.Color]::FromArgb(11, 138, 106)
$btnSyncNow.ForeColor = [System.Drawing.Color]::White
$form.Controls.Add($btnSyncNow)

$btnClose = New-Object System.Windows.Forms.Button
$btnClose.Text = 'Close'
$btnClose.Width = 90
$btnClose.Height = 36
$btnClose.Location = New-Object System.Drawing.Point(680, 80)
$form.Controls.Add($btnClose)

$chkAutoSync = New-Object System.Windows.Forms.CheckBox
$chkAutoSync.Text = 'Auto Sync'
$chkAutoSync.AutoSize = $true
$chkAutoSync.Location = New-Object System.Drawing.Point(790, 89)
$form.Controls.Add($chkAutoSync)

$btnSaveAutoSync = New-Object System.Windows.Forms.Button
$btnSaveAutoSync.Text = 'Save'
$btnSaveAutoSync.Width = 70
$btnSaveAutoSync.Height = 30
$btnSaveAutoSync.Location = New-Object System.Drawing.Point(880, 84)
$form.Controls.Add($btnSaveAutoSync)

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

$lblBackupRootTitle = New-Object System.Windows.Forms.Label
$lblBackupRootTitle.Text = 'Backup Folder:'
$lblBackupRootTitle.AutoSize = $true
$lblBackupRootTitle.Location = New-Object System.Drawing.Point(300, 132)
$form.Controls.Add($lblBackupRootTitle)

$lblBackupRootValue = New-Object System.Windows.Forms.Label
$lblBackupRootValue.Text = '-'
$lblBackupRootValue.AutoSize = $true
$lblBackupRootValue.MaximumSize = New-Object System.Drawing.Size(640, 0)
$lblBackupRootValue.Location = New-Object System.Drawing.Point(390, 132)
$form.Controls.Add($lblBackupRootValue)

$lblCatalogTitle = New-Object System.Windows.Forms.Label
$lblCatalogTitle.Text = 'Catalog Path:'
$lblCatalogTitle.AutoSize = $true
$lblCatalogTitle.Location = New-Object System.Drawing.Point(300, 156)
$form.Controls.Add($lblCatalogTitle)

$lblCatalogValue = New-Object System.Windows.Forms.Label
$lblCatalogValue.Text = '-'
$lblCatalogValue.AutoSize = $true
$lblCatalogValue.MaximumSize = New-Object System.Drawing.Size(640, 0)
$lblCatalogValue.Location = New-Object System.Drawing.Point(390, 156)
$form.Controls.Add($lblCatalogValue)

$lblSyncPathTitle = New-Object System.Windows.Forms.Label
$lblSyncPathTitle.Text = 'Selected Zip:'
$lblSyncPathTitle.AutoSize = $true
$lblSyncPathTitle.Location = New-Object System.Drawing.Point(300, 180)
$form.Controls.Add($lblSyncPathTitle)

$txtSyncPath = New-Object System.Windows.Forms.TextBox
$txtSyncPath.Location = New-Object System.Drawing.Point(390, 176)
$txtSyncPath.Width = 450
$form.Controls.Add($txtSyncPath)

$btnSyncSelected = New-Object System.Windows.Forms.Button
$btnSyncSelected.Text = 'Sync Selected'
$btnSyncSelected.Width = 110
$btnSyncSelected.Height = 28
$btnSyncSelected.Location = New-Object System.Drawing.Point(850, 174)
$form.Controls.Add($btnSyncSelected)

$lstBackups = New-Object System.Windows.Forms.ListBox
$lstBackups.Font = New-Object System.Drawing.Font('Consolas', 8.5)
$lstBackups.Location = New-Object System.Drawing.Point(22, 214)
$lstBackups.Size = New-Object System.Drawing.Size(440, 430)
$form.Controls.Add($lstBackups)

$txtOutput = New-Object System.Windows.Forms.TextBox
$txtOutput.Multiline = $true
$txtOutput.ReadOnly = $true
$txtOutput.ScrollBars = 'Vertical'
$txtOutput.Font = New-Object System.Drawing.Font('Consolas', 9)
$txtOutput.Location = New-Object System.Drawing.Point(474, 214)
$txtOutput.Size = New-Object System.Drawing.Size(488, 430)
$form.Controls.Add($txtOutput)

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
    if ($null -ne $cfg.auto_sync) {
        $chkAutoSync.Checked = [bool]$cfg.auto_sync
    }
    if ($cfg.backup_folder) {
        $lblBackupRootValue.Text = [string]$cfg.backup_folder
    } else {
        $lblBackupRootValue.Text = '-'
    }
    if ($cfg.catalog_path) {
        $lblCatalogValue.Text = [string]$cfg.catalog_path
    } else {
        $lblCatalogValue.Text = '-'
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

function Invoke-Action([string]$action, [string]$payload = '') {
    try {
        if ([string]::IsNullOrWhiteSpace($payload)) {
            $raw = & $exe --action $action --pipe $pipe 2>&1 | Out-String
        } else {
            $raw = & $exe --action $action --pipe $pipe --payload $payload 2>&1 | Out-String
        }
        $txtOutput.Text = $raw.Trim()

        try {
            $obj = $raw | ConvertFrom-Json
            if ($obj -and $null -ne $obj.ok) {
                Set-Reachability([bool]$obj.ok)
            }

            if ($obj.data) {
                if ($obj.data.status_text) { $lblStatusValue.Text = [string]$obj.data.status_text }
                if ($obj.data.tray_color) { $lblTrayValue.Text = [string]$obj.data.tray_color }
                if ($null -ne $obj.data.auto_sync -or $obj.data.backup_folder -or $obj.data.catalog_path) {
                    Render-Config $obj.data
                }
                if ($obj.data -is [System.Array] -and $obj.data.Count -gt 0 -and $obj.data[0].path) {
                    Render-Backups $obj.data
                }
            } elseif ($obj.error) {
                $lblStatusValue.Text = [string]$obj.error
                $lblTrayValue.Text = '-'
            }
        } catch {
            Set-Reachability($false)
            $lblStatusValue.Text = 'Invalid JSON output'
        }
    } catch {
        Set-Reachability($false)
        $lblStatusValue.Text = $_.Exception.Message
        $txtOutput.Text = $_.Exception.Message
    }
}

$btnPing.Add_Click({ Invoke-Action 'ping' })
$btnStatus.Add_Click({ Invoke-Action 'status' })
$btnGetConfig.Add_Click({ Invoke-Action 'get-config' })
$btnGetBackups.Add_Click({ Invoke-Action 'get-backups' })
$btnSyncNow.Add_Click({
    Invoke-Action 'sync-now'
    Start-Sleep -Milliseconds 220
    Invoke-Action 'status'
})
$btnSaveAutoSync.Add_Click({
    $payloadObj = @{
        auto_sync = [bool]$chkAutoSync.Checked
    }
    $payloadJson = $payloadObj | ConvertTo-Json -Compress
    Invoke-Action 'save-config' $payloadJson
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
})
$btnClose.Add_Click({ $form.Close() })

$lstBackups.Add_SelectedIndexChanged({
    if ($lstBackups.SelectedItem) {
        $txtSyncPath.Text = [string]$lstBackups.SelectedItem
    }
})

$timer = New-Object System.Windows.Forms.Timer
$timer.Interval = 2500
$timer.Add_Tick({ Invoke-Action 'status' })
$timer.Start()

$form.Add_Shown({
    Invoke-Action 'status'
    Invoke-Action 'get-config'
    Invoke-Action 'get-backups'
})
[void]$form.ShowDialog()
`, escapedExe, escapedPipe, strings.ReplaceAll(uiHarnessWindowTitle, "'", "''"))
}
