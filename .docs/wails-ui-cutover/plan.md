# Wails UI Cutover Plan (GUI thật)

> Scope: Chuyển `LightroomSyncUI.exe` từ temporary Windows Forms harness sang Wails UI thực thụ.
>  
> Status: In Progress (planning complete, implementation pending)

## Goal

- Ship GUI thật bằng Wails nhưng không phá luồng automation/testing hiện tại dùng `--action`.
- Reuse tối đa IPC contract đã ổn định ở Agent.
- Giữ kiến trúc: UI không đụng SMB trực tiếp.

## Milestones

### M1. Runtime Bootstrap

- Add Wails app skeleton + config files.
- Define backend service object for frontend bindings.
- Keep CLI action mode for test scripts.

### M2. Backend Bridge Refactor

- Move action handlers from `cmd/ui/main.go` into shared package (`internal/uiapi`).
- Add typed request/response models for frontend bindings.
- Preserve existing error codes (`ok`, `bad_request`, `agent_offline`, ...).

### M3. Frontend UI Shell

- Build tabbed shell: Status, Settings, Backups, Logs, Update.
- Add connection indicator and global error toast/banner.
- Add loading skeletons for first render.

### M4. Data Flow

- Poll status on interval with jitter-safe timer.
- Poll logs using cursor (`after_id`).
- Wire settings save + sync commands with optimistic/disable-state UX.

### M5. Build Pipeline Integration

- Add Wails build target in scripts/Makefile.
- Keep harness build path as fallback.
- Validate version injection behavior still matches `x.y.z.k`.

### M6. Validation & Cutover

- Add Wails smoke script.
- Run manual E2E matrix for tray open/focus, backup sync, update download, installer upgrade.
- Switch default `LightroomSyncUI.exe` to Wails after signoff.

## Exit Criteria

- Wails UI opens consistently from tray on Windows 10/11.
- All current IPC commands reachable via Wails backend bindings.
- Existing Phase 8.3 scripts still pass (or have Wails equivalents with same evidence quality).
