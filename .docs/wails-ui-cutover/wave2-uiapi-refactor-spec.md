# Wave 2 Spec — Shared `internal/uiapi` Refactor

> Phase: 6R / Wave 2  
> Goal: Tách toàn bộ action logic khỏi `cmd/ui/main.go` để dùng chung cho CLI mode và Wails backend.

## Why This Wave Matters

- Giảm rủi ro duplicated logic giữa CLI/harness/Wails.
- Dễ test parity từng command trước khi build frontend thật.
- Giữ một nguồn sự thật cho mapping IPC command + error envelope.

## Planned Package Layout

1. `internal/uiapi/service.go`
   - `Service` struct chứa `PipeName`, timeout config.
   - Public methods: `Ping`, `GetStatus`, `GetConfig`, `SaveConfig`, `GetBackups`, `SyncNow`, `SyncBackup`, `PauseSync`, `ResumeSync`, `SubscribeLogs`, `CheckUpdate`, `DownloadUpdate`.
2. `internal/uiapi/types.go`
   - Shared response envelope dùng cho CLI/Wails bridge.
   - Typed payload helper structs cho parse/validation.
3. `internal/uiapi/errors.go`
   - Error normalization helpers (map to `agent_offline`, `bad_request`, `internal_error`).
4. `internal/uiapi/service_test.go`
   - Unit tests for payload parse + response code mapping parity.

## Call-Site Refactor Plan

1. `cmd/ui/main.go`
   - Replace current `action*` function set with calls vào `internal/uiapi`.
   - Keep `runAction(action, payload, pipe)` signature for backward compatibility.
2. `cmd/ui/wails_app.go` (Wave 1/2 bridge)
   - Inject `uiapi.Service` instance và expose bind methods cho frontend.

## Command Parity Matrix (Must Preserve)

Actions to preserve exactly:

- `ping`
- `status`
- `get-config`
- `save-config`
- `get-backups`
- `sync-now`
- `sync-backup`
- `pause-sync`
- `resume-sync`
- `subscribe-logs`
- `check-update`
- `download-update`

Parity rules:

1. Response top-level keys unchanged: `ok`, `id`, `success`, `code`, `error`, `data`, `server_ts`.
2. Unsupported action still returns `bad_request`.
3. Offline agent still maps to `agent_offline`.
4. Payload parse errors still map to `bad_request`.

## Verification Strategy

### Baseline Capture (before refactor)

Run and keep outputs for comparison:

```powershell
.\build\bin\LightroomSyncUI.exe --action ping
.\build\bin\LightroomSyncUI.exe --action status
.\build\bin\LightroomSyncUI.exe --action get-config
.\build\bin\LightroomSyncUI.exe --action get-backups
```

### Post-Refactor Checks

```powershell
.\build\bin\LightroomSyncUI.exe --action ping
.\build\bin\LightroomSyncUI.exe --action status
.\build\bin\LightroomSyncUI.exe --action get-config
.\build\bin\LightroomSyncUI.exe --action save-config --payload "{}"
```

Expected:
- JSON envelope stable.
- Code mapping stable.
- No regressions in existing scripts (`e2e_windows_manual.ps1`, `e2e_tray_ui_smoke.ps1`).

## Rollback Strategy

- Keep previous `cmd/ui/main.go` action functions in one checkpoint commit before deleting.
- If regression detected:
  - restore old action path in `cmd/ui/main.go`
  - keep `internal/uiapi` behind feature flag until fixed.

## Out of Scope for Wave 2

- No full frontend tabs yet.
- No installer cutover.
- No default runtime switch.

## Execution Result (Current Session)

- Implemented:
  - `internal/uiapi` package (`types`, `service`, `errors`)
  - `cmd/ui/main.go` action dispatch migrated to `uiapi.Service`
  - `cmd/ui/wails_app.go` now calls shared `uiapi` service
  - parity automation helper `scripts/e2e_ui_command_parity.ps1`
- Verified:
  - `go test ./internal/uiapi` passes
  - command parity checks pass and emit report under `build/e2e/ui-command-parity-*.json`
