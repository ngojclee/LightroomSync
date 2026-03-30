# Lightroom Sync — Go Rewrite Plan (Windows-First, Cross-OS Ready)

> **Version**: 1.1.0  
> **Date**: 2026-03-30  
> **Status**: Draft (Updated)  
> **Target Version**: v2.0.0.0

## Progress Snapshot (2026-03-30)

- Phase 0 foundations completed: fixtures + compatibility tests + IPC contract skeleton + platform boundary folders/tags.
- Phase 1 scaffold mostly completed: module layout, Agent/UI entrypoints, Makefile, binaries build.
- Phase 2 baseline completed: config migration, startup-registry wiring, and UI migration hint signaling are implemented.
- IPC implementation now has named-pipe server/client, timeout/error code contract, startup reconnect logic, and integration tests for ping/status roundtrip.
- Lock manager now has heartbeat loop with retry/backoff and best-effort OFFLINE write on shutdown.
- Event coordinator now includes a bounded single-flight sync worker queue, wired to `sync_now` IPC command.
- Lightroom monitor loop is wired in Agent with edge-trigger started/stopped events and error logging hooks.
- UI now includes a temporary Windows Forms GUI harness (launched by `LightroomSyncUI.exe`) so IPC flows can be tested before Wails Phase 6.
- Backup monitor is now wired with recursive zip discovery, duplicate suppression by signature, and configurable polling interval.
- UI now enforces single-instance behavior and attempts to focus the existing window on relaunch.
- Agent now has managed goroutine lifecycle and graceful shutdown (context cancel, IPC close, OFFLINE lock best-effort, bounded wait for worker stop).
- Resilience layer now includes an operation watchdog (`op_id` + deadline), integrated with sync worker timeout alerts.
- Network resilience now includes a share-health circuit breaker with outage/recovery events and state restoration on reconnect.
- Sleep/resume resilience is now implemented via gap-based resume detection with forced post-resume network revalidation.
- Catalog core now has safe restore primitives: zip integrity validation, zip-slip protection, pre-extract cleanup, and wrapper-folder flatten support.
- Catalog orchestration now includes startup manifest check, pending sync while Lightroom is open, manifest write on local backup detection, and last-synced timestamp update on successful network sync.
- Retention policy now includes automatic pre-sync backup snapshots and zip retention cleanup for both pre-sync and network backup locations.
- App status now exposes monitor health metrics (error counters + last resume gap) for UI diagnostics.
- Preset sync core is now implemented with deletion-aware state tracking, push/pull reconciliation, and mtime tolerance to reduce false conflicts.
- Dynamic preset category discovery/filtering is now added with remote category bootstrap under `Presets/`.
- Watermark/logo sync compatibility is now implemented for `.lrtemplate` and `.lrsmv`, including logo copy dedupe (size check) and path rewrite for local/network contexts.
- Agent IPC now supports `GetConfig`/`SaveConfig` and `GetBackups`/`SyncBackup`, enabling richer GUI-driven end-to-end testing before full Wails UI.
- Temporary Windows Forms GUI harness now supports config read/save (auto-sync toggle), backup listing, and sync-selected backup actions.
- Sync worker pause/resume control is now wired end-to-end via IPC (`pause_sync`/`resume_sync`) and exposed in the temporary GUI harness for manual state testing.
- Agent now exposes `subscribe_logs` with in-memory cursor-based buffering (`after_id`, `limit`) and level filtering (`INFO/WARN/ERROR/DEBUG`); the temporary GUI harness now polls/renders logs in a dedicated panel with level selector and capped display buffer.
- Temporary GUI harness now includes a full settings form (paths, startup flags, sync intervals, preset options) with client-side validation and full `save_config` payload binding, plus live status/backup/log polling exclusively through Agent IPC.
- Update flow is now wired via IPC (`check_update` + `download_update`) with semantic version comparison, release-note/asset metadata, background download orchestration, and progress surfaced through the harness update panel + log stream.
- Phase 7.1 build pipeline is now automated via `scripts/build_windows.ps1` + `Makefile` build target, producing versioned Agent/UI binaries, verifying `--version` injection, and writing `build/bin/build-metadata.json` (sha256/size/timestamps).
- Phase 7.2 installer pipeline is now implemented with Inno Setup (`installer/LightroomSyncSetup.iss`) for two-process deployment (Agent + UI), startup registration for Agent only, and pre-install/uninstall process-stop hooks to keep upgrade/uninstall file replacement safe.
- Phase 7.3 release pipeline is now implemented via GitHub Actions (`.github/workflows/windows-release.yml`) with deterministic release asset naming (`LightroomSync-v<version>-windows-amd64`), artifact upload, tag-triggered GitHub Release publishing, and optional code-signing steps controlled by repository secrets.
- Phase 8.2 chaos coverage is now added with automated tests for slow SMB timeout handling, disconnect/reconnect recovery, sleep/resume-like stall recovery for heartbeat + sync worker, and two-machine lock contention; lock writes are now more robust via unique temp lock filenames and rename retry under contention.
- Phase 8.3 execution tooling is now added with a Windows manual E2E runbook (`.docs/e2e-windows-manual.md`) and a reusable probe script (`scripts/e2e_windows_manual.ps1`) for snapshot and latency evidence capture.
- Installer regression validation tooling is now added via `scripts/e2e_installer_regression.ps1`, including silent install/upgrade/uninstall flow checks and evidence artifacts (JSON + installer logs) under `build/e2e`.
- Two-machine validation aggregation tooling is now added via `scripts/e2e_two_machine_compare.ps1` to compare cross-host snapshots/latency reports and emit pass/fail evidence in JSON and markdown.
- Tray/UI smoke validation tooling is now added via `scripts/e2e_tray_ui_smoke.ps1`, covering IPC readiness, `sync_now` command reachability, tray status publication, and optional UI relaunch focus assertion.
- UI command-envelope parity tooling is now added via `scripts/e2e_ui_command_parity.ps1` to validate CLI/UI action envelope compatibility after refactors.
- Current desktop UI remains a temporary Windows Forms harness embedded in `cmd/ui/main.go`; Wails frontend/runtime cutover is now tracked as a dedicated remaining phase.
- Wails cutover planning is now expanded with execution waves, file-ownership map, quality gates, and step-by-step command checklist under `.docs/wails-ui-cutover/`.
- Wave 1 bootstrap planning is now formalized with a dedicated implementation spec (`.docs/wails-ui-cutover/wave1-bootstrap-spec.md`) and local readiness verification (`wails version` confirmed).
- Wave 2 `internal/uiapi` refactor planning is now formalized with command-parity spec and dependency/timeline map for the full cutover sequence.
- Wave 3 frontend shell planning is now formalized with tab architecture spec and tab-to-command contract map to lock UI/IPC alignment before implementation.
- Wave 1 implementation now includes build-tagged embedded runtime wiring (`wails_runtime_wails.go` / `wails_runtime_nowails.go`), `go.mod` Wails dependency declaration, runtime mode switch (`--runtime harness|wails`), Wails backend entrypoint, and frontend scaffold; `--action` flow remains compatible while strict Wails build/launch is currently blocked by host DNS/module-fetch constraints plus missing transitive `go.sum` entries on this offline host.
- Wave 2 implementation is now in code: command handlers are centralized in `internal/uiapi`, `cmd/ui` dispatch is simplified, and automated envelope parity evidence is generated via `scripts/e2e_ui_command_parity.ps1`.
- Wave 3/Wave 4 frontend baseline is now implemented in code: full tabbed shell (Status/Settings/Backups/Logs/Update), shared bridge adapter (`window.LightroomSyncBridge` + Wails global fallback), visibility-aware status/log polling, and in-flight request guards to prevent duplicate mutations while keeping UI responsive during agent reconnect.
- Wave 5 build/installer integration baseline is now implemented: runtime-aware build flags (`-UIRuntime harness|wails`) with optional fallback policy (`-AllowHarnessFallback`), a secondary direct `go build -tags wails` attempt before harness fallback, runtime provenance in `build-metadata.json` (`requested/effective/fallback/warnings`), and installer-side runtime validation/define propagation for deterministic packaging behavior.
- Wave 6 smoke baseline is now implemented with `scripts/e2e_wails_ui_smoke.ps1`, providing reproducible startup/IPC/close evidence for Wails runtime and explicit known-blocker capture mode for preflight-constrained or fallback-stub hosts.
- Wave 6 tray validation tooling is now runtime-aware: `scripts/e2e_tray_ui_smoke.ps1` supports `-UIRuntime wails` and blocker-aware focus evidence capture, plus `cmd/ui` now enforces single-instance/focus flow before runtime dispatch for both harness and wails modes.
- Agent now has a tray bootstrap module (`internal/tray`) with Windows NotifyIcon host, menu actions (`Open UI`, `Sync Now`, `Exit Agent`), and status label updates via shared status file.
- Lock manager now tracks internal `session_id` and monotonic `epoch` metadata for heartbeat sequencing while preserving legacy on-disk lock wire format (`STATUS|MACHINE|TIMESTAMP`).
- Phase 0.2 architecture spike automation is now added via `scripts/phase0_2_architecture_spike.ps1`, with runbook in `.docs/phase0-2-architecture-spike.md` to validate tray bootstrap + UI focus + IPC roundtrip.
- Integration validation now includes end-to-end temp-dir tests that combine catalog restore and preset round-trip sync in `internal/sync/integration_test.go`.

