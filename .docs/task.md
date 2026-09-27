# Lightroom Sync Go Rewrite — Task Tracking

> **Plan**: [plan.md](/d:/Python/projects/LightroomSync/.docs/plan.md)
> **Target**: v2.0.0.0
> **Date**: 2026-03-30
> **Last Progress Sync**: 2026-03-30

## Phase 0: Architecture Gates (Must Pass First)

### 0.1 Compatibility Contract Freeze

- [X] Capture Python fixtures for `lightroom_lock.txt`, `sync_manifest.json`, `network_settings.json`, `preset_state.json`
- [X] Define manifest compatibility rule: reader accepts `zip_file` and `zip_path`; writer outputs `zip_file`
- [X] Add golden tests for all compatibility fixtures
- [X] Document immutable wire-format contract in `.docs/plan.md`

### 0.2 Process & IPC Contract

- [X] Define IPC command set: `GetStatus`, `SaveConfig`, `SyncNow`, `SyncBackup`, `GetBackups`, `SubscribeLogs`
- [X] Define IPC error model and timeout behavior
- [X] Define reconnect behavior when UI starts before Agent
- [X] Implement architecture spike proving: Agent tray + UI launch/focus + IPC roundtrip (automated script `scripts/phase0_2_architecture_spike.ps1` + runbook in `.docs/phase0-2-architecture-spike.md`)

### 0.3 Platform Boundary

- [X] Create `internal/platform/windows` and `internal/platform/common`
- [X] Add build tags for Windows-specific code from day 1
- [X] Ensure core sync code compiles without Windows-only imports

## Phase 1: Scaffold & Runtime Skeleton

### 1.1 Project Bootstrap

- [X] Install Go 1.22+ and Wails CLI
- [X] Initialize module `github.com/ngojclee/lightroom-sync`
- [X] Create structure: `cmd/agent`, `cmd/ui`, `internal/*`, `frontend/`
- [X] Add `.gitignore` for Go/Wails/build artifacts
- [X] Add `Makefile` or `build.ps1` with `agent`, `ui`, `all` targets

### 1.2 Agent Skeleton

- [X] Implement `cmd/agent/main.go` with single-instance mutex
- [X] Add tray bootstrap (`internal/tray`) with status label + menu items
- [X] Add graceful shutdown flow (stop workers, write OFFLINE, quit tray)

### 1.3 UI Skeleton

- [X] Implement `cmd/ui/main.go` (Wails window only)
- [X] UI startup checks Agent availability via IPC ping
- [X] Add temporary Windows Forms GUI harness for IPC testing (`status`, `sync_now`, `ping`) before full Wails implementation
- [X] Add window focus behavior when already-open UI instance exists

## Phase 2: Config & Windows Integration

### 2.1 Config Model

- [X] Define `Config` struct matching Python YAML semantics
- [X] Implement load/save/default/validation
- [X] Config location: `%LOCALAPPDATA%\LightroomSync\config.yaml`

### 2.2 Legacy Migration

- [X] Detect legacy config paths (Python version locations)
- [X] Migrate legacy config once, preserve original backup copy
- [X] Emit migration log event and status hint in UI

### 2.3 Windows Integration

- [X] Implement start-with-Windows registry write/delete
- [X] Include `--minimized` when configured
- [X] Validate startup path quoting and spaces safety

## Phase 3: Monitors, Event Loop, and Resilience

### 3.1 Lightroom Monitor

- [X] Implement process detection (`CreateToolhelp32Snapshot`)
- [X] Edge-trigger events: started/stopped only on state transition
- [X] Add monitor health metrics and error logs

### 3.2 Lock Manager

- [X] Implement parser/writer for `STATUS|MACHINE|TIMESTAMP`
- [X] Implement atomic write: write temp + rename
- [X] Add optional `session_id` and `epoch` internally (do not break legacy file format)
- [X] Heartbeat loop with retry/backoff policy

### 3.3 Backup Monitor

- [X] Implement recursive zip discovery
- [X] Track last-seen signature to avoid duplicate emits
- [X] Make polling interval configurable

### 3.4 Resilience Layer

- [X] Add operation watchdog with `op_id` and deadline
- [X] Add circuit breaker for unstable network share
- [X] Add reconnect recovery workflow after share returns
- [X] Add sleep/resume handler: force state revalidation post-resume

