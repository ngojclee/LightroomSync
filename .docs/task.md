# Lightroom Sync Go Rewrite — Task Tracking

> **Plan**: [plan.md](/d:/Python/projects/LightroomSync/.docs/plan.md)  
> **Target**: v2.0.0.0  
> **Date**: 2026-03-30
> **Last Progress Sync**: 2026-03-30

## Phase 0: Architecture Gates (Must Pass First)

### 0.1 Compatibility Contract Freeze

- [x] Capture Python fixtures for `lightroom_lock.txt`, `sync_manifest.json`, `network_settings.json`, `preset_state.json`
- [x] Define manifest compatibility rule: reader accepts `zip_file` and `zip_path`; writer outputs `zip_file`
- [x] Add golden tests for all compatibility fixtures
- [x] Document immutable wire-format contract in `.docs/plan.md`

### 0.2 Process & IPC Contract

- [x] Define IPC command set: `GetStatus`, `SaveConfig`, `SyncNow`, `SyncBackup`, `GetBackups`, `SubscribeLogs`
- [x] Define IPC error model and timeout behavior
- [x] Define reconnect behavior when UI starts before Agent
- [x] Implement architecture spike proving: Agent tray + UI launch/focus + IPC roundtrip (automated script `scripts/phase0_2_architecture_spike.ps1` + runbook in `.docs/phase0-2-architecture-spike.md`)

### 0.3 Platform Boundary

- [x] Create `internal/platform/windows` and `internal/platform/common`
- [x] Add build tags for Windows-specific code from day 1
- [x] Ensure core sync code compiles without Windows-only imports

## Phase 1: Scaffold & Runtime Skeleton

### 1.1 Project Bootstrap

- [x] Install Go 1.22+ and Wails CLI
- [x] Initialize module `github.com/ngojclee/lightroom-sync`
- [x] Create structure: `cmd/agent`, `cmd/ui`, `internal/*`, `frontend/`
- [x] Add `.gitignore` for Go/Wails/build artifacts
- [x] Add `Makefile` or `build.ps1` with `agent`, `ui`, `all` targets

### 1.2 Agent Skeleton

- [x] Implement `cmd/agent/main.go` with single-instance mutex
- [x] Add tray bootstrap (`internal/tray`) with status label + menu items
- [x] Add graceful shutdown flow (stop workers, write OFFLINE, quit tray)

### 1.3 UI Skeleton

- [x] Implement `cmd/ui/main.go` (Wails window only)
- [x] UI startup checks Agent availability via IPC ping
- [x] Add temporary Windows Forms GUI harness for IPC testing (`status`, `sync_now`, `ping`) before full Wails implementation
- [x] Add window focus behavior when already-open UI instance exists

## Phase 2: Config & Windows Integration

### 2.1 Config Model

- [x] Define `Config` struct matching Python YAML semantics
- [x] Implement load/save/default/validation
- [x] Config location: `%LOCALAPPDATA%\LightroomSync\config.yaml`

### 2.2 Legacy Migration

- [x] Detect legacy config paths (Python version locations)
- [x] Migrate legacy config once, preserve original backup copy
- [x] Emit migration log event and status hint in UI

### 2.3 Windows Integration

- [x] Implement start-with-Windows registry write/delete
- [x] Include `--minimized` when configured
- [x] Validate startup path quoting and spaces safety

## Phase 3: Monitors, Event Loop, and Resilience

### 3.1 Lightroom Monitor

- [x] Implement process detection (`CreateToolhelp32Snapshot`)
- [x] Edge-trigger events: started/stopped only on state transition
- [x] Add monitor health metrics and error logs

### 3.2 Lock Manager

- [x] Implement parser/writer for `STATUS|MACHINE|TIMESTAMP`
- [x] Implement atomic write: write temp + rename
- [x] Add optional `session_id` and `epoch` internally (do not break legacy file format)
- [x] Heartbeat loop with retry/backoff policy

### 3.3 Backup Monitor

- [x] Implement recursive zip discovery
- [x] Track last-seen signature to avoid duplicate emits
- [x] Make polling interval configurable

### 3.4 Resilience Layer

- [x] Add operation watchdog with `op_id` and deadline
- [x] Add circuit breaker for unstable network share
- [x] Add reconnect recovery workflow after share returns
- [x] Add sleep/resume handler: force state revalidation post-resume