## 1. Motivation & Problem Statement

Ứng dụng Python hiện tại (v0.3.1.0) gặp các vấn đề kiến trúc cần rewrite:

| Vấn đề | Nguyên nhân gốc | Tại sao patch khó giải quyết triệt để |
|---|---|---|
| GUI freeze khi SMB chậm / sau sleep-resume | UI path vẫn chạm network share, polling chồng lấn, syscall SMB có thể block lâu | `root.after()` không giải quyết blocking I/O ở tầng OS/network redirector |
| RAM idle cao | CPython + Tkinter + runtime overhead | Giới hạn tự nhiên của mô hình interpreted app |
| `Access Denied`/AV friction lúc build & update | Binary reputation + scanner lock file handle trong quá trình ghi đè/cài đặt | Cần thay đổi packaging/release/signing strategy, không chỉ sửa logic app |

**Quyết định kiến trúc**: Rewrite core bằng Go, tách **Agent process** và **UI process**.

- `LightroomSyncAgent.exe` chạy 24/7, sở hữu toàn bộ monitoring/sync/tray.
- `LightroomSyncUI.exe` (Wails) chỉ mở khi cần cấu hình/log.
- UI không đọc SMB trực tiếp; chỉ giao tiếp với Agent qua IPC cục bộ.

## 2. Non-Goals & Scope

