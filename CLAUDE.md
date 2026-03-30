# LightroomSync - Developer Documentation

> **Last Updated**: 2026-03-30
> **Project**: A robust, two-machine synchronization application for Adobe Lightroom Classic catalogs and settings, built with Go and Wails (replacing legacy Python implementation). Designed to safely handle SMB network latency and concurrency through a single-flight background worker, separate from the Wails frontend.

---

## 🏗️ Architecture & Methods

### File Structure
LightroomSync/
├── cmd/
│   ├── agent/                 # Background daemon (Systray, Monitors, Sync Worker, IPC Server)
│   └── ui/                    # Wails application (Frontend runner, Native Dialog Hooks)
├── frontend/                  # React + TypeScript UI
│   ├── src/
│   │   ├── App.ts             # Sidebar Dashboard shell & bindings
│   │   ├── bridge.ts          # Wails vs Fallback mock IPC abstraction
│   │   └── styles.css         # CSS Variables (Premium Editorial Theme, Glassmorphism)
├── internal/
│   ├── config/                # YAML configuration parsing and validation
│   ├── lrc/                   # Lightroom tracking (lock file parsing, activity detection)
│   ├── sync/                  # Catalog & Preset single-flight copy logic
│   ├── uiapi/                 # Shared boundary handler for UI requests (IPC / Wails)
│   └── update/                # GitHub release fetcher & self-updating logic
├── .docs/                     # Master Plan (plan.md), Task tracking (task.md)
└── scripts/                   # Build and E2E testing helpers (Windows/PS1)

### Key Components
- **Agent (`cmd/agent`)**: Runs seamlessly in the background. Holds the authoritative state. Executes SMB network calls on a single queue to avoid Windows Explorer locks. Exposes Named Pipe IPC for the UI.
- **UI (`cmd/ui` & `frontend`)**: The Wails GUI. Extremely thin layer doing no direct disk/network operations. Relies on the Bridge to talk to `internal/uiapi`. Polls state via interval loops.
- **Sync Worker (`internal/sync`)**: Safely moves giant `.lrcat` files using robust streaming and progress updates. Detects identical hashes/sizes to skip redundant copies.

---

## 🗄️ Important Variables & State

- `wailsapp/v2/runtime` Context: The Wails context is captured in `cmd/ui/wails_app.go` (`Startup` method) and is required to launch Native Windows OS Dialogs (`wailsruntime.OpenDirectoryDialog`) without blocking the DOM.
- `Bridge` (`frontend/src/bridge.ts`): All frontend state queries (e.g., `GetConfig`, `GetBackups`) go through `window.bridge`.

---

## ⚠️ Known Patterns & Gotchas

### 1. IPC vs Direct Wails Binding
While the UI is rendered by Wails, it still behaves like the decoupled IPC client. Commands like `SaveConfig` go through `window.bridge` which invokes the Wails bound `Settings.SaveConfig` returning via `internal/uiapi`. The UI NEVER touches `os.ReadFile`.
### 2. single-flight Concurrency
SMB calls (GetBackups, Sync) can freeze the executing thread for 5+ seconds if the share drops. Therefore, `LightroomSyncUI.exe` never performs these directly. The Agent performs them.
### 3. Build Tags
Due to Wails embedding requirement, dual-build architecture is present. The Agent is built normally, but UI must be built with `-tags wails`. See `scripts/build_windows.ps1` for explicit runtime boundaries.
### 4. Versioning bump
Uses `x.y.z.k` across `package.json`, `wails.json`, `LightroomSync.iss`. Bumping handled globally.

---

## 🐛 Troubleshooting

- **UI Status Hanging**: Check if backend context is stuck inside an active SMB sync call. The UI polls every 5s (`bridge.ts` -> `startPolling`). If the Go JSON response blocks, the UI might show a loading spinner or stale data until the sync yields.
- **"wails runtime is not included in this build"**: Wails UI was built without `-tags wails`.
- **Directory/File Pickers not Opening**: Caused by missing Wails bound context. Make sure `wails_runtime_wails.go` successfully runs `WailsApp.Startup` hook.

---

**Conventions**: 
- Use PascalCase for Go structs bounding Wails.
- Do NOT add external third-party packing tools; the build relies on pure `go build` and Inno Setup `build_installer.ps1`.
- Maintain test coverage via E2E PowerShell smoke assertions (`scripts/e2e_tray_ui_smoke.ps1`).