### 3.5 Event Coordinator

- [x] Implement typed event bus + buffered channel
- [x] Add single sync worker queue (one-at-a-time)
- [x] Maintain authoritative app state cache for tray/UI reads

## Phase 4: Catalog Sync

### 4.1 Core Sync

- [x] Validate zip integrity before extract
- [x] Implement zip-slip protection (canonical destination path check)
- [x] Cleanup old catalog artifacts before extraction
- [x] Support wrapper-folder zip layouts

### 4.2 Manifest Logic

- [x] Implement manifest read/write with compatibility keys
- [x] Implement anti-self-sync rules
- [x] Verify zip existence + size before sync
- [x] Add structured reason codes for skip/allow decisions

### 4.3 Orchestration

- [x] Initial startup manifest check
- [x] Pending sync queue when Lightroom is running
- [x] Write manifest after local backup creation
- [x] Update `LastSyncedTimestamp` only on successful sync completion

### 4.4 Retention

- [x] Implement pre-sync backup folder policy
- [x] Implement retention cleanup for pre-sync and network backups

## Phase 5: Preset & Watermark Sync

### 5.1 Preset Sync

- [x] Implement push + pull with deletion-aware state
- [x] Use mtime tolerance to reduce false conflict
- [x] Commit state file only after successful sync cycle

### 5.2 Category Discovery

- [x] Scan Lightroom preset directories dynamically
- [x] Filter categories using user config
- [x] Ensure remote category folder bootstrap

### 5.3 Watermark/Logo

- [x] Parse `.lrtemplate` and `.lrsmv` image paths
- [x] Copy logos to shared `Logos/` folder
- [x] Rewrite paths for local and network contexts
- [x] Skip redundant copy when size/hash unchanged

## Phase 6: UI (IPC-Driven Harness Complete)

### 6.1 Settings Tab

- [x] Implement config form and validation messages
- [x] Bind actions to Agent IPC (`SaveConfig`, `SyncNow`, `RefreshStatus`)
- [x] Render live status without direct filesystem calls
- [x] Implement Agent-side IPC commands `GetConfig` + `SaveConfig` with partial payload patching and validation (temporary GUI harness integration)

### 6.2 Backup Browser Tab

- [x] Fetch list from Agent (`GetBackups`)
- [x] Implement sync-selected workflow (`SyncBackup`)
- [x] Add pause/resume sync control via Agent state
- [x] Implement Agent-side IPC commands `GetBackups` + `SyncBackup` and temporary GUI harness controls for manual testing

### 6.3 Log Tab

- [x] Subscribe to log stream from Agent
- [x] Implement level filters + max buffer limit

### 6.4 Update Tab

- [x] Show current/latest version and release notes
- [x] Trigger update flow via Agent
- [x] Render progress events

## Phase 6R: Real Wails GUI Cutover (Pending)

> Detailed tracker: [.docs/wails-ui-cutover/task.md](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/task.md)
>  
> Execution guide: [.docs/wails-ui-cutover/execution.md](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/execution.md)

- [x] Expand Wails cutover planning docs (waves, gates, command checklist)
- [x] Add Wave 1 bootstrap spec + readiness check (`wails version`)
- [x] Add Wave 2 `internal/uiapi` refactor spec + timeline/dependency map
- [x] Add Wave 3 frontend shell spec + tab-to-command contract map
- [x] Bootstrap Wails runtime + keep `--action` CLI compatibility
Status note: runtime switch + Wails/frontend scaffold are implemented, and `go.mod` now declares `github.com/wailsapp/wails/v2` with build-tagged embedded runtime wiring. Strict Wails runtime validation passed.
- [x] Extract reusable UI API bridge from `cmd/ui` into `internal/uiapi`
- [x] Add Wave 2 parity automation (`scripts/e2e_ui_command_parity.ps1`) + evidence output under `build/e2e`
- [x] Implement Wails frontend tabs (Status/Settings/Backups/Logs/Update)
- [x] Wire polling/event flows and robust in-flight/error handling
Status note: Wave 3 + Wave 4 baseline is now implemented in `frontend/src` with a full tab shell, Wails/global bridge fallback, visibility-aware polling (`status` + `subscribe-logs`), and in-flight guards. Validation passed with real Wails window.
- [x] Integrate Wails artifact into build/installer pipeline
Status note: `scripts/build_windows.ps1` and `scripts/build_installer.ps1` are runtime-aware (`-UIRuntime harness|wails`) with optional fallback (`-AllowHarnessFallback`) and metadata/runtime validation to keep packaged UI artifacts explicit and traceable. The Wails build path successfully builds strict Wails without harness fallback.
- [x] Execute Wails validation matrix and switch default UI runtime
Status note: Wails smoke + tray automation are available via `scripts/e2e_wails_ui_smoke.ps1` and runtime-aware `scripts/e2e_tray_ui_smoke.ps1` (`-UIRuntime wails`). Strict runs pass in this host with network dependencies fully resolved through AI assistant infrastructure.

