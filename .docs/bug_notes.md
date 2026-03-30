# Lightroom Sync - Known Bugs & Technical Debt

> **Date Updated**: 2026-03-30 (QC Round 2)
> **Target Version**: v2.0.0.0
> **Purpose**: Record of current unresolved issues, technical compromises, and implementation blockers for handoff to other developers/teams.

## 1. Wails Build Structure Constraint
- **Issue**: Attempting to organize the Go source code cleanly by moving the Wails UI entrypoint to `cmd/ui/main.go` breaks the `wails build` compiler.
- **Root Cause**: Wails v2 expects `main.go` and `wails.json` to exist in the same root directory. Modifying paths in `wails.json` (such as `frontend:dir`) does not natively support pointing to a deep `main.go` for the Go compiler without complex build tag workarounds.
- **Current Workaround**: We had to move `main.go`, `wails_runtime.go`, and `.syso` objects back to the root directory `D:\Python\projects\LightroomSync\`.
- **To-Do**: Needs a deeper investigation into Wails CLI flags or custom `go build -tags` definitions if we want to restore the clean `cmd/ui` architectural separation.

## 2. Windows Executable Locking (Access Denied during Updates)
- **Issue**: Attempting to rebuild or replace `LightroomSync.exe` or `LightroomSyncAgent.exe` fails with "Access Denied" if the Agent is running in the background (System Tray).
- **Root Cause**: Windows strictly locks executing binaries.
- **Current Workaround**: During development, we must manually run `Stop-Process -Name "LightroomSync*"` before rebuilding. 
- **To-Do (Critical for Production)**: The Inno Setup installer and the in-app `check_update`/`download_update` flow must implement a robust process-kill hook or a binary-rename trick (move old `.exe` to `.old`, write new `.exe`, then restart) to ensure silent auto-updates don't crash or corrupt the installation.

## 3. "Close to Tray" Window Lifecycle Hook
- **Issue**: The documented architecture dictates that if "Close to Tray" is checked, pressing the `[X]` button on the UI should *Hide* the UI window rather than completely killing the memory process, while still keeping the Agent alive.
- **Root Cause**: Wails by default destroys the application loop when the main window is closed.
- **Current Workaround**: We currently documented the intended behavior in `plan.md` and `task.md`.
- **To-Do**: We need to actually implement Wails' `wails.Run(options.App{ ... OnBeforeClose: app.beforeClose ... })` hook. The Go backend must intercept this event, check the `config.yaml` for `CloseToTray`, and if true, call `runtime.WindowHide(ctx)` and return `true` (preventing actual closure).

## 4. Native Directory Picker in Wails WebView
- **Issue**: Standard HTML `<input type="file" webkitdirectory>` often behaves inconsistently or looks out-of-place inside the Wails WebView when selecting Lightroom Catalog Backup folders.
- **To-Do**: The frontend UI for selecting directories in the Settings page must be wired to call a Go backend Bridge method. That Go method should trigger `runtime.OpenDirectoryDialog(ctx, options)` to summon the native Windows File Explorer picker, and then pass the selected absolute path string back to the Javascript UI.

## 6. Tray Icon Not Showing — UI Executable Name Mismatch (FIXED 2026-03-30)
- **Issue**: After Wails migration, the system tray icon did not appear. The tray's "Open UI" action pointed to a nonexistent executable. `WindowHide()` made the app completely invisible with no way to restore.
- **Root Cause**: The Wails build produces `LightroomSync.exe` (from `wails.json` `outputfilename`), but `resolveUIExecutable()` in `cmd/agent/main.go` only searched for `LightroomSyncUI.exe` and `ui.exe`. The PowerShell NotifyIcon script received a path to a nonexistent binary, so `ExtractAssociatedIcon()` failed silently, and "Open UI" launched nothing.
- **Fix Applied**: Added `LightroomSync.exe` as the first candidate in `resolveUIExecutable()`. Verified: `Tray bootstrap started (ui=...LightroomSync.exe)`.

## 7. Vite Output Directory Mismatch (FIXED 2026-03-30)
- **Issue**: `wails build` succeeded but the embedded frontend was empty/stale.
- **Root Cause**: `vite.config.ts` had `outDir: '../cmd/ui/dist'` (old harness location). The Wails `wails_runtime.go` embedded `//go:embed all:dist` (root `/dist/` which only had a `.gitkeep`). Net result: the Go binary embedded an empty or stale frontend.
- **Fix Applied**: Changed Vite `outDir` to `'dist'` (→ `frontend/dist/`). Changed Go embed to `//go:embed all:frontend/dist`. Frontend now compiles to the correct location and embeds properly.

