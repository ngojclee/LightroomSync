export type ActionCode =
  | "ok"
  | "bad_request"
  | "timeout"
  | "unknown_command"
  | "internal_error"
  | "agent_offline"
  | string;

export interface ActionEnvelope<T = unknown> {
  ok: boolean;
  id?: string;
  success: boolean;
  code?: ActionCode;
  error?: string;
  data?: T;
  server_ts?: string;
}

export interface AppStatus {
  tray_color?: string;
  status_text?: string;
  lightroom_running?: boolean;
  sync_in_progress?: boolean;
  sync_paused?: boolean;
  lightroom_monitor_errors?: number;
  backup_monitor_errors?: number;
  network_monitor_errors?: number;
  lock_monitor_errors?: number;
  last_resume_gap_seconds?: number;
  auto_sync?: boolean;
}

export interface ConfigSnapshot {
  backup_folder?: string;
  catalog_path?: string;
  start_with_windows?: boolean;
  start_minimized?: boolean;
  minimize_to_tray?: boolean;
  auto_sync?: boolean;
  heartbeat_interval?: number;
  check_interval?: number;
  lock_timeout?: number;
  max_catalog_backups?: number;
  preset_sync_enabled?: boolean;
  preset_categories?: string[];
}

export interface BackupInfo {
  path?: string;
  catalog_name?: string;
  size?: number;
  mod_time?: string;
}

export interface StreamLogEntry {
  id?: number;
  timestamp?: string;
  level?: string;
  message?: string;
}

export interface SubscribeLogsResult {
  entries?: StreamLogEntry[];
  last_id?: number;
}

export interface CheckUpdateResult {
  current_version?: string;
  latest_version?: string;
  has_update?: boolean;
  release_notes?: string;
  asset_url?: string;
  asset_name?: string;
  download_in_progress?: boolean;
}