### 3.5 Event Coordinator

- [X] Implement typed event bus + buffered channel
- [X] Add single sync worker queue (one-at-a-time)
- [X] Maintain authoritative app state cache for tray/UI reads

## Phase 4: Catalog Sync

### 4.1 Core Sync

- [X] Validate zip integrity before extract
- [X] Implement zip-slip protection (canonical destination path check)
- [X] Cleanup old catalog artifacts before extraction
- [X] Support wrapper-folder zip layouts

### 4.2 Manifest Logic

- [X] Implement manifest read/write with compatibility keys
- [X] Implement anti-self-sync rules
- [X] Verify zip existence + size before sync
- [X] Add structured reason codes for skip/allow decisions

### 4.3 Orchestration

- [X] Initial startup manifest check
- [X] Pending sync queue when Lightroom is running
- [X] Write manifest after local backup creation
- [X] Update `LastSyncedTimestamp` only on successful sync completion

### 4.4 Retention

- [X] Implement pre-sync backup folder policy
- [X] Implement retention cleanup for pre-sync and network backups

## Phase 5: Preset & Watermark Sync

### 5.1 Preset Sync

- [X] Implement push + pull with deletion-aware state
- [X] Use mtime tolerance to reduce false conflict
- [X] Commit state file only after successful sync cycle

### 5.2 Category Discovery

- [X] Scan Lightroom preset directories dynamically
- [X] Filter categories using user config
- [X] Ensure remote category folder bootstrap

### 5.3 Watermark/Logo

- [X] Parse `.lrtemplate` and `.lrsmv` image paths
- [X] Copy logos to shared `Logos/` folder
- [X] Rewrite paths for local and network contexts
- [X] Skip redundant copy when size/hash unchanged

## Phase 6: UI (IPC-Driven Harness Complete)

### 6.1 Settings Tab

- [X] Implement config form and validation messages
- [X] Bind actions to Agent IPC (`SaveConfig`, `SyncNow`, `RefreshStatus`)
- [X] Render live status without direct filesystem calls
- [X] Implement Agent-side IPC commands `GetConfig` + `SaveConfig` with partial payload patching and validation (temporary GUI harness integration)

### 6.2 Backup Browser Tab

- [X] Fetch list from Agent (`GetBackups`)
- [X] Implement sync-selected workflow (`SyncBackup`)
- [X] Add pause/resume sync control via Agent state
- [X] Implement Agent-side IPC commands `GetBackups` + `SyncBackup` and temporary GUI harness controls for manual testing

### 6.3 Log Tab

- [X] Subscribe to log stream from Agent
- [X] Implement level filters + max buffer limit

### 6.4 Update Tab

- [X] Show current/latest version and release notes
- [X] Trigger update flow via Agent
- [X] Render progress events

## Phase 6R: Real Wails GUI Cutover (Pending)

> Detailed tracker: [.docs/wails-ui-cutover/task.md](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/task.md)
>
> Execution guide: [.docs/wails-ui-cutover/execution.md](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/execution.md)

- [X] Expand Wails cutover planning docs (waves, gates, command checklist)
- [X] Add Wave 1 bootstrap spec + readiness check (`wails version`)
- [X] Add Wave 2 `internal/uiapi` refactor spec + timeline/dependency map
- [X] Add Wave 3 frontend shell spec + tab-to-command contract map
- [X] Bootstrap Wails runtime + keep `--action` CLI compatibility
  Status note: runtime switch + Wails/frontend scaffold are implemented, and `go.mod` now declares `github.com/wailsapp/wails/v2` with build-tagged embedded runtime wiring. Strict Wails runtime validation passed.
- [X] Extract reusable UI API bridge from `cmd/ui` into `internal/uiapi`
- [X] Add Wave 2 parity automation (`scripts/e2e_ui_command_parity.ps1`) + evidence output under `build/e2e`
- [X] Implement Wails frontend tabs (Status/Settings/Backups/Logs/Update)
- [X] Wire polling/event flows and robust in-flight/error handling
  Status note: Wave 3 + Wave 4 baseline is now implemented in `frontend/src` with a full tab shell, Wails/global bridge fallback, visibility-aware polling (`status` + `subscribe-logs`), and in-flight guards. Validation passed with real Wails window.
