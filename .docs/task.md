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
- [ ] Bootstrap Wails runtime + keep `--action` CLI compatibility
- [ ] Extract reusable UI API bridge from `cmd/ui` into `internal/uiapi`
- [ ] Implement Wails frontend tabs (Status/Settings/Backups/Logs/Update)
- [ ] Wire polling/event flows and robust in-flight/error handling
- [ ] Integrate Wails artifact into build/installer pipeline
- [ ] Execute Wails validation matrix and switch default UI runtime

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
- [ ] Two-machine end-to-end sync validation
- [ ] Tray actions and notifications validation
- [ ] UI responsiveness validation under network stress
- [ ] Installer upgrade/uninstall regression

## Post-Launch

- [ ] Update project memory docs (`CLAUDE.md`) with final architecture
- [ ] Archive Python release as `python-final`
- [ ] Monitor production telemetry/log patterns for first 7 days
- [ ] Decide macOS/Linux pilot based on architecture readiness
