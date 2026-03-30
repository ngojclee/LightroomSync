# UI Command Contract Map (Tabs ↔ IPC)

> Purpose: khóa mapping giữa màn hình Wails và command contract để tránh drift trong Wave 3/4.

## Envelope Contract (keep stable)

Response shape used by UI layers:

- `ok`
- `id`
- `success`
- `code`
- `error`
- `data`
- `server_ts`

This envelope must stay backward-compatible with current CLI mode.

## Tab Mapping

### Status

- `ping`: determine initial connection state.
- `get_status`: render status snapshot.
- `sync_now`: manual sync trigger.
- `pause_sync` / `resume_sync`: sync control.

Primary fields:

- `status_text`, `tray_color`
- `lightroom_running`, `sync_in_progress`, `sync_paused`
- monitor error counters

### Settings

- `get_config`: load form defaults.
- `save_config`: persist form changes.

Primary fields:

- `backup_folder`, `catalog_path`
- startup flags
- intervals/timeouts
- preset sync options/categories

### Backups

- `get_backups`: list available backups.
- `sync_backup`: sync selected backup path.

Primary fields:

- `path`, `catalog_name`, `size`, `mod_time`

### Logs

- `subscribe_logs`: pull incremental log entries by cursor.

Primary fields:

- `entries[]`
- `last_id`

### Update

- `check_update`: fetch latest release metadata.
- `download_update`: start binary download.

Primary fields:

- `current_version`, `latest_version`, `has_update`
- `release_notes`, `asset_name`, `asset_url`
- `download_in_progress`, `destination_path`

## Error Handling Policy

1. `agent_offline`:
   - Show disconnected banner.
   - Keep UI interactive for local actions (tab switch/filter).
2. `bad_request`:
   - Show field-level or action-level validation message.
3. `internal_error` / `timeout`:
   - Show retry hint + keep previous data displayed if available.

## Drift Prevention Rules

1. New IPC commands must be added to this map before UI wiring.
2. Any envelope key change requires:
   - update this map
   - update Wave 2 parity tests
   - update automation scripts if impacted.