- [X] Integrate Wails artifact into build/installer pipeline
  Status note: `scripts/build_windows.ps1` and `scripts/build_installer.ps1` are runtime-aware (`-UIRuntime harness|wails`) with optional fallback (`-AllowHarnessFallback`) and metadata/runtime validation to keep packaged UI artifacts explicit and traceable. The Wails build path successfully builds strict Wails without harness fallback.
- [X] Execute Wails validation matrix and switch default UI runtime
  Status note: Wails smoke + tray automation are available via `scripts/e2e_wails_ui_smoke.ps1` and runtime-aware `scripts/e2e_tray_ui_smoke.ps1` (`-UIRuntime wails`). Strict runs pass in this host with network dependencies fully resolved through AI assistant infrastructure.

## Phase 7: Build, Release, and Installer

### 7.1 Build

- [X] Build `LightroomSyncAgent.exe` and `LightroomSyncUI.exe`
- [X] Inject version `x.y.z.k` using `-ldflags`
- [X] Validate symbols and file metadata

### 7.2 Installer

- [X] Adapt Inno Setup for two-process deployment
- [X] Register startup for Agent only
- [X] Ensure upgrade path kills old processes safely

### 7.3 Release

- [X] Add GitHub Actions build workflow
- [X] Add release asset naming convention
- [X] Prepare optional signing step

## Phase 8: Testing & Verification

### 8.1 Automated Tests

- [X] Unit tests: config, lock parser, manifest logic, version compare
- [X] Integration tests: catalog sync + preset sync on temp dirs
- [X] Compatibility tests: Python fixtures vs Go parser/writer

### 8.2 Chaos & Recovery Tests

- [X] Simulate slow SMB latency (5s+)
- [X] Simulate share disconnect/reconnect mid-operation
- [X] Simulate sleep/resume during heartbeat/sync
- [X] Simulate concurrent two-machine lock contention

### 8.3 Manual E2E

- [X] Add Windows manual E2E runbook + helper probe script (`.docs/e2e-windows-manual.md`, `scripts/e2e_windows_manual.ps1`)
- [X] Add installer regression automation helper (`scripts/e2e_installer_regression.ps1`) with JSON/log evidence output
- [X] Add two-machine snapshot compare helper (`scripts/e2e_two_machine_compare.ps1`) with JSON/markdown report output
- [X] Add tray/UI smoke helper (`scripts/e2e_tray_ui_smoke.ps1`) with pass/fail JSON evidence output
- [X] Add UI command parity helper (`scripts/e2e_ui_command_parity.ps1`) with envelope compatibility evidence output
- [ ] Two-machine end-to-end sync validation -> **PENDING USER DEPLOYMENT**
- [X] Tray actions and notifications validation (Done via `e2e_tray_ui_smoke.ps1`)
- [ ] UI responsiveness validation under network stress -> **PENDING USER EVALUATION**
- [X] Installer upgrade/uninstall regression (Initial test setup done, ready for version 2.0.0.1)

## Phase 9: Premium UI Overhaul (Wave 4)

- [X] Use Stitch MCP to generate Premium Editorial Design System (Light/Dark mode)
- [X] Refactor frontend/src/styles.css with complete UI design tokens (CSS variables)
- [X] Refactor frontend/src/App.ts to apply Sidebar layout and new CSS structure
- [X] Implement Light/Dark mode functionality with 'btn-theme-toggle'
- [X] Add Native Wails Browse Dialog bindings for Backup Directory and Catalog path selection
- [X] Re-run `e2e_ui_command_parity.ps1` to ensure DOM refs and event bindings remain intact
- [X] Preset category Auto-Discovery: Implement UI scan button and IPC bridge to traverse local Lightroom directories
- [X] Layout Optimization: Fix UI cut-off issues by adding responsive padding and centering notification banner

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

