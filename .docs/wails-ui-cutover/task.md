# Wails UI Cutover — Task Tracking

> Plan: [plan.md](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/plan.md)
>  
> Target: Real Wails GUI as default UI runtime

## M1. Runtime Bootstrap

- [ ] Add Wails project config files and baseline app bootstrap
- [ ] Add backend binding entrypoint for frontend calls
- [ ] Keep existing `--action` CLI mode operational

## M2. Backend Bridge Refactor

- [ ] Extract action handlers into `internal/uiapi` reusable package
- [ ] Add typed adapter layer for frontend bindings
- [ ] Add unit tests for command mapping + error code parity

## M3. Frontend Shell

- [ ] Implement app layout + tab navigation
- [ ] Implement Status tab (state summary + quick actions)
- [ ] Implement Settings tab (full config editor + validation)
- [ ] Implement Backups tab (list + sync selected)
- [ ] Implement Logs tab (level filter + live tail)
- [ ] Implement Update tab (check + download)

## M4. Data/State Flow

- [ ] Poll `get_status` and render connection state
- [ ] Poll `subscribe_logs` with cursor and cap buffer size
- [ ] Handle command loading/error states (disable buttons during in-flight calls)

## M5. Build/Installer

- [ ] Add Wails UI build target to `scripts/build_windows.ps1`
- [ ] Add optional fallback harness build flag for transition period
- [ ] Ensure installer includes the correct UI runtime artifact

## M6. Validation

- [ ] Add Wails-specific smoke script for startup + IPC + close behavior
- [ ] Execute tray open/focus validation with Wails UI
- [ ] Execute Phase 8.3 manual matrix using Wails UI
- [ ] Mark cutover complete and switch default UI runtime
