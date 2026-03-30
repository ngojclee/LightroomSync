# Wails UI Cutover — Execution Guide

> Companion docs:
> - [Plan](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/plan.md)
> - [Task](/d:/Python/projects/LightroomSync/.docs/wails-ui-cutover/task.md)

## Recommended Working Sequence

1. Complete Wave 1 + Wave 2 in a single coding batch (bootstrap + uiapi extraction).
2. Implement Wave 3 tab shell before deep styling/UX polish.
3. Wire Wave 4 data-flow after all tabs exist to avoid rework.
4. Integrate Wave 5 build pipeline only when Wails app is runnable.
5. Run Wave 6 validation and switch default runtime last.

## Command Checklist Per Wave

### Wave 1

```powershell
pwsh -File scripts/build_windows.ps1 -SkipTests
.\build\bin\LightroomSyncUI.exe --action ping
```

Expected:
- CLI action returns JSON.
- Wails shell can launch in dev mode.

### Wave 2

```powershell
.\build\bin\LightroomSyncUI.exe --action status
.\build\bin\LightroomSyncUI.exe --action get-config
```

Expected:
- Action responses are structurally identical to pre-refactor behavior.

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
pwsh -File scripts/build_windows.ps1 -Version 2.0.0.0
pwsh -File scripts/build_installer.ps1 -Version 2.0.0.0
```

Expected:
- UI binary + metadata generated.
- Installer includes intended UI artifact.

### Wave 6

```powershell
pwsh -File scripts/e2e_installer_regression.ps1
pwsh -File scripts/e2e_windows_manual.ps1 -Mode latency -Iterations 120 -IntervalMs 40 -P95TargetMs 100
```

Expected:
- Regression scripts pass.
- Manual matrix evidence complete for cutover signoff.

## Evidence Folder Convention

- Keep all validation outputs under `build/e2e/`.
- Preserve the latest pass set for:
  - `snapshot-*.json`
  - `latency-*.json`
  - `tray-ui-smoke-*.json`
  - `installer-regression-*.json`