### 2.1 In Scope (v2.0.0.0)

- Feature parity với bản Python: catalog sync, preset sync, watermark/logo sync, lock + manifest.
- Windows-first release (production).
- Thiết kế module và interface để port sang macOS/Linux sau này.

### 2.2 Out of Scope (v2.0.0.0)

- Ship production binary cho macOS/Linux.
- Chuyển storage/coordination sang cloud backend.
- Thay đổi format dữ liệu network hiện tại.

## 3. Technology Stack

| Component | Technology | Reason |
|---|---|---|
| Core language | Go 1.22+ | Native binary, concurrency tốt, nhẹ |
| Agent tray | `getlantern/systray` (hoặc `energye/systray` nếu cần) | Ổn định cho tray loop |
| UI settings window | Wails v2 + vanilla TS | Dễ làm UI desktop mà vẫn nhanh |
| IPC (local) | Windows named pipe | Tách UI khỏi SMB I/O, low-latency local comms |
| Config | `yaml.v3` | Giữ schema YAML hiện có |
| Windows integration | `x/sys/windows` | Registry, mutex, process APIs |
| Notification | Windows toast lib | Native toast |
| Build/release | Go build + Inno Setup | Pipeline rõ ràng cho Windows |

## 4. Architecture Overview

### 4.1 Process Model

