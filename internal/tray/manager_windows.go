//go:build windows

package tray

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"unicode/utf16"
)

// Options configures tray bootstrap behavior.
type Options struct {
	AppName      string
	AgentPID     int
	UIExecutable string
	PipeName     string
	StatusPath   string
}

// Manager hosts a lightweight tray process via PowerShell NotifyIcon.
type Manager struct {
	opts Options

	mu  sync.Mutex
	cmd *exec.Cmd
}

// NewManager creates a Windows tray manager.
func NewManager(opts Options) *Manager {
	if strings.TrimSpace(opts.AppName) == "" {
		opts.AppName = "Lightroom Sync"
	}
	if opts.AgentPID <= 0 {
		opts.AgentPID = -1
	}
	return &Manager{opts: opts}
}

// Start launches tray host as detached PowerShell process.
func (m *Manager) Start(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil && m.cmd.Process != nil {
		return nil
	}

	script := renderPowerShellTrayScript(m.opts)
	encoded := encodePowerShellCommand(script)

	var lastErr error
	for _, shell := range []string{"powershell", "pwsh"} {
		cmd := exec.Command(
			shell,
			"-NoProfile",
			"-ExecutionPolicy",
			"Bypass",
			"-WindowStyle", "Hidden",
			"-EncodedCommand",
			encoded,
		)
		if err := cmd.Start(); err != nil {
			lastErr = err
			continue
		}
		m.cmd = cmd
		return nil
	}

	if lastErr != nil {
		return fmt.Errorf("launch tray host: %w", lastErr)
	}
	return fmt.Errorf("launch tray host: unknown error")
}

// Stop terminates tray host process if still running.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}

	err := m.cmd.Process.Kill()
	m.cmd = nil
	return err
}

func encodePowerShellCommand(script string) string {
	// PowerShell -EncodedCommand expects UTF-16LE bytes.
	words := utf16.Encode([]rune(script))
	utf16LE := make([]byte, 0, len(words)*2)
	for _, w := range words {
		utf16LE = append(utf16LE, byte(w), byte(w>>8))
	}
	return base64.StdEncoding.EncodeToString(utf16LE)
}

func psSingleQuote(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func renderPowerShellTrayScript(opts Options) string {
	appName := psSingleQuote(opts.AppName)
	uiExe := psSingleQuote(opts.UIExecutable)
	pipe := psSingleQuote(opts.PipeName)
	statusPath := psSingleQuote(opts.StatusPath)

	return fmt.Sprintf(`
$ErrorActionPreference = 'SilentlyContinue'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$AppName = '%s'
$AgentPid = %d
$UIExe = '%s'
$PipeName = '%s'
$StatusFile = '%s'

function Invoke-UiAction([string]$action) {
    if ([string]::IsNullOrWhiteSpace($UIExe)) { return }
    try {
        Start-Process -FilePath $UIExe -ArgumentList @('--action', $action, '--pipe', $PipeName) -WindowStyle Hidden | Out-Null
    } catch {}
}

$notify = New-Object System.Windows.Forms.NotifyIcon
$notify.Text = $AppName
$notify.Icon = [System.Drawing.SystemIcons]::Application
$notify.Visible = $true

$menu = New-Object System.Windows.Forms.ContextMenuStrip
$statusItem = New-Object System.Windows.Forms.ToolStripMenuItem('Status: starting...')
$statusItem.Enabled = $false
$menu.Items.Add($statusItem) | Out-Null
$menu.Items.Add('-') | Out-Null

$openUI = New-Object System.Windows.Forms.ToolStripMenuItem('Open UI')
$openUI.Add_Click({
    try {
        if (-not [string]::IsNullOrWhiteSpace($UIExe)) {
            Start-Process -FilePath $UIExe | Out-Null
        }
    } catch {}
})
$menu.Items.Add($openUI) | Out-Null

$syncNow = New-Object System.Windows.Forms.ToolStripMenuItem('Sync Now')
$syncNow.Add_Click({ Invoke-UiAction 'sync-now' })
$menu.Items.Add($syncNow) | Out-Null

$menu.Items.Add('-') | Out-Null
$exitItem = New-Object System.Windows.Forms.ToolStripMenuItem('Exit Agent')
$exitItem.Add_Click({
    try {
        if ($AgentPid -gt 0) {
            Stop-Process -Id $AgentPid -Force -ErrorAction SilentlyContinue
        }
    } catch {}
})
$menu.Items.Add($exitItem) | Out-Null

$notify.ContextMenuStrip = $menu
$notify.Add_DoubleClick({
    try {
        if (-not [string]::IsNullOrWhiteSpace($UIExe)) {
            Start-Process -FilePath $UIExe | Out-Null
        }
    } catch {}
})

$timer = New-Object System.Windows.Forms.Timer
$timer.Interval = 1000
$timer.Add_Tick({
    if ($AgentPid -gt 0) {
        $alive = Get-Process -Id $AgentPid -ErrorAction SilentlyContinue
        if (-not $alive) {
            $timer.Stop()
            $notify.Visible = $false
            $notify.Dispose()
            [System.Windows.Forms.Application]::Exit()
            return
        }
    }

    if (Test-Path $StatusFile) {
        try {
            $raw = Get-Content -LiteralPath $StatusFile -Raw
            $obj = $raw | ConvertFrom-Json
            if ($obj -and $obj.status_text) {
                $statusText = [string]$obj.status_text
                if ($statusText.Length -gt 40) {
                    $statusText = $statusText.Substring(0, 40) + '...'
                }
                $statusItem.Text = 'Status: ' + $statusText
                $notify.Text = (($AppName + ' - ' + $statusText).Substring(0, [Math]::Min(($AppName + ' - ' + $statusText).Length, 63)))
            }
        } catch {}
    }
})
$timer.Start()

[System.Windows.Forms.Application]::Run()
$notify.Visible = $false
$notify.Dispose()
`, appName, opts.AgentPID, uiExe, pipe, statusPath)
}