## Phase 7: Build, Release, and Installer

### 7.1 Build

- [x] Build `LightroomSyncAgent.exe` and `LightroomSyncUI.exe`
- [x] Inject version `x.y.z.k` using `-ldflags`
- [x] Validate symbols and file metadata

### 7.2 Installer

- [x] Adapt Inno Setup for two-process deployment
- [x] Register startup for Agent only
- [x] Ensure upgrade path kills old processes safely

### 7.3 Release

- [x] Add GitHub Actions build workflow
- [x] Add release asset naming convention
- [x] Prepare optional signing step

## Phase 8: Testing & Verification

### 8.1 Automated Tests

- [x] Unit tests: config, lock parser, manifest logic, version compare
- [x] Integration tests: catalog sync + preset sync on temp dirs
- [x] Compatibility tests: Python fixtures vs Go parser/writer

### 8.2 Chaos & Recovery Tests

- [x] Simulate slow SMB latency (5s+)
- [x] Simulate share disconnect/reconnect mid-operation
- [x] Simulate sleep/resume during heartbeat/sync
- [x] Simulate concurrent two-machine lock contention

### 8.3 Manual E2E

- [x] Add Windows manual E2E runbook + helper probe script (`.docs/e2e-windows-manual.md`, `scripts/e2e_windows_manual.ps1`)
- [x] Add installer regression automation helper (`scripts/e2e_installer_regression.ps1`) with JSON/log evidence output
- [x] Add two-machine snapshot compare helper (`scripts/e2e_two_machine_compare.ps1`) with JSON/markdown report output
- [x] Add tray/UI smoke helper (`scripts/e2e_tray_ui_smoke.ps1`) with pass/fail JSON evidence output
- [x] Add UI command parity helper (`scripts/e2e_ui_command_parity.ps1`) with envelope compatibility evidence output
- [ ] Two-machine end-to-end sync validation -> **PENDING USER DEPLOYMENT**
- [x] Tray actions and notifications validation (Done via `e2e_tray_ui_smoke.ps1`)
- [ ] UI responsiveness validation under network stress -> **PENDING USER EVALUATION**
- [x] Installer upgrade/uninstall regression (Initial test setup done, ready for version 2.0.0.1)

## Phase 9: Premium UI Overhaul (Wave 4)

- [x] Use Stitch MCP to generate Premium Editorial Design System (Light/Dark mode)
- [x] Refactor frontend/src/styles.css with complete UI design tokens (CSS variables)
- [x] Refactor frontend/src/App.ts to apply Sidebar layout and new CSS structure
- [x] Implement Light/Dark mode functionality with 'btn-theme-toggle'
- [x] Add Native Wails Browse Dialog bindings for Backup Directory and Catalog path selection
- [x] Re-run `e2e_ui_command_parity.ps1` to ensure DOM refs and event bindings remain intact
- [x] Preset category Auto-Discovery: Implement UI scan button and IPC bridge to traverse local Lightroom directories
- [x] Layout Optimization: Fix UI cut-off issues by adding responsive padding and centering notification banner

## UI and Tray Icon Collaboration Model (Chia nhóm các trường hợp GUI & Tray)
The Agent runs locally in the background and is controlled via IPC connections from the UI. Below are the interaction models:

* **Trường hợp 1: Mở UI khi Agent đã chạy (Bình thường)**
  - UI kết nối tới `\\.\pipe\LightroomSyncIPC`.
  - Agent trả về tín hiệu Status (Green/Active). UI cập nhật badge `Agent Active`.
  - Khi UI gọi Scan Preset, Agent nhận request quét nhanh thư mục và trả Array cho UI hiển thị ngay lập tức.
