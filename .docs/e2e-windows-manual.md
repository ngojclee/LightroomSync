# Lightroom Sync — Windows Manual E2E Runbook

> Scope: Phase 8.3 manual validation for release candidate builds.
>  
> Helper script: `scripts/e2e_windows_manual.ps1`

## 1. Prerequisites

- Build binaries: `pwsh -File scripts/build_windows.ps1 -Version 2.0.0.0`
- Optional installer build: `pwsh -File scripts/build_installer.ps1 -Version 2.0.0.0`
- Prepare two Windows machines sharing the same SMB folder.
- Ensure both machines use their own Lightroom local catalog paths.

## 2. Quick Machine Snapshot (Both Machines)

Run on each machine and keep generated JSON:

```powershell
pwsh -File scripts/e2e_windows_manual.ps1 -Mode snapshot
```

Output files are created under `build/e2e/` as:

- `snapshot-<HOST>-<TIMESTAMP>.json`

Use these snapshots to compare live status/config/backup visibility between machine A and B.

## 3. UI Responsiveness Validation Under Stress

Start stress condition manually:

1. Keep Agent running.
2. While test is running, simulate network stress (disconnect SMB, reconnect, or inject delay).
3. Execute latency probe:

```powershell
pwsh -File scripts/e2e_windows_manual.ps1 -Mode latency -Iterations 120 -IntervalMs 40 -P95TargetMs 100
```

Pass criteria:

- `p95_ms <= 100`
- `failures == 0`

Report file:

- `build/e2e/latency-<HOST>-<TIMESTAMP>.json`

## 4. Two-Machine End-to-End Sync Validation

Checklist:

1. Machine A tạo backup mới (khi Lightroom đóng).
2. Verify machine A snapshot shows latest backup in `get_backups`.
3. Machine B nhận backup list và chạy `SyncBackup` từ UI.
4. Verify machine B catalog cập nhật, `LastSyncedTimestamp` thay đổi.
5. Confirm lock file remains parseable (`STATUS|MACHINE|TIMESTAMP`) and no corruption.
6. Repeat inverse direction (B -> A) to validate bi-directional workflow.

Automated compare helper (after collecting both snapshot files):

```powershell
pwsh -File scripts/e2e_two_machine_compare.ps1 `
  -SnapshotA build/e2e/snapshot-MACHINEA-<TIMESTAMP>.json `
  -SnapshotB build/e2e/snapshot-MACHINEB-<TIMESTAMP>.json
```

Optional latency validation in the same compare report:

```powershell
pwsh -File scripts/e2e_two_machine_compare.ps1 `
  -SnapshotA build/e2e/snapshot-MACHINEA-<TIMESTAMP>.json `
  -SnapshotB build/e2e/snapshot-MACHINEB-<TIMESTAMP>.json `
  -LatencyA build/e2e/latency-MACHINEA-<TIMESTAMP>.json `
  -LatencyB build/e2e/latency-MACHINEB-<TIMESTAMP>.json
```

Outputs:

- `build/e2e/two-machine-compare-<TIMESTAMP>.json`
- `build/e2e/two-machine-compare-<TIMESTAMP>.md`

## 5. Tray Actions & Notifications Validation

On one machine:

1. Right-click tray icon and run `Open UI`, verify UI focus behavior.
2. Run `Sync Now` from tray and confirm status/log stream updates.
3. Kill/restart UI and confirm Agent remains alive.
4. Exit Agent from tray and verify lock OFFLINE write + process shutdown.
5. Re-launch Agent and verify tray state returns correctly.

Automated smoke helper (IPC + tray status file + UI relaunch focus check):

```powershell
pwsh -File scripts/e2e_tray_ui_smoke.ps1
```

Headless fallback (skips UI focus assertion but keeps IPC/tray checks):

```powershell
pwsh -File scripts/e2e_tray_ui_smoke.ps1 -SkipUIFocus
```

Output file:

- `build/e2e/tray-ui-smoke-<HOST>-<TIMESTAMP>.json`

## 6. Installer Upgrade/Uninstall Regression

Using produced installer:

1. Install silently or via wizard.
2. Start Agent and UI, ensure both binaries launch from install directory.
3. Re-run installer (upgrade path) while Agent/UI are running.
4. Confirm old processes are terminated and files are replaced successfully.
5. Uninstall and verify:
   - startup registry entry removed
   - install directory cleaned
   - no orphan Agent process left

Automated helper (recommended before manual spot-check):

```powershell
pwsh -File scripts/e2e_installer_regression.ps1
```

Dry-run mode (to verify command flow without touching system state):

```powershell
pwsh -File scripts/e2e_installer_regression.ps1 -DryRun
```

Generated reports/logs are written to `build/e2e/`:

- `installer-regression-<TIMESTAMP>.json`
- `installer-install-<TIMESTAMP>.log`
- `installer-upgrade-<TIMESTAMP>.log`
- `installer-uninstall-<TIMESTAMP>.log`

## 7. Result Logging Template

Record final result in `.docs/task.md` and attach artifact paths:

- Snapshot files for machine A/B.
- Latency report JSON under stress.
- Installer logs (if executed).
- Notes for any blocker/regression.
