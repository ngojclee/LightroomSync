# Phase 0.2 Architecture Spike (Agent tray + UI focus + IPC)

## Purpose
This spike validates the process/IPC contract by proving:

1. Agent starts and tray bootstrap is active.
2. UI can perform IPC roundtrip (`ping`, `status`) against Agent.
3. UI single-instance focus behavior works (second launch focuses existing window).

## Automation Script

- Script: [scripts/phase0_2_architecture_spike.ps1](/d:/Python/projects/LightroomSync/scripts/phase0_2_architecture_spike.ps1)

## Recommended Run Sequence (Windows)

1. Build latest binaries:

```powershell
cd D:\Python\projects\LightroomSync
make all
```

2. Run full spike check:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts\phase0_2_architecture_spike.ps1
```

3. Optional headless-friendly run (skip UI focus interaction):

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts\phase0_2_architecture_spike.ps1 -SkipUIFocus
```

## Pass Criteria

- `steps.tray_bootstrap.ok = true`
- `steps.ipc_roundtrip.ok = true`
- `steps.ui_launch_focus.ok = true`
- `overall_ok = true`

The script outputs a JSON summary and exits with code `0` on full pass.
