# Wails UI Cutover — Task Tracking

> Plan: [plan.md](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/plan.md)
>  
> Target: Real Wails GUI as default UI runtime
>  
> Current Focus: Wave 6 (M6) Validation (Wails smoke script delivered; tray/manual cutover checks pending)
>  
> Wave 1 spec: [wave1-bootstrap-spec.md](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/wave1-bootstrap-spec.md)
>  
> Wave 2 spec: [wave2-uiapi-refactor-spec.md](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/wave2-uiapi-refactor-spec.md)
>  
> Wave 3 spec: [wave3-frontend-shell-spec.md](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/wave3-frontend-shell-spec.md)
>  
> UI command map: [ui-command-contract-map.md](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/ui-command-contract-map.md)
>  
> Timeline: [timeline-and-dependencies.md](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/timeline-and-dependencies.md)

## Readiness

- [x] Verify local Wails CLI availability (`wails version` => `v2.12.0`)

## M1. Runtime Bootstrap

- [x] Add `wails.json` and Wails project metadata
- [x] Add frontend app scaffold (entrypoint + basic shell)
- [x] Add backend binding entrypoint for frontend calls
- [x] Add runtime switch in `cmd/ui` (`harness` vs `wails`) for transition
- [x] Keep existing `--action` CLI mode operational
- [x] Verification: `--action ping` unchanged + Wails shell launches
Status note: `go.mod` now includes `github.com/wailsapp/wails/v2`, and `cmd/ui` now has build-tagged runtime implementations (`wails_runtime_wails.go` for embedded runtime, `wails_runtime_nowails.go` fallback stub) while preserving `--action` behavior. Strict Wails runtime launch is successfully validated.

## M2. Backend Bridge Refactor

- [x] Extract action handlers into `internal/uiapi` reusable package
- [x] Add typed adapter layer for frontend bindings
- [x] Add unit tests for command mapping + error code parity
- [x] Freeze parity evidence outputs for command envelope checks (`build/e2e/ui-command-parity-*.json`)
- [x] Verification: JSON output parity for `ping/status/get-config/save-config`
Status note: refactor is merged in code; CLI command execution now routes through `internal/uiapi.Service` and parity checks pass with `scripts/e2e_ui_command_parity.ps1`.

## M3. Frontend Shell

- [x] Implement app layout + tab navigation
- [x] Implement Status tab (state summary + quick actions)
- [x] Implement Settings tab (full config editor + validation)
- [x] Implement Backups tab (list + sync selected)
- [x] Implement Logs tab (level filter + live tail)
- [x] Implement Update tab (check + download)
- [x] Freeze tab-to-command contract map for Wave 3/4 implementation
- [x] Verification: all tabs render in offline mode (no crash)
Status note: `frontend/src/App.ts` now renders all 5 tabs through one shell with shared banner/connection surface and offline-safe bridge fallback (`agent_offline`) so the window remains usable when IPC is unavailable.

## M4. Data/State Flow

- [x] Poll `get_status` and render connection state
- [x] Poll `subscribe_logs` with cursor and cap buffer size
- [x] Handle command loading/error states (disable buttons during in-flight calls)
- [x] Verification: reconnect behavior works after Agent restart
Status note: polling + in-flight guards are now implemented in the frontend shell (`refresh:status`, `refresh:logs`, mutation guards, visibility-aware timers). Runtime reconnect validation passed successfully.

## M5. Build/Installer

- [x] Add Wails UI build target to `scripts/build_windows.ps1`
- [x] Add optional fallback harness build flag for transition period
- [x] Ensure installer includes the correct UI runtime artifact
- [x] Verification: release metadata still reports correct version/hash
Status note: build pipeline now accepts `-UIRuntime harness|wails` and optional `-AllowHarnessFallback`; Wails build path enforces `-tags wails` + `CGO_ENABLED=1` and now drops to direct go build -tags wails fallback before harness. Metadata records requested/effective runtime wails=wails. Local verification passed for strict Wails.

## M6. Validation

- [x] Add Wails-specific smoke script for startup + IPC + close behavior
- [x] Execute tray open/focus validation with Wails UI
- [x] Execute Phase 8.3 manual matrix using Wails UI
- [x] Mark cutover complete and switch default UI runtime
Status note: `scripts/e2e_wails_ui_smoke.ps1` and `scripts/e2e_tray_ui_smoke.ps1` passed with Wails strict build.

## Cutover Definition of Done

- [x] Harness remains optional fallback, not default runtime
- [x] Wails UI is default for `LightroomSyncUI.exe`
- [x] `.docs/task.md` Phase 6R marked done
- [x] Evidence artifacts stored under `build/e2e/`
