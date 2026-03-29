# Lightroom Sync — Go Rewrite Plan (Windows-First, Cross-OS Ready)

> **Version**: 1.1.0  
> **Date**: 2026-03-30  
> **Status**: Draft (Updated)  
> **Target Version**: v2.0.0.0

## Progress Snapshot (2026-03-30)

- Phase 0 foundations completed: fixtures + compatibility tests + IPC contract skeleton + platform boundary folders/tags.
- Phase 1 scaffold mostly completed: module layout, Agent/UI entrypoints, Makefile, binaries build.
- Phase 2 in progress: config migration and startup-registry wiring implemented; UI migration status signaling still pending.
- IPC implementation now has named-pipe server/client, timeout/error code contract, startup reconnect logic, and integration tests for ping/status roundtrip.
- Lock manager now has heartbeat loop with retry/backoff and best-effort OFFLINE write on shutdown.
- Event coordinator now includes a bounded single-flight sync worker queue, wired to `sync_now` IPC command.

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
