# Wave 1 Spec — Wails Runtime Bootstrap

> Phase: 6R / Wave 1  
> Goal: Có Wails runtime chạy được nhưng vẫn giữ nguyên hành vi CLI `--action`.

## Preconditions

- Wails CLI available (`wails version`).
- Existing UI automation scripts still depend on `--action`.
- Agent IPC contract is already stable.

## Planned File Changes

1. `wails.json`
   - Define app name, output binary name, frontend dir, dev server settings.
2. `frontend/`
   - `package.json`, `tsconfig.json`, `vite.config.ts` (hoặc equivalent minimal scaffold).
   - `index.html`, `src/main.ts`, `src/App.ts`, `src/styles.css`.
3. `cmd/ui/main.go`
   - Keep `--action` behavior unchanged.
   - Add runtime mode switch:
     - default: `harness` during transition
     - optional: `--runtime wails`
4. `cmd/ui/wails_app.go` (new)
   - Wails-bound app struct + startup/shutdown hooks.
   - Minimal exported methods for health/ping placeholder.

## Acceptance Criteria

1. CLI parity:
   - `LightroomSyncUI.exe --action ping` output JSON shape unchanged.
2. Wails runtime boot:
   - `wails dev` opens window successfully.
   - frontend renders shell text (placeholder is fine for Wave 1).
3. No packaging regression:
   - Existing `go build ./cmd/ui` still works.
4. Transition-safe:
   - If Wails runtime errors, harness mode can still run.

## Verification Commands

```powershell
wails version
pwsh -File scripts/build_windows.ps1 -SkipTests
.\build\bin\LightroomSyncUI.exe --action ping
```

Dev-run check:

```powershell
wails dev
```

## Rollback Strategy

- Keep harness entrypoint untouched and guarded by runtime switch.
- If Wave 1 breaks startup, fallback to:
  - run with `--runtime harness`
  - or temporarily disable Wails path in `cmd/ui/main.go`.

## Out of Scope for Wave 1

- No real tab UI implementation yet.
- No `internal/uiapi` extraction yet.
- No installer/build pipeline cutover yet.
