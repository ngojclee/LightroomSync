# Wails UI Cutover — Execution Guide

> Companion docs:
> - [Plan](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/plan.md)
> - [Task](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/task.md)
> - [Wave 1 Spec](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/wave1-bootstrap-spec.md)
> - [Wave 2 Spec](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/wave2-uiapi-refactor-spec.md)
> - [Wave 3 Spec](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/wave3-frontend-shell-spec.md)
> - [UI Command Map](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/ui-command-contract-map.md)
> - [Timeline](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/timeline-and-dependencies.md)

## Recommended Working Sequence

1. Complete Wave 1 + Wave 2 in a single coding batch (bootstrap + uiapi extraction).
2. Implement Wave 3 tab shell before deep styling/UX polish.
3. Wire Wave 4 data-flow after all tabs exist to avoid rework.
4. Integrate Wave 5 build pipeline only when Wails app is runnable.
5. Run Wave 6 validation and switch default runtime last.

## Command Checklist Per Wave

### Wave 1

```powershell
wails version
pwsh -File scripts/build_windows.ps1 -SkipTests
.\build\bin\LightroomSyncUI.exe --action ping
```

Expected:
- CLI action returns JSON.
- Wails shell can launch in dev mode.
Current blocker:
- Wails CLI currently stops at preflight (`Unable to find Wails in go.mod`) in this environment; keep harness runtime as default until Wave 1 unblock.

### Wave 2

```powershell
.\build\bin\LightroomSyncUI.exe --action status
.\build\bin\LightroomSyncUI.exe --action get-config
.\build\bin\LightroomSyncUI.exe --action save-config --payload "{}"
pwsh -File scripts/e2e_ui_command_parity.ps1
```

Expected:
- Action responses are structurally identical to pre-refactor behavior.
- Error/code mapping remains stable (`bad_request`, `agent_offline`, `ok`).

### Wave 3 + Wave 4

```powershell
pwsh -File scripts/e2e_windows_manual.ps1 -Mode snapshot
pwsh -File scripts/e2e_tray_ui_smoke.ps1 -SkipUIFocus
```

Expected:
- Status/config/backups/log flows visible from Wails UI.
- Smoke report produced in `build/e2e`.

### Wave 5

```powershell
pwsh -File scripts/build_windows.ps1 -Version 2.0.0.0 -UIRuntime harness
pwsh -File scripts/build_windows.ps1 -Version 2.0.0.0 -UIRuntime wails -AllowHarnessFallback
pwsh -File scripts/build_installer.ps1 -Version 2.0.0.0 -UIRuntime wails -AllowHarnessFallback
```

Expected:
- UI binary + metadata generated.
- Metadata captures `ui_runtime_requested/effective` for traceability.
- Installer includes intended UI artifact.

### Wave 6

```powershell
pwsh -File scripts/e2e_wails_ui_smoke.ps1
pwsh -File scripts/e2e_wails_ui_smoke.ps1 -AcceptKnownPreflightBlocker
pwsh -File scripts/e2e_installer_regression.ps1
pwsh -File scripts/e2e_windows_manual.ps1 -Mode latency -Iterations 120 -IntervalMs 40 -P95TargetMs 100
```

Expected:
- Wails smoke report generated (`wails-ui-smoke-*.json`) with startup/IPC/close checks.
- Regression scripts pass.
- Manual matrix evidence complete for cutover signoff.
Environment note:
- In hosts where Wails preflight is still blocked, use `-AcceptKnownPreflightBlocker` to capture explicit blocker evidence while keeping the run auditable.

## Evidence Folder Convention

- Keep all validation outputs under `build/e2e/`.
- Preserve the latest pass set for:
  - `snapshot-*.json`
  - `latency-*.json`
  - `tray-ui-smoke-*.json`
  - `installer-regression-*.json`