* **Trường hợp 2: Agent đang chạy nhưng gặp lỗi (Lỗi Sync/Mất kế nối mạng)**
  - Agent tự động đổi Icon trên Taskbar sang màu Vàng/Đỏ tùy mức độ lỗi.
  - UI (nếu đang bật) sẽ nhận Broadcast từ vòng lặp IPC để cập nhật lỗi chi tiết trên bảng Dashboard đồng bộ thời gian thực.
* **Trường hợp 3: Mở UI nhưng Agent chưa chạy (Offline / Disconnected)**
  - UI không thể kết nối tới Pipe, sẽ hiển thị màn hình Overlay `Agent Unreachable`.
  - Cho phép người dùng chọn `Launch Agent`. UI sẽ khởi chạy độc lập `LightroomSyncAgent.exe`.
  - Icon dưới Taskbar tự động xuất hiện. UI kết nối lại và mở màn hình chính.
* **Trường hợp 4: Agent tự động Sync (Background Sync)**
  - Khởi phát từ Agent (đủ interval hoặc bật Lightroom).
  - Tray Taskbar icon nhấp nháy/Spin theo trạng thái Syncing.
  - Nếu UI đang mở: Progress Card sẽ hiển thị chi tiết "Syncing..." mà không freeze giao diện.

### Quản lý Trạng thái Cửa sổ (Window Management & Closing Behaviors)
* **Trường hợp 5: Auto-start khởi động cùng Windows**
  - Mặc định khởi chạy hoàn toàn ở chế độ ngầm (Minimized/Hidden). Lập tức xuất hiện Icon dưới Taskbar mà không hiển thị pop-up GUI, tránh làm phiền người dùng.
* **Trường hợp 6: Thu nhỏ (Minimize GUI)**
  - Khi nhấn nút dấu trừ (Minimize) `[-]` trên GUI, cửa sổ quản lý thu nhỏ xuống thanh Taskbar Windows như các ứng dụng hệ thống bình thường.
* **Trường hợp 7: Đóng GUI (Close Window `[X]`) VỚI tùy chọn "Close to Tray" đang bật**
  - Cửa sổ UI `LightroomSync.exe` đóng và giải phóng RAM đồ họa.
  - Tuy nhiên, `LightroomSyncAgent.exe` không bị ảnh hưởng, tiếp tục duy trì hoạt động và giữ biểu tượng ở System Tray.
* **Trường hợp 8: Đóng GUI (Close Window `[X]`) NẾU KHÔNG bật tùy chọn "Close to Tray"**
  - Nút (X) vừa đóng ngay lập tức tiến trình `LightroomSync.exe` (Wails UI), vừa phát tín hiệu IPC/System Kill để triệt tiêu trực tiếp tiến trình ngầm `LightroomSyncAgent.exe`.
  - Ứng dụng thoát sạch 100% kèm mất Icon dưới Tray.
* **Trường hợp 9: Quit từ Tray (Exit Application)**
  - Nếu người dùng chuột phải vào Icon System Tray > chọn nút `Exit / Quit`.
  - Agent gửi signal cảnh báo EOF cho `LightroomSync.exe` (nếu GUI đang bật mặt tiền) để Shutdown UI.
  - Agent đóng Named Pipe và tự động kết liễu toàn bộ hệ sinh thái phần mềm. Thoát sạch hoàn toàn GUI lẫn Tray.

## Phase 10: QC Round 1 — Build & Lifecycle Validation (2026-03-30)

### 10.1 Build Pipeline Fixes
- [x] Fix `vite.config.ts` outDir: `'../cmd/ui/dist'` → `'dist'` (frontend/dist/)
- [x] Fix `wails_runtime.go` embed: `all:dist` → `all:frontend/dist`
- [x] Fix `build_windows.ps1`: default to wails, remove harness path, fix binary name to `LightroomSync.exe`
- [x] Fix build order: Wails UI first, then Agent (avoids `-clean` wiping agent)
- [x] Remove `-clean` from wails build (avoids Windows file locking issues)
- [x] Delete stale `bin/` (root) and `cmd/ui/dist/` artifacts
- [x] Clean `temp_scripts/` phase0_2 run artifacts and wails_template
- [x] Verify clean build produces `build/bin/LightroomSync.exe` + `build/bin/LightroomSyncAgent.exe` with correct version

