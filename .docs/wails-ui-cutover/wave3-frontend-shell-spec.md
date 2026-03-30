# Wave 3 Spec — Frontend Shell & Tab Architecture

> Phase: 6R / Wave 3  
> Goal: Dựng GUI shell thật bằng Wails với đủ 5 tab nghiệp vụ, ưu tiên ổn định/offline-safe trước UI polish.

## Scope

- Implement shell layout + tab navigation.
- Implement first usable version cho: Status, Settings, Backups, Logs, Update.
- Add global connection/error surface dùng chung toàn app.

Out of scope:
- Deep styling/polish cuối.
- Build/installer cutover.
- Final smoke/regression matrix.

## Information Architecture

App regions:

1. Top bar:
   - App name/version
   - Agent connection indicator (`Connected` / `Disconnected`)
   - Last refresh timestamp
2. Tab nav:
   - Status
   - Settings
   - Backups
   - Logs
   - Update
3. Content pane:
   - Tab-specific UI.
4. Global feedback layer:
   - Inline warning banner for recoverable errors.
   - Toast/snackbar for command success/failure.

## Required Tab Capabilities (Wave 3 baseline)

### Status Tab

- Show: `status_text`, `tray_color`, `sync_in_progress`, `sync_paused`, monitor error counters.
- Quick actions: `Sync Now`, `Pause`, `Resume`.
- Show degraded/blocked state when agent offline.

### Settings Tab

- Editable fields mirroring `ConfigSnapshot`.
- Client-side validation hints (e.g., lock timeout >= heartbeat interval).
- Save action using same payload semantics as current harness.

### Backups Tab

- List backups from agent (`path`, `catalog_name`, `size`, `mod_time`).
- Select item + trigger sync selected backup.
- Refresh list action.

### Logs Tab

- Show rolling entries (`timestamp`, `level`, `message`).
- Level filter (ALL/INFO/WARN/ERROR/DEBUG).
- Clear view action (UI buffer reset only).

### Update Tab

- Show `current_version`, `latest_version`, `has_update`.
- Show release notes.
- Trigger `check_update` and `download_update`.

## IPC Command Mapping

Wave 3 must wire these commands:

- `get_status`, `sync_now`, `pause_sync`, `resume_sync`
- `get_config`, `save_config`
- `get_backups`, `sync_backup`
- `subscribe_logs`
- `check_update`, `download_update`

`ping` should be used for connection bootstrap/health check.

## UX Behavior Rules

1. Offline-first safety:
   - If IPC fails, tab remains rendered with disabled command buttons and clear error message.
2. In-flight command guard:
   - Disable the action button while request is running.
3. Deterministic tab state:
   - Switching tabs must not reset persisted form values unless explicit refresh.
4. Non-blocking UI:
   - Long-running agent actions must not freeze navigation or scrolling.

## Minimal Component Structure (suggested)

- `AppShell`
- `ConnectionBadge`
- `GlobalBanner`
- `StatusTab`
- `SettingsTab`
- `BackupsTab`
- `LogsTab`
- `UpdateTab`

## Acceptance Criteria

1. All 5 tabs render from a single Wails window without runtime crash.
2. Each tab can run at least one real command successfully when agent is reachable.
3. App stays usable when agent is offline (no white screen/no hard crash).
4. Command errors are visible to user in a consistent way.

## Verification Commands

```powershell
wails dev
.\build\bin\LightroomSyncUI.exe --action status
.\build\bin\LightroomSyncUI.exe --action get-config
.\build\bin\LightroomSyncUI.exe --action get-backups
```

Manual verify checklist:

- Open each tab once.
- Trigger at least one action per tab.
- Stop agent and verify UI shows disconnected state.
- Restart agent and verify UI can recover.
