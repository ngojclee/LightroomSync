# Lightroom Sync

**Đồng bộ Lightroom Catalog & Presets qua Network Share (NAS/SMB)**

> Two-process architecture: Background Agent (system tray) + Desktop UI (Wails)

![Version](https://img.shields.io/badge/version-2.0.7.202604071812-blue)
![Platform](https://img.shields.io/badge/platform-Windows%2064--bit-success)
![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)

---

## Features

### Catalog Sync
- Tự động detect backup mới trên network share
- Anti-self-sync: không sync lại backup của chính máy đó
- Pre-sync backup: tạo bản sao trước khi overwrite
- Chờ tự động nếu Lightroom đang mở

### Preset Sync (Two-way)
- Push & Pull: `Develop Presets`, `Export Presets`, `Watermarks`, `Metadata Presets`, `Filename Templates`
- Deletion-aware: xóa preset trên 1 máy → xóa trên tất cả
- Auto-discovery: scan categories từ thư mục Lightroom local
- Watermark logo extraction & path rewriting tự động

### System Tray Agent
- Background agent chạy ngầm, icon trên system tray
- Status badge (Green/Yellow/Red) theo trạng thái sync
- Khởi động cùng Windows
- IPC pipe communication với UI

### Desktop UI
- Wails-based native window (dark/light theme)
- Real-time status, log streaming, backup browser
- Settings với browse dialog cho paths
- Single-app UX: mở UI → Agent tự động khởi chạy

---

## Architecture

```
┌─────────────────────┐     \\NAS\Share\         ┌─────────────────────┐
│   LightroomSync.exe │ ◄── Catalog/*.zip ──►    │   LightroomSync.exe │
│   (Wails UI)        │ ◄── Presets/**    ──►    │   (Wails UI)        │
│         ▲           │     Presets/Logos/        │         ▲           │
│    IPC  │           │                          │    IPC  │           │
│         ▼           │                          │         ▼           │
│ LightroomSyncAgent  │                          │ LightroomSyncAgent  │
│   (System Tray)     │                          │   (System Tray)     │
└─────────────────────┘                          └─────────────────────┘
        Machine A                                       Machine B
```

- **Agent** (`LightroomSyncAgent.exe`): Background process, system tray, sync engine
- **UI** (`LightroomSync.exe`): Desktop app, communicates with Agent via Named Pipe IPC
- **Network Share**: NAS/SMB share as sync hub (no cloud dependency)

---

## Installation

### Option 1: Installer (Recommended)
1. Download `LightroomSyncSetup-v2.0.7.202604071812-windows-amd64.exe` from [Releases](https://github.com/ngojclee/win-toolbox/releases/tag/LightroomSync-v2.0.7.202604071812)
2. Run installer → auto-registers startup + creates shortcuts
3. Open **Lightroom Sync** from Start Menu or Desktop

### Option 2: Portable
1. Download `LightroomSyncAgent-v*.exe` + `LightroomSync-v*.exe`
2. Place in same directory
3. Run `LightroomSync.exe` → Agent starts automatically

---

## Configuration

Config file: `%LOCALAPPDATA%\LightroomSync\config.yaml`

| Setting | Description | Default |
|---------|-------------|---------|
| `backup_directory` | Network share path | `\\NAS\Share\Catalog` |
| `catalog_directory` | Local Lightroom catalog path | (auto-detected) |
| `machine_name` | Unique hostname for this machine | `%COMPUTERNAME%` |
| `auto_sync` | Auto-sync on startup | `true` |
| `sync_interval_seconds` | Polling interval | `60` |
| `max_backup_copies` | Retention count | `5` |
| `preset_categories` | Categories to sync | (auto-discovered) |
| `start_with_windows` | Register agent on boot | `true` |

---

## Sync Mechanisms

See [.docs/sync-architecture.md](.docs/sync-architecture.md) for detailed documentation.

### Catalog
- Manifest-based: each backup writes `sync_manifest.json`
- 3 anti-self-sync rules (machine name, timestamp, zip integrity)
- Pending sync if Lightroom is running

### Presets
- mtime-based two-way sync with 2s tolerance
- State file tracks deletions across machines
- 4-phase algorithm: Pull-Delete → Pull-New → Push-Delete → Push-New

### Watermark Logos
- Automatic logo extraction and path rewriting
- Logos centralized in `Presets/Logos/` on network share

---

## Build from Source

### Prerequisites
- Go 1.22+
- Wails CLI v2
- Node.js 18+
- Inno Setup 6 (for installer)

### Build
```powershell
# Build both UI + Agent
.\scripts\build_windows.ps1 -Version "2.0.7.202604071812"

# Build installer
.\scripts\build_installer.ps1 -Version "2.0.7.202604071812"
```

### Output
```
build/bin/
├── LightroomSync.exe          # Wails UI
├── LightroomSyncAgent.exe     # Background Agent
└── build-metadata.json        # Build provenance

build/installer/
└── LightroomSyncSetup-v2.0.7.202604071812-windows-amd64.exe
```

---

## Project Structure

```
LightroomSync/
├── cmd/agent/           # Agent entry point
├── frontend/src/        # Wails frontend (TypeScript)
├── internal/
│   ├── config/          # YAML config load/save
│   ├── coordinator/     # Sync orchestrator + event bus
│   ├── ipc/             # Named Pipe IPC
│   ├── monitor/         # Lightroom process + backup monitor
│   ├── sync/            # Catalog restore + preset sync + manifest
│   ├── tray/            # System tray (PowerShell WinForms)
│   └── update/          # Self-update checker
├── installer/           # Inno Setup script
├── scripts/             # Build + test scripts
└── .docs/               # Architecture + task tracking
```

---

## License

Private project — © ngojclee
