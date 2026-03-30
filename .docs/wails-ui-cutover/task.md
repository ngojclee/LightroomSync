# Wails UI Cutover — Task Tracking

> Plan: [plan.md](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/plan.md)
>  
> Target: Real Wails GUI as default UI runtime
>  
> Current Focus: Wave 5 (M5) Build/Installer integration (runtime-aware pipeline delivered; host-specific installer compile still pending)
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
- [ ] Verification: `--action ping` unchanged + Wails shell launches
Status note: `--action ping` pass confirmed from `build/bin/LightroomSyncUI.exe`; `--runtime wails` currently blocked because Wails CLI reports `Unable to find Wails in go.mod` in this offline environment.

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
- [ ] Verification: reconnect behavior works after Agent restart
Status note: polling + in-flight guards are now implemented in the frontend shell (`refresh:status`, `refresh:logs`, mutation guards, visibility-aware timers). Runtime reconnect validation with real Wails window is pending because npm/wails preflight is still blocked in this environment.

## M5. Build/Installer

- [x] Add Wails UI build target to `scripts/build_windows.ps1`
- [x] Add optional fallback harness build flag for transition period
- [x] Ensure installer includes the correct UI runtime artifact
- [x] Verification: release metadata still reports correct version/hash
Status note: build pipeline now accepts `-UIRuntime harness|wails` and optional `-AllowHarnessFallback`; metadata records requested/effective runtime and fallback warnings. `build_installer.ps1` now validates runtime from metadata and passes runtime defines into Inno Setup. Local verification passed for `build_windows.ps1` (harness + wails fallback path); installer compile is pending on a machine with `ISCC.exe`.

## M6. Validation

- [ ] Add Wails-specific smoke script for startup + IPC + close behavior
- [ ] Execute tray open/focus validation with Wails UI
- [ ] Execute Phase 8.3 manual matrix using Wails UI
- [ ] Mark cutover complete and switch default UI runtime

## Cutover Definition of Done

- [ ] Harness remains optional fallback, not default runtime
- [ ] Wails UI is default for `LightroomSyncUI.exe`
- [ ] `.docs/task.md` Phase 6R marked done
- [ ] Evidence artifacts stored under `build/e2e/`