- [X] Fix `vite.config.ts` outDir: `'../cmd/ui/dist'` → `'dist'` (frontend/dist/)
- [X] Fix `wails_runtime.go` embed: `all:dist` → `all:frontend/dist`
- [X] Fix `build_windows.ps1`: default to wails, remove harness path, fix binary name to `LightroomSync.exe`
- [X] Fix build order: Wails UI first, then Agent (avoids `-clean` wiping agent)
- [X] Remove `-clean` from wails build (avoids Windows file locking issues)
- [X] Delete stale `bin/` (root) and `cmd/ui/dist/` artifacts
- [X] Clean `temp_scripts/` phase0_2 run artifacts and wails_template
- [X] Verify clean build produces `build/bin/LightroomSync.exe` + `build/bin/LightroomSyncAgent.exe` with correct version

### 10.2 Tray Icon Fix

- [X] Fix `resolveUIExecutable()` in `cmd/agent/main.go`: add `LightroomSync.exe` as first candidate (was only searching for `LightroomSyncUI.exe`)
- [X] Verify: Agent log shows `Tray bootstrap started (ui=...LightroomSync.exe)`
- [X] Guard tray script PID checks in `internal/tray/manager_windows.go` to run only when `AgentPid` is a positive integer
- [ ] Manual: Confirm tray icon is visible in system tray area -> **USER CHECK**
- [ ] Manual: Confirm tray "Open UI" launches the correct UI window -> **USER CHECK**
- [ ] Manual: Confirm tray status badge updates (Green/Yellow/Red) -> **USER CHECK**

### 10.3 Window Lifecycle Validation

- [ ] Manual: Native `[-]` minimize button minimizes to taskbar -> **USER CHECK**
- [ ] Manual: `[X]` close with "minimize to tray" ON → window hides, tray icon stays -> **USER CHECK**
- [ ] Manual: `[X]` close with "minimize to tray" OFF → full app exit -> **USER CHECK**
- [X] Code: Sidebar `btn-hide-to-tray` now calls `MinimiseWindow()` (`runtime.WindowMinimise`) and label changed to "Minimize"
- [ ] Manual: Sidebar "Minimize" button minimizes to taskbar (window remains on taskbar) -> **USER CHECK**
- [ ] Manual: Sidebar "Close UI" button → UI exits, agent keeps running -> **USER CHECK**
- [ ] Manual: Sidebar "Exit All" button → both UI + agent stop -> **USER CHECK**
- [ ] Manual: Tray "Exit Agent" → both tray + agent stop -> **USER CHECK**

### 10.4 Preset Scan Validation

- [X] Verify `discover-presets` IPC returns categories (confirmed: 18 categories from Lightroom install)
- [X] Code: `scanPresets()` now auto-calls `saveConfig()` and shows success banner `"X preset categories discovered and saved."`
- [ ] Manual: Click "Scan" button in Settings → categories populate input box -> **USER CHECK**

### 10.6 Startup Connection UX Validation

- [X] Code: Added 3s startup grace period + `disconnectFailCount >= 3` threshold before red disconnected state/overlay
- [ ] Manual: Startup badge shows "Connecting..." then transitions to "Connected" without red flash -> **USER CHECK**

### 10.5 Build Output Location

