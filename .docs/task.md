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
- [ ] Implement architecture spike proving: Agent tray + UI launch/focus + IPC roundtrip

### 0.3 Platform Boundary

- [x] Create `internal/platform/windows` and `internal/platform/common`
- [x] Add build tags for Windows-specific code from day 1
- [x] Ensure core sync code compiles without Windows-only imports

## Phase 1: Scaffold & Runtime Skeleton

### 1.1 Project Bootstrap

- [ ] Install Go 1.22+ and Wails CLI
- [x] Initialize module `github.com/ngojclee/lightroom-sync`
- [x] Create structure: `cmd/agent`, `cmd/ui`, `internal/*`, `frontend/`
- [x] Add `.gitignore` for Go/Wails/build artifacts
- [x] Add `Makefile` or `build.ps1` with `agent`, `ui`, `all` targets

### 1.2 Agent Skeleton

- [x] Implement `cmd/agent/main.go` with single-instance mutex
- [ ] Add tray bootstrap (`internal/tray`) with status label + menu items
- [ ] Add graceful shutdown flow (stop workers, write OFFLINE, quit tray)

### 1.3 UI Skeleton

- [x] Implement `cmd/ui/main.go` (Wails window only)
- [x] UI startup checks Agent availability via IPC ping
- [ ] Add window focus behavior when already-open UI instance exists

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
- [ ] Add monitor health metrics and error logs

### 3.2 Lock Manager

- [x] Implement parser/writer for `STATUS|MACHINE|TIMESTAMP`
- [x] Implement atomic write: write temp + rename
- [ ] Add optional `session_id` and `epoch` internally (do not break legacy file format)
- [x] Heartbeat loop with retry/backoff policy

### 3.3 Backup Monitor

- [ ] Implement recursive zip discovery
- [ ] Track last-seen signature to avoid duplicate emits
- [ ] Make polling interval configurable

### 3.4 Resilience Layer

- [ ] Add operation watchdog with `op_id` and deadline
- [ ] Add circuit breaker for unstable network share
- [ ] Add reconnect recovery workflow after share returns
- [ ] Add sleep/resume handler: force state revalidation post-resume

### 3.5 Event Coordinator

- [x] Implement typed event bus + buffered channel
- [x] Add single sync worker queue (one-at-a-time)
- [x] Maintain authoritative app state cache for tray/UI reads

## Phase 4: Catalog Sync

### 4.1 Core Sync

- [ ] Validate zip integrity before extract
- [ ] Implement zip-slip protection (canonical destination path check)
- [ ] Cleanup old catalog artifacts before extraction
- [ ] Support wrapper-folder zip layouts

### 4.2 Manifest Logic

- [x] Implement manifest read/write with compatibility keys
- [x] Implement anti-self-sync rules
- [x] Verify zip existence + size before sync
- [x] Add structured reason codes for skip/allow decisions

### 4.3 Orchestration

- [ ] Initial startup manifest check
- [ ] Pending sync queue when Lightroom is running
- [ ] Write manifest after local backup creation
- [ ] Update `LastSyncedTimestamp` only on successful sync completion

### 4.4 Retention

- [ ] Implement pre-sync backup folder policy
- [ ] Implement retention cleanup for pre-sync and network backups

## Phase 5: Preset & Watermark Sync

### 5.1 Preset Sync

- [ ] Implement push + pull with deletion-aware state
- [ ] Use mtime tolerance to reduce false conflict
- [ ] Commit state file only after successful sync cycle

### 5.2 Category Discovery

- [ ] Scan Lightroom preset directories dynamically
- [ ] Filter categories using user config
- [ ] Ensure remote category folder bootstrap

### 5.3 Watermark/Logo

- [ ] Parse `.lrtemplate` and `.lrsmv` image paths
- [ ] Copy logos to shared `Logos/` folder
- [ ] Rewrite paths for local and network contexts
- [ ] Skip redundant copy when size/hash unchanged

## Phase 6: UI (Wails, IPC-Driven)

### 6.1 Settings Tab

- [ ] Implement config form and validation messages
- [ ] Bind actions to Agent IPC (`SaveConfig`, `SyncNow`, `RefreshStatus`)
- [ ] Render live status without direct filesystem calls

### 6.2 Backup Browser Tab

- [ ] Fetch list from Agent (`GetBackups`)
- [ ] Implement sync-selected workflow (`SyncBackup`)
- [ ] Add pause/resume sync control via Agent state

### 6.3 Log Tab

- [ ] Subscribe to log stream from Agent
- [ ] Implement level filters + max buffer limit

### 6.4 Update Tab

- [ ] Show current/latest version and release notes
- [ ] Trigger update flow via Agent
- [ ] Render progress events

## Phase 7: Build, Release, and Installer

### 7.1 Build

- [ ] Build `LightroomSyncAgent.exe` and `LightroomSyncUI.exe`
- [ ] Inject version `x.y.z.k` using `-ldflags`
- [ ] Validate symbols and file metadata

### 7.2 Installer

- [ ] Adapt Inno Setup for two-process deployment
- [ ] Register startup for Agent only
- [ ] Ensure upgrade path kills old processes safely

### 7.3 Release

- [ ] Add GitHub Actions build workflow
- [ ] Add release asset naming convention
- [ ] Prepare optional signing step

## Phase 8: Testing & Verification

### 8.1 Automated Tests

- [x] Unit tests: config, lock parser, manifest logic, version compare
- [ ] Integration tests: catalog sync + preset sync on temp dirs
- [x] Compatibility tests: Python fixtures vs Go parser/writer

### 8.2 Chaos & Recovery Tests

- [ ] Simulate slow SMB latency (5s+)
- [ ] Simulate share disconnect/reconnect mid-operation
- [ ] Simulate sleep/resume during heartbeat/sync
- [ ] Simulate concurrent two-machine lock contention

### 8.3 Manual E2E

- [ ] Two-machine end-to-end sync validation
- [ ] Tray actions and notifications validation
- [ ] UI responsiveness validation under network stress
- [ ] Installer upgrade/uninstall regression

## Post-Launch

- [ ] Update project memory docs (`CLAUDE.md`) with final architecture
- [ ] Archive Python release as `python-final`
- [ ] Monitor production telemetry/log patterns for first 7 days
- [ ] Decide macOS/Linux pilot based on architecture readiness