### 10.2 Tray Icon Fix
- [x] Fix `resolveUIExecutable()` in `cmd/agent/main.go`: add `LightroomSync.exe` as first candidate (was only searching for `LightroomSyncUI.exe`)
- [x] Verify: Agent log shows `Tray bootstrap started (ui=...LightroomSync.exe)`
- [x] Guard tray script PID checks in `internal/tray/manager_windows.go` to run only when `AgentPid` is a positive integer
- [ ] Manual: Confirm tray icon is visible in system tray area -> **USER CHECK**
- [ ] Manual: Confirm tray "Open UI" launches the correct UI window -> **USER CHECK**
- [ ] Manual: Confirm tray status badge updates (Green/Yellow/Red) -> **USER CHECK**

### 10.3 Window Lifecycle Validation
- [ ] Manual: Native `[-]` minimize button minimizes to taskbar -> **USER CHECK**
- [ ] Manual: `[X]` close with "minimize to tray" ON → window hides, tray icon stays -> **USER CHECK**
- [ ] Manual: `[X]` close with "minimize to tray" OFF → full app exit -> **USER CHECK**
- [x] Code: Sidebar `btn-hide-to-tray` now calls `MinimiseWindow()` (`runtime.WindowMinimise`) and label changed to "Minimize"
- [ ] Manual: Sidebar "Minimize" button minimizes to taskbar (window remains on taskbar) -> **USER CHECK**
- [ ] Manual: Sidebar "Close UI" button → UI exits, agent keeps running -> **USER CHECK**
- [ ] Manual: Sidebar "Exit All" button → both UI + agent stop -> **USER CHECK**
- [ ] Manual: Tray "Exit Agent" → both tray + agent stop -> **USER CHECK**

### 10.4 Preset Scan Validation
- [x] Verify `discover-presets` IPC returns categories (confirmed: 18 categories from Lightroom install)
- [x] Code: `scanPresets()` now auto-calls `saveConfig()` and shows success banner `"X preset categories discovered and saved."`
- [ ] Manual: Click "Scan" button in Settings → categories populate input box -> **USER CHECK**

### 10.6 Startup Connection UX Validation
- [x] Code: Added 3s startup grace period + `disconnectFailCount >= 3` threshold before red disconnected state/overlay
- [ ] Manual: Startup badge shows "Connecting..." then transitions to "Connected" without red flash -> **USER CHECK**

### 10.5 Build Output Location
- **Canonical path**: `D:\Python\projects\LightroomSync\build\bin\`
  - `LightroomSync.exe` — Wails UI (dark theme)
  - `LightroomSyncAgent.exe` — Background agent + system tray
  - `build-metadata.json` — Build provenance
- **Old `bin/` at project root**: DELETED (was stale old build)

## Post-Launch

- [x] Update project memory docs (`CLAUDE.md`) with final architecture
- [ ] Archive Python release as `python-final`
- [ ] Monitor production telemetry/log patterns for first 7 days
- [ ] Decide macOS/Linux pilot based on architecture readiness

## Session 2026-03-30 (v2.0.1.0 — tray fix + single-app UX)
- [x] **Fix tray icon crash**: Root cause was `$ErrorActionPreference = 'Stop'` at script top, causing any non-critical error to terminate the entire PowerShell tray host before log files could be written. Reverted to `SilentlyContinue` with explicit `-ErrorAction Stop` only on assembly loading.
- [x] **Fix Agent CMD window**: Agent was built without `-H windowsgui` ldflags, causing it to always open a console window. Added `-H windowsgui` to agent build so it runs silently as a GUI subsystem app (PE Subsystem = 2).
- [x] **Auto-launch Agent from UI**: `bootstrap()` in `App.ts` now automatically calls `launchAgent()` when Agent is unreachable — user opens ONE app and everything starts.
- [x] **Installer cleanup**: Removed redundant "Start with Windows" task from install wizard (only "Desktop shortcut" remains). Auto-start registry is always set on install (user can toggle from Settings). Post-install shows single "Launch Lightroom Sync" checkbox.
- [x] **Build + Release v2.0.1.0**: Pushed to GitHub main, published on `win-toolbox`.
