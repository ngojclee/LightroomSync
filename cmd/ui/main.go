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
	action := flag.String("action", "", "Run one IPC action and print JSON result (ping|status|sync-now)")
	pipeName := flag.String("pipe", ipc.PipeName, "Named pipe path for Agent IPC")
	flag.Parse()

	log.Printf("[INFO] LightroomSync UI %s", Version)

	if *action != "" {
		env := runAction(*action, *pipeName)
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

func runAction(action, pipeName string) actionEnvelope {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "ping":
		return actionPing(pipeName)
	case "status":
		return actionStatus(pipeName)
	case "sync-now":
		return actionSyncNow(pipeName)
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
$form.Text = 'Lightroom Sync - Temporary GUI Test'
$form.Width = 880
$form.Height = 620
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

$btnSync = New-Object System.Windows.Forms.Button
$btnSync.Text = 'Sync Now'
$btnSync.Width = 120
$btnSync.Height = 36
$btnSync.Location = New-Object System.Drawing.Point(290, 80)
$btnSync.BackColor = [System.Drawing.Color]::FromArgb(11, 138, 106)
$btnSync.ForeColor = [System.Drawing.Color]::White
$form.Controls.Add($btnSync)

$btnClose = New-Object System.Windows.Forms.Button
$btnClose.Text = 'Close'
$btnClose.Width = 100
$btnClose.Height = 36
$btnClose.Location = New-Object System.Drawing.Point(420, 80)
$form.Controls.Add($btnClose)

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

$txtOutput = New-Object System.Windows.Forms.TextBox
$txtOutput.Multiline = $true
$txtOutput.ReadOnly = $true
$txtOutput.ScrollBars = 'Vertical'
$txtOutput.Font = New-Object System.Drawing.Font('Consolas', 9)
$txtOutput.Location = New-Object System.Drawing.Point(22, 214)
$txtOutput.Size = New-Object System.Drawing.Size(820, 340)
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

function Invoke-Action([string]$action) {
    try {
        $raw = & $exe --action $action --pipe $pipe 2>&1 | Out-String
        $txtOutput.Text = $raw.Trim()

        try {
            $obj = $raw | ConvertFrom-Json
            if ($obj -and $null -ne $obj.ok) {
                Set-Reachability([bool]$obj.ok)
            }

            if ($obj.data) {
                if ($obj.data.status_text) { $lblStatusValue.Text = [string]$obj.data.status_text }
                if ($obj.data.tray_color) { $lblTrayValue.Text = [string]$obj.data.tray_color }
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
$btnSync.Add_Click({
    Invoke-Action 'sync-now'
    Start-Sleep -Milliseconds 220
    Invoke-Action 'status'
})
$btnClose.Add_Click({ $form.Close() })

$timer = New-Object System.Windows.Forms.Timer
$timer.Interval = 2500
$timer.Add_Tick({ Invoke-Action 'status' })
$timer.Start()

$form.Add_Shown({ Invoke-Action 'status' })
[void]$form.ShowDialog()
`, escapedExe, escapedPipe)
}