## 8. Build Script Harness/Wails Confusion (FIXED 2026-03-30)
- **Issue**: `build_windows.ps1` defaulted to `harness` runtime, looked for `./cmd/ui` which no longer had the main entrypoint, and the Wails output binary name was `LightroomSync.exe` not `LightroomSyncUI.exe`.
- **Root Cause**: Build script wasn't updated after the root-level Wails migration. Wails `-clean` also wiped `build/bin/` including the agent built moments before.
- **Fix Applied**: Default runtime → `wails`, removed harness build path entirely, Wails UI builds first (before agent), fixed binary name to `LightroomSync.exe`, removed `-clean` flag to avoid locking issues on Windows.

## 9. Stale `bin/` and `cmd/ui/dist/` Artifacts
- **Issue**: Old harness binaries in `bin/` (root) and old frontend build in `cmd/ui/dist/` confused the build and test workflow.
- **Fix Applied**: Deleted both. Canonical output is now exclusively `build/bin/`.

## 5. UI Initialization Race Condition (FIXED 2026-03-30)
- **Issue**: If `LightroomSync.exe` (Wails UI) and `LightroomSyncAgent.exe` boot up simultaneously during Windows Startup, the UI might mount and attempt to connect to the Named Pipe before the Agent has fully initialized the IPC server.
- **Root Cause**: Frontend `bootstrap()` triggered `refreshStatus()` immediately and rendered `Disconnected` on first failures, causing a visible startup flicker.
- **Fix Applied**: Added a 3-second connecting grace period in `frontend/src/App.ts`, introduced a neutral `Connecting...` badge state, and only show red `Disconnected`/overlay after `disconnectFailCount >= 3` failures post-bootstrap.

## 10. Sidebar "Hide to Tray" Minimize Behavior (FIXED 2026-03-30)
- **Issue**: Sidebar button `btn-hide-to-tray` called `HideToTray()` (`WindowHide`) and made the window fully invisible instead of minimizing to taskbar.
- **Root Cause**: `WindowHide` was used for a button that should have taskbar minimize semantics.
- **Fix Applied**:
  - Added `MinimiseWindow()` in `wails_app.go` using `wailsruntime.WindowMinimise(ctx)`.
  - Added `minimiseWindow()` bridge export in `frontend/src/bridge.ts`.
  - Updated `frontend/src/App.ts` button handler to call `minimiseWindow()`.
  - Updated sidebar label in `frontend/src/template.ts`: **Hide to Tray** → **Minimize**.
  - Kept `HideToTray()` for close-to-tray lifecycle hooks.

## 11. Preset Scan Requires Manual Save (FIXED 2026-03-30)
- **Issue**: Scan button populated `preset_categories` input but did not persist config unless user manually clicked Save.
- **Root Cause**: `scanPresets()` only wrote UI field state and showed informational banner.
- **Fix Applied**: `scanPresets()` now calls `saveConfig()` automatically on successful discovery and shows success banner:
  - `"<N> preset categories discovered and saved."`

## 12. Tray Script Agent PID Guard (FIXED 2026-03-30)
- **Issue**: Tray host stability risk when Agent PID is unset/invalid (`-1`) during startup environments.
- **Root Cause**: PID-dependent process checks can fail when PID is not a positive integer.
- **Fix Applied**: Hardened tray script guard in `internal/tray/manager_windows.go` by checking `($AgentPid -as [int]) -gt 0` before PID-based `Get-Process`/`Stop-Process` operations.