```text
┌────────────────────────────────────────┐
│ LightroomSyncAgent.exe                │
│ - Systray loop                        │
│ - Monitors (Lightroom, Backup, Lock)  │
│ - Sync worker (single-flight)         │
│ - State cache + log stream            │
│ - IPC server (named pipe)             │
└───────────────┬────────────────────────┘
                │ local IPC only
┌───────────────▼────────────────────────┐
│ LightroomSyncUI.exe (Wails)           │
│ - Settings / Backup Browser / Log      │
│ - Calls Agent API                      │
│ - No direct SMB operations             │
└────────────────────────────────────────┘
```

### 4.2 Core Design Rules

1. UI không truy cập network share.
2. Chỉ Agent truy cập SMB.
3. Một sync worker duy nhất (`single-flight`) để tránh xung đột.
4. Tất cả network operation đi qua retry policy + circuit breaker + health state.
5. Mọi format file trên network phải backward-compatible với Python version.

### 4.3 Concurrency Model (Agent)

```text
main goroutine
  ├── systray.Run()                    [blocks]
  ├── go runLightroomMonitor()         [ticker]
  ├── go runBackupMonitor()            [ticker]
  ├── go runLockHeartbeat()            [while LR running]
  ├── go runEventLoop()                [dispatch typed events]
  ├── go runSyncWorker()               [queue, 1 at a time]
  └── go runIPCServer()                [UI commands + state query]
```

### 4.4 SMB I/O Resilience Strategy

- `context.WithTimeout` dùng để đặt deadline orchestration, không giả định syscall luôn hủy được.
- Mỗi operation có `op_id` + watchdog.
- Exponential backoff với jitter cho retry.
- Circuit breaker khi share mất ổn định để tránh tự DOS.
- Sleep/resume hook: force revalidation trạng thái lock/manifest trước sync tiếp.

## 5. Project Structure

```text
d:\Python\projects\LightroomSync\
├── cmd/
│   ├── agent/
│   │   └── main.go
│   └── ui/
│       └── main.go
├── internal/
│   ├── config/               # yaml, defaults, validation, migration
│   ├── monitor/              # lightroom, backup, lock
│   ├── sync/                 # catalog, preset, watermark, manifest
│   ├── coordinator/          # event bus, worker queue, state machine
│   ├── ipc/                  # named pipe server/client contracts
│   ├── tray/                 # tray menu + icon state
│   └── platform/
│       ├── windows/          # registry, mutex, notifications
│       └── common/
├── frontend/                 # Wails UI assets
├── build/
├── installer/
└── .docs/
```

## 6. Backward Compatibility Contract

Trong giai đoạn chuyển đổi, Go và Python phải chạy đồng thời trên cùng network.

| Artifact | Compatibility Rule |
|---|---|
| `lightroom_lock.txt` | Giữ nguyên `STATUS|MACHINE|TIMESTAMP` |
| `sync_manifest.json` | Reader chấp nhận cả `zip_file` (legacy) và `zip_path` (nếu xuất hiện); writer mặc định ghi `zip_file` để tương thích ngược |
| `network_settings.json` | Giữ schema hiện tại |
| `preset_state.json` | Giữ semantics đồng bộ xóa/sửa |
| `LightroomSyncConfig.yaml` | Mapping schema tương thích + migration 1 lần từ location cũ |

## 7. Phasing

### Phase 0: Architecture Gates (must-pass)

- Khóa compatibility contract (`lock`, `manifest`, `state`) bằng test fixtures.
- Chốt IPC contract giữa Agent và UI.
- Chạy spike xác nhận lifecycle Agent + UI + tray không deadlock.