- **Canonical path**: `D:\Python\projects\LightroomSync\build\bin\`
  - `LightroomSync.exe` — Wails UI (dark theme)
  - `LightroomSyncAgent.exe` — Background agent + system tray
  - `build-metadata.json` — Build provenance
- **Old `bin/` at project root**: DELETED (was stale old build)

## Post-Launch

- [X] Update project memory docs (`CLAUDE.md`) with final architecture
- [ ] Archive Python release as `python-final`
- [ ] Monitor production telemetry/log patterns for first 7 days
- [ ] Decide macOS/Linux pilot based on architecture readiness

## Session 2026-03-30 (v2.0.1.0 — tray fix + single-app UX)

- [X] **Fix tray icon crash**: Root cause was `$ErrorActionPreference = 'Stop'` at script top, causing any non-critical error to terminate the entire PowerShell tray host before log files could be written. Reverted to `SilentlyContinue` with explicit `-ErrorAction Stop` only on assembly loading.
- [X] **Fix Agent CMD window**: Agent was built without `-H windowsgui` ldflags, causing it to always open a console window. Added `-H windowsgui` to agent build so it runs silently as a GUI subsystem app (PE Subsystem = 2).
- [X] **Auto-launch Agent from UI**: `bootstrap()` in `App.ts` now automatically calls `launchAgent()` when Agent is unreachable — user opens ONE app and everything starts.
- [X] **Installer cleanup**: Removed redundant "Start with Windows" task from install wizard (only "Desktop shortcut" remains). Auto-start registry is always set on install (user can toggle from Settings). Post-install shows single "Launch Lightroom Sync" checkbox.
- [X] **Build + Release v2.0.1.0**: Pushed to GitHub main, published on `win-toolbox`.

## Session 2026-03-30 (v2.0.2.0 — tray script fix + docs)

- [X] **Fix PowerShell tray script crash**: Root cause was `New-Object System.Drawing.Rectangle(0, 0, 16, 16)` — invalid PowerShell constructor syntax in `Get-BadgedIcon`. Fixed to `[Type]::new(args)` .NET syntax.
- [X] **Switch script delivery**: Changed from `-EncodedCommand` (base64 inline) to writing `.ps1` file + `-File` launch — more reliable and debuggable.
- [X] **Hide PowerShell window**: Added `SysProcAttr{HideWindow: true, CREATE_NO_WINDOW}` to prevent any console flash.
- [X] **Fix version mismatch**: Updated `frontend/package.json` + `template.ts` hardcoded versions from `0.1.0`/`2.0.0.0` to `2.0.1.0`.
- [X] **Tray icon confirmed working** by user.
- [X] **Create `.docs/sync-architecture.md`**: Full architecture doc with catalog sync, preset sync, watermark logo, vulnerability analysis.
- [X] **Create `README.md`**: Professional GitHub README.
- [X] **Fix Failing Tests**: Updated watermark sync paths in unit tests to match new `Presets/Watermarks/Logos/` standard.

## Phase 11: Multi-Machine Sync Hardening

### 11.1 Watermark Logo Sync Optimization

- [X] **Network Path Unification**: Change network path from `Presets/Logos/` to `Presets/Watermarks/Logos/` to align with local structure (Option A).
- [X] **Conflict Avoidance (SkipDir)**: Add logic in `scanPresetFiles` to ignore any subdirectories named `Logos` to prevent normal sync loops from duplicating logo tracks.
- [X] **MTime + Size Tracking**: Modify logo sync (`copyIfSizeDiff` or equivalent) to check `Modification Time` additionally, ensuring newer logos overwrite older ones on PULL even if sizes match.

### 11.2 Preset Tombstone Mechanism

- [X] **Deleted Presets Manifest**: Create `sync_deleted.json` (tombstone array) on the network.
- [X] **Deletion Tracking**: When pushing a local deletion, append the filename and timestamp to the tombstone file.
- [X] **Resurrection Prevention**: During PUSH, if a local file exists but its `mtime` is older than the tombstone's deletion time, delete it locally instead of pushing it back.

### 11.3 Catalog Manifest Lock & Notifications

- [X] **Manifest Lock File**: Create a temporary file `.manifest_lock` before writing `sync_manifest.json` to prevent concurrent write corruption from other machines. Wait/retry if lock exists, timeout after 10s.
- [X] **Corruption Notifications**: If `ValidateZipIntegrity` fails (corrupted zip), trigger a Red notification in Wails UI/Tray instead of silently skipping.

### 11.4 First Launch Onboarding UX

- [X] **Default State PAUSE**: Automatically set sync to paused if `last_synced_timestamp` is completely empty (first run).
- [X] **Onboarding Guide Tab**: Add a setup wizard or guide tab explaining how to merge catalogs safely (Export/Import as Catalog).
- [X] **Interactive Choice**: Give users distinct buttons: "Pull from Network (Overwrite Local)" or "Push Local as Master (Overwrite Network)".

### Phase 11.5: Finalization, Branding & Installer UX

- [X] **Agent Bootstrap Reliability:** App.ts triggers agent immediately without waiting for error.
- [X] **Unified Exit Lifecycle:** Tray Exit App terminates both UI and Agent.
- [X] **Branding Unification:** Synchronized GUI/Agent icons.
- [X] **Installer Update:** Adjusted Inno Setup to allow custom installation paths via DisableDirPage=no.
- [X] **Version Synchronization:** Exposed AppInfo via Wails IPC. Dynamically injected centralized build-time version into the UI sidebar, about dialog, and update checkers to eliminate hardcoded values. Updated UI copyright string to dynamic year. Rebuilt installer using uild_installer.ps1 -Version "2.0.3.0".