### Phase 1: Scaffold & Runtime Skeleton

- Tạo project Go theo cấu trúc `cmd/agent`, `cmd/ui`, `internal/*`.
- Systray skeleton + single instance guard.
- IPC skeleton: ping/status/log stream.

### Phase 2: Config & Windows Integration

- YAML config + defaults + validation.
- Legacy config migration.
- Start with Windows (registry), start minimized.

### Phase 3: Monitors & Coordination

- Lightroom monitor.
- Backup monitor.
- Lock heartbeat manager.
- Event loop + sync queue + state cache.

### Phase 4: Catalog Sync

- Safe unzip (zip-slip prevention).
- Cleanup + extract + retention + manifest rules.
- Pending sync orchestration.

### Phase 5: Preset Sync

- 2-way sync + deletion-aware state.
- Dynamic category discovery.
- Watermark/logo rewrite compatibility.

### Phase 6: UI (Wails)

- Settings, Backup Browser, Log, Update.
- UI gọi Agent qua IPC; không truy cập network trực tiếp.
- Real-time status/log events từ Agent.

### Phase 7: Build, Installer, Release

- Build scripts, version injection, installer adaptation.
- Signing-ready pipeline.
- E2E ổn định trên Windows.

## 8. Testing Strategy

| Level | Scope | Tooling |
|---|---|---|
| Unit | parser, manifest rules, config migration, lock logic | `go test` |
| Integration | sync flows với temp dirs + real zip | `go test` |
| Chaos | simulated slow SMB, disconnect/reconnect, sleep/resume | automated harness + manual |
| Compatibility | Go ↔ Python artifact tests | fixture + 2-machine test |
| E2E | tray/UI/installer/update lifecycle | manual scripted checklist |

## 9. Success Criteria (SLO-style)

- Agent idle RAM: `< 20 MB` trên Windows 11.
- UI responsiveness: `p95 < 100 ms` khi SMB artificial delay = 5s (vì UI không chạm SMB).
- Không có deadlock khi sleep/resume + reconnect.
- Không phá backward compatibility artifacts.
- Startup Agent `< 1s` trên máy mục tiêu.
- Full feature parity với Python version trước khi cutover.

## 10. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| SMB syscall treo lâu | watchdog + operation state + breaker, không block UI |
| Manifest schema drift giữa Python/Go | golden fixtures + compatibility CI tests |
| Tray/UI process desync | authoritative state ở Agent + reconnect protocol |
| AV/SmartScreen friction | signing-ready release flow, stable installer path, no packers |
| Portability debt khi code Windows-specific | `internal/platform` + build tags từ đầu |

## 11. Remaining Steps To Real GUI (Wails Cutover)

Current state: core IPC/backend is production-grade, but `LightroomSyncUI.exe` is still running a temporary Windows Forms harness.  
To ship the real GUI, we need **6 implementation steps**:

1. **Wails App Bootstrap**
   - Add Wails runtime project files (`wails.json`, frontend app shell, bindable backend entrypoint).
   - Keep current `--action` CLI pathway for automation scripts.
2. **UI API Extraction**
   - Extract `cmd/ui` action handlers into shared package (`internal/uiapi`) used by both CLI mode and Wails backend.
   - Preserve contract parity with existing named-pipe IPC commands.
3. **Frontend Shell + Navigation**
   - Implement real tabs/pages (Status/Settings/Backups/Logs/Update) in Wails frontend.
   - Add connection-state banner and common error boundary.
4. **State + Poll/Event Wiring**
   - Wire periodic status polling + log cursor polling + command mutation flows.
   - Prevent duplicate in-flight requests and ensure cancel-safe unmount behavior.
5. **Build/Installer Cutover**
   - Extend build scripts to produce Wails UI binary deterministically.
   - Keep fallback harness target during transition until signoff is complete.
6. **Validation + Switch Default**
   - Add Wails-specific smoke checks.
   - Run Phase 8.3 manual matrix with Wails UI and switch default UI target after pass.
