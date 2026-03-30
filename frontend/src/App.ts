import { executeAction, selectDirectory, selectFile, exitApplication, launchAgent, syncMinimizeToTray, discoverPresets } from "./bridge";
import { appTemplate } from "./template";
import type {
  ActionEnvelope,
  AppStatus,
  BackupInfo,
  CheckUpdateResult,
  ConfigSnapshot,
  StreamLogEntry,
  SubscribeLogsResult
} from "./types";

type TabKey = "status" | "settings" | "backups" | "logs" | "update" | "about";
type BannerKind = "error" | "info" | "success";

interface InvokeOptions {
  quietError?: boolean;
}

interface RefreshOptions {
  quietError?: boolean;
  silent?: boolean;
}

interface UIState {
  activeTab: TabKey;
  connected: boolean;
  connectionDetail: string;
  lastRefresh: string;
  status: AppStatus;
  config: ConfigSnapshot;
  backups: BackupInfo[];
  selectedBackup: string;
  logs: StreamLogEntry[];
  logsLastId: number;
  logLevel: string;
  update: CheckUpdateResult;
  presetCategoryOptions: string[];
  disconnectFailCount: number;
  overlayDismissed: boolean;
}

interface Refs {
  banner: HTMLDivElement;
  bannerText: HTMLSpanElement;
  connectionBadge: HTMLSpanElement;
  connectionDetail: HTMLSpanElement;
  lastRefresh: HTMLSpanElement;
  tabButtons: HTMLButtonElement[];
  tabPanels: HTMLDivElement[];
  statusText: HTMLSpanElement;
  trayColor: HTMLSpanElement;
  syncProgress: HTMLSpanElement;
  syncPaused: HTMLSpanElement;
  monitorErrors: HTMLSpanElement;
  btnSyncNow: HTMLButtonElement;
  btnPauseSync: HTMLButtonElement;
  btnResumeSync: HTMLButtonElement;
  btnRefreshStatus: HTMLButtonElement;
  inputBackupFolder: HTMLInputElement;
  inputCatalogPath: HTMLInputElement;
  chkStartWithWindows: HTMLInputElement;
  chkStartMinimized: HTMLInputElement;
  chkMinimizeToTray: HTMLInputElement;
  chkAutoSync: HTMLInputElement;
  inputHeartbeat: HTMLInputElement;
  inputCheckInterval: HTMLInputElement;
  inputLockTimeout: HTMLInputElement;
  inputMaxBackups: HTMLInputElement;
  chkPresetSyncEnabled: HTMLInputElement;
  inputPresetCategories: HTMLInputElement;
  presetChecklist: HTMLDivElement;
  btnPresetSelectAll: HTMLButtonElement;
  btnPresetClear: HTMLButtonElement;
  btnScanPresets: HTMLButtonElement;
  btnGetConfig: HTMLButtonElement;
  btnSaveConfig: HTMLButtonElement;
  btnBrowseBackup: HTMLButtonElement;
  btnBrowseCatalog: HTMLButtonElement;
  backupsSelect: HTMLSelectElement;
  backupsHelper: HTMLDivElement;
  btnRefreshBackups: HTMLButtonElement;
  btnSyncSelected: HTMLButtonElement;
  logLevelSelect: HTMLSelectElement;
  btnRefreshLogs: HTMLButtonElement;
  btnClearLogs: HTMLButtonElement;
  logsOutput: HTMLPreElement;
  updateCurrentVersion: HTMLSpanElement;
  updateLatestVersion: HTMLSpanElement;
  updateHasUpdate: HTMLSpanElement;
  updateNotes: HTMLTextAreaElement;
  btnCheckUpdate: HTMLButtonElement;
  btnForceDownloadLatest: HTMLButtonElement;
  btnDownloadUpdate: HTMLButtonElement;
  btnExitApp: HTMLButtonElement;
  agentOverlay: HTMLDivElement;
  btnLaunchAgent: HTMLButtonElement;
  btnDismissOverlay: HTMLButtonElement;
  agentOverlayStatus: HTMLParagraphElement;
}

function envValue(key: string, fallback = "unknown"): string {
  const value = (globalThis as Record<string, unknown>)[key];
  if (typeof value === "string" && value.trim() !== "") {
    return value;
  }
  return fallback;
}

function nowTime(): string {
  return new Date().toLocaleTimeString();
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return null;
}

function asString(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function asBoolean(value: unknown, fallback = false): boolean {
  return typeof value === "boolean" ? value : fallback;
}

function asNumber(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function formatBytes(size: number): string {
  if (!Number.isFinite(size) || size <= 0) {
    return "-";
  }
  const units = ["B", "KB", "MB", "GB"];
  let value = size;
  let idx = 0;
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024;
    idx += 1;
  }
  return `${value.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`;
}

function formatDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value || "-";
  }
  return `${date.toLocaleDateString()} ${date.toLocaleTimeString()}`;
}

function toInt(input: string, fallback: number): number {
  const parsed = Number.parseInt(input, 10);
  if (!Number.isFinite(parsed)) {
    return fallback;
  }
  return parsed;
}

class FrontendShell {
  private readonly state: UIState;
  private readonly refs: Refs;
  private readonly pipeName: string;
  private readonly version: string;
  private readonly connectingGraceMs = 3000;
  private readonly bootstrapStartedAt = Date.now();
  private readonly inFlight = new Set<string>();
  private readonly disposeHandlers: Array<() => void> = [];
  private statusTimer: number | undefined;
  private logsTimer: number | undefined;
  private disposed = false;

  constructor(private readonly root: HTMLElement) {
    this.pipeName = envValue("LIGHTROOMSYNC_PIPE", "\\\\.\\pipe\\LightroomSyncIPC");
    this.version = envValue("LIGHTROOMSYNC_UI_VERSION", "dev");
    this.state = {
      activeTab: "status",
      connected: false,
      connectionDetail: "Not connected",
      lastRefresh: "-",
      status: {},
      config: {
        start_with_windows: false,
        start_minimized: false,
        minimize_to_tray: true,
        auto_sync: false,
        heartbeat_interval: 30,
        check_interval: 60,
        lock_timeout: 120,
        max_catalog_backups: 5,
        preset_sync_enabled: true,
        preset_categories: ["Export Presets", "Develop Presets", "Watermarks"]
      },
      backups: [],
      selectedBackup: "",
      logs: [],
      logsLastId: 0,
      logLevel: "ALL",
      update: {},
      presetCategoryOptions: ["Develop Presets", "Export Presets", "Watermarks"],
      disconnectFailCount: 0,
      overlayDismissed: false
    };

    this.root.innerHTML = this.template();
    this.refs = this.collectRefs();
    this.bindEvents();
    this.updateTabVisibility();
    this.renderConnection();
    this.renderStatus();
    this.renderConfig();
    this.renderBackups();
    this.renderLogs();
    this.renderUpdate();
    this.bindLifecycleEvents();
    void this.bootstrap();
  }

  private template(): string {
    return appTemplate;
  }

  private normalizeCategories(values: string[]): string[] {
    const seen = new Set<string>();
    const output: string[] = [];
    values.forEach((raw) => {
      const value = raw.trim();
      if (value === "") {
        return;
      }
      const key = value.toLowerCase();
      if (seen.has(key)) {
        return;
      }
      seen.add(key);
      output.push(value);
    });
    return output.sort((a, b) => a.localeCompare(b));
  }

  private parseCategoriesInput(value: string): string[] {
    return this.normalizeCategories(
      value
        .split(",")
        .map((item) => item.trim())
        .filter((item) => item.length > 0)
    );
  }

  private syncCategoryInputFromChecklist(): void {
    const checked = Array.from(
      this.refs.presetChecklist.querySelectorAll<HTMLInputElement>('input[type="checkbox"]:checked')
    ).map((checkbox) => checkbox.value);
    this.refs.inputPresetCategories.value = this.normalizeCategories(checked).join(", ");
  }

  private renderPresetChecklist(): void {
    const selected = new Set(this.parseCategoriesInput(this.refs.inputPresetCategories.value).map((it) => it.toLowerCase()));
    const options = this.normalizeCategories([
      ...this.state.presetCategoryOptions,
      ...this.parseCategoriesInput(this.refs.inputPresetCategories.value)
    ]);

    this.state.presetCategoryOptions = options;
    this.refs.presetChecklist.innerHTML = "";

    if (options.length === 0) {
      const empty = document.createElement("p");
      empty.className = "text-[11px] text-on-surface-variant/70";
      empty.textContent = "Click Scan to load preset folders.";
      this.refs.presetChecklist.appendChild(empty);
      return;
    }

    options.forEach((category, index) => {
      const label = document.createElement("label");
      label.className = "flex items-center gap-2 px-2 py-1 rounded-md border border-outline-variant/20 bg-surface-container-low text-xs text-on-surface cursor-pointer";

      const checkbox = document.createElement("input");
      checkbox.type = "checkbox";
      checkbox.value = category;
      checkbox.id = `preset-cat-${index}`;
      checkbox.className = "h-3.5 w-3.5 rounded border-outline-variant/40 bg-surface-container-lowest";
      checkbox.checked = selected.has(category.toLowerCase());
      checkbox.addEventListener("change", () => this.syncCategoryInputFromChecklist());

      const text = document.createElement("span");
      text.textContent = category;

      label.appendChild(checkbox);
      label.appendChild(text);
      this.refs.presetChecklist.appendChild(label);
    });
  }

  private collectRefs(): Refs {
    const byId = <T extends HTMLElement>(id: string): T => {
      const node = this.root.querySelector<T>(`#${id}`);
      if (!node) {
        throw new Error(`Missing required element: ${id}`);
      }
      return node;
    };

    return {
      banner: byId<HTMLDivElement>("banner"),
      bannerText: byId<HTMLSpanElement>("banner-text"),
      connectionBadge: byId<HTMLSpanElement>("connection-badge"),
      connectionDetail: byId<HTMLSpanElement>("connection-detail"),
      lastRefresh: byId<HTMLSpanElement>("last-refresh"),
      tabButtons: Array.from(this.root.querySelectorAll<HTMLButtonElement>(".tab-button")),
      tabPanels: Array.from(this.root.querySelectorAll<HTMLDivElement>(".tab-panel")),
      statusText: byId<HTMLSpanElement>("status-text"),
      trayColor: byId<HTMLSpanElement>("status-tray-color"),
      syncProgress: byId<HTMLSpanElement>("status-sync-progress"),
      syncPaused: byId<HTMLSpanElement>("status-sync-paused"),
      monitorErrors: byId<HTMLSpanElement>("status-monitor-errors"),
      btnSyncNow: byId<HTMLButtonElement>("btn-sync-now"),
      btnPauseSync: byId<HTMLButtonElement>("btn-pause-sync"),
      btnResumeSync: byId<HTMLButtonElement>("btn-resume-sync"),
      btnRefreshStatus: byId<HTMLButtonElement>("btn-refresh-status"),
      inputBackupFolder: byId<HTMLInputElement>("cfg-backup-folder"),
      inputCatalogPath: byId<HTMLInputElement>("cfg-catalog-path"),
      chkStartWithWindows: byId<HTMLInputElement>("cfg-start-with-windows"),
      chkStartMinimized: byId<HTMLInputElement>("cfg-start-minimized"),
      chkMinimizeToTray: byId<HTMLInputElement>("cfg-minimize-to-tray"),
      chkAutoSync: byId<HTMLInputElement>("cfg-auto-sync"),
      inputHeartbeat: byId<HTMLInputElement>("cfg-heartbeat"),
      inputCheckInterval: byId<HTMLInputElement>("cfg-check-interval"),
      inputLockTimeout: byId<HTMLInputElement>("cfg-lock-timeout"),
      inputMaxBackups: byId<HTMLInputElement>("cfg-max-backups"),
      chkPresetSyncEnabled: byId<HTMLInputElement>("cfg-preset-sync-enabled"),
      inputPresetCategories: byId<HTMLInputElement>("cfg-preset-categories"),
      presetChecklist: byId<HTMLDivElement>("cfg-preset-checklist"),
      btnPresetSelectAll: byId<HTMLButtonElement>("btn-preset-select-all"),
      btnPresetClear: byId<HTMLButtonElement>("btn-preset-clear"),
      btnScanPresets: byId<HTMLButtonElement>("btn-scan-presets"),
      btnGetConfig: byId<HTMLButtonElement>("btn-get-config"),
      btnSaveConfig: byId<HTMLButtonElement>("btn-save-config"),
      btnBrowseBackup: byId<HTMLButtonElement>("btn-browse-backup"),
      btnBrowseCatalog: byId<HTMLButtonElement>("btn-browse-catalog"),
      backupsSelect: byId<HTMLSelectElement>("backups-list"),
      backupsHelper: byId<HTMLDivElement>("backups-helper"),
      btnRefreshBackups: byId<HTMLButtonElement>("btn-refresh-backups"),
      btnSyncSelected: byId<HTMLButtonElement>("btn-sync-selected"),
      logLevelSelect: byId<HTMLSelectElement>("logs-level"),
      btnRefreshLogs: byId<HTMLButtonElement>("btn-refresh-logs"),
      btnClearLogs: byId<HTMLButtonElement>("btn-clear-logs"),
      logsOutput: byId<HTMLPreElement>("logs-output"),
      updateCurrentVersion: byId<HTMLSpanElement>("upd-current-version"),
      updateLatestVersion: byId<HTMLSpanElement>("upd-latest-version"),
      updateHasUpdate: byId<HTMLSpanElement>("upd-has-update"),
      updateNotes: byId<HTMLTextAreaElement>("upd-release-notes"),
      btnCheckUpdate: byId<HTMLButtonElement>("btn-check-update"),
      btnForceDownloadLatest: byId<HTMLButtonElement>("btn-force-download-latest"),
      btnDownloadUpdate: byId<HTMLButtonElement>("btn-download-update"),
      btnExitApp: byId<HTMLButtonElement>("btn-exit-app"),
      agentOverlay: byId<HTMLDivElement>("agent-overlay"),
      btnLaunchAgent: byId<HTMLButtonElement>("btn-launch-agent"),
      btnDismissOverlay: byId<HTMLButtonElement>("btn-dismiss-overlay"),
      agentOverlayStatus: byId<HTMLParagraphElement>("agent-overlay-status")
    };
  }

  private bindEvents(): void {
    this.refs.tabButtons.forEach((button) => {
      button.addEventListener("click", () => {
        const tab = button.dataset.tab as TabKey | undefined;
        if (!tab) {
          return;
        }
        this.state.activeTab = tab;
        this.updateTabVisibility();
      });
    });

    this.refs.btnRefreshStatus.addEventListener("click", () => void this.refreshStatus());
    this.refs.btnSyncNow.addEventListener("click", () => void this.runStatusAction("sync-now"));
    this.refs.btnPauseSync.addEventListener("click", () => void this.runStatusAction("pause-sync"));
    this.refs.btnResumeSync.addEventListener("click", () => void this.runStatusAction("resume-sync"));

    this.refs.btnGetConfig.addEventListener("click", () => void this.refreshConfig());
    this.refs.btnSaveConfig.addEventListener("click", () => void this.saveConfig());
    this.refs.btnScanPresets.addEventListener("click", () => void this.scanPresets());
    this.refs.btnPresetSelectAll.addEventListener("click", () => {
      this.refs.presetChecklist.querySelectorAll<HTMLInputElement>('input[type="checkbox"]').forEach((checkbox) => {
        checkbox.checked = true;
      });
      this.syncCategoryInputFromChecklist();
    });
    this.refs.btnPresetClear.addEventListener("click", () => {
      this.refs.presetChecklist.querySelectorAll<HTMLInputElement>('input[type="checkbox"]').forEach((checkbox) => {
        checkbox.checked = false;
      });
      this.syncCategoryInputFromChecklist();
    });
    this.refs.inputPresetCategories.addEventListener("input", () => {
      this.renderPresetChecklist();
    });
    this.refs.btnBrowseBackup.addEventListener("click", async () => {
      const dir = await selectDirectory("Select Backup Destination Directory");
      if (dir) {
        this.refs.inputBackupFolder.value = dir;
      }
    });
    this.refs.btnBrowseCatalog.addEventListener("click", async () => {
      const file = await selectFile("Select Lightroom Catalog File", "*.lrcat");
      if (file) {
        this.refs.inputCatalogPath.value = file;
      }
    });

    this.refs.btnRefreshBackups.addEventListener("click", () => void this.refreshBackups());
    this.refs.btnSyncSelected.addEventListener("click", () => void this.syncSelectedBackup());
    this.refs.backupsSelect.addEventListener("change", () => {
      this.state.selectedBackup = this.refs.backupsSelect.value;
    });

    this.refs.btnRefreshLogs.addEventListener("click", () => void this.refreshLogs(true));
    this.refs.btnClearLogs.addEventListener("click", () => {
      this.state.logs = [];
      this.state.logsLastId = 0;
      this.renderLogs();
      this.setBanner("success", "Cleared local log buffer.");
    });
    this.refs.logLevelSelect.addEventListener("change", () => {
      this.state.logLevel = this.refs.logLevelSelect.value;
      this.state.logs = [];
      this.state.logsLastId = 0;
      this.renderLogs();
      void this.refreshLogs(true);
    });

    this.refs.btnCheckUpdate.addEventListener("click", () => void this.refreshUpdate());
    this.refs.btnForceDownloadLatest.addEventListener("click", () => void this.forceDownloadLatest());
    this.refs.btnDownloadUpdate.addEventListener("click", () => void this.downloadUpdate());

    // Handle Title update per tab
    this.refs.tabButtons.forEach((button) => {
      button.addEventListener("click", () => {
        const tab = button.dataset.tab;
        const viewTitle = this.root.querySelector("#view-title");
        if (viewTitle && tab) {
            viewTitle.textContent = tab.charAt(0).toUpperCase() + tab.slice(1) + " Dashboard";
        }
      });
    });

    // --- Lifecycle button ---
    this.refs.btnExitApp.addEventListener("click", () => {
      void exitApplication();
    });

    this.refs.btnLaunchAgent.addEventListener("click", () => void this.handleLaunchAgent());
    this.refs.btnDismissOverlay.addEventListener("click", () => {
      this.state.overlayDismissed = true;
      this.hideAgentOverlay();
    });
  }

  private bindLifecycleEvents(): void {
    const onVisibilityChange = () => {
      if (document.hidden) {
        this.stopPolling();
        return;
      }
      this.startPolling();
      void this.refreshStatus({ quietError: true, silent: true });
      void this.refreshLogs(false, { quietError: true, silent: true });
    };
    document.addEventListener("visibilitychange", onVisibilityChange);
    this.disposeHandlers.push(() => document.removeEventListener("visibilitychange", onVisibilityChange));

    const onBeforeUnload = () => this.dispose();
    window.addEventListener("beforeunload", onBeforeUnload);
    this.disposeHandlers.push(() => window.removeEventListener("beforeunload", onBeforeUnload));
  }

  private updateTabVisibility(): void {
    this.refs.tabButtons.forEach((button) => {
      const tab = button.dataset.tab as TabKey | undefined;
      button.classList.toggle("is-active", tab === this.state.activeTab);
    });
    this.refs.tabPanels.forEach((panel) => {
      const matches = panel.id === `panel-${this.state.activeTab}`;
      panel.classList.toggle("is-hidden", !matches);
      panel.classList.toggle("hidden", !matches);
    });
  }

  private setBanner(kind: BannerKind, message: string): void {
    this.refs.banner.className = `banner banner-${kind}`;
    this.refs.bannerText.textContent = message;
  }

  private clearBanner(): void {
    this.refs.banner.className = "banner banner-hidden";
    this.refs.bannerText.textContent = "";
  }

  private startPolling(): void {
    this.stopPolling();
    if (this.disposed || document.hidden) {
      return;
    }

    this.statusTimer = window.setInterval(() => {
      void this.refreshStatus({ quietError: true, silent: true });
    }, 2500);

    this.logsTimer = window.setInterval(() => {
      void this.refreshLogs(false, { quietError: true, silent: true });
    }, 3000);
  }

  private stopPolling(): void {
    if (this.statusTimer !== undefined) {
      window.clearInterval(this.statusTimer);
      this.statusTimer = undefined;
    }
    if (this.logsTimer !== undefined) {
      window.clearInterval(this.logsTimer);
      this.logsTimer = undefined;
    }
  }

  private dispose(): void {
    if (this.disposed) {
      return;
    }
    this.disposed = true;
    this.stopPolling();
    this.inFlight.clear();
    while (this.disposeHandlers.length > 0) {
      const handler = this.disposeHandlers.pop();
      try {
        handler?.();
      } catch {
        // Best-effort lifecycle cleanup.
      }
    }
  }

  private setConnection(connected: boolean, detail: string): void {
    this.state.connected = connected;
    this.state.connectionDetail = detail;
    this.renderConnection();
  }

  private markRefresh(): void {
    this.state.lastRefresh = nowTime();
    this.refs.lastRefresh.textContent = this.state.lastRefresh;
  }

  private isInConnectingGracePeriod(): boolean {
    return Date.now() - this.bootstrapStartedAt < this.connectingGraceMs;
  }

  private shouldShowDisconnectedState(): boolean {
    return !this.state.connected && !this.isInConnectingGracePeriod() && this.state.disconnectFailCount >= 3;
  }

  private renderConnection(): void {
    if (this.state.connected) {
      this.refs.connectionBadge.textContent = "Connected";
      this.refs.connectionBadge.className = "badge connected";
    } else if (this.shouldShowDisconnectedState()) {
      this.refs.connectionBadge.textContent = "Disconnected";
      this.refs.connectionBadge.className = "badge disconnected";
    } else {
      this.refs.connectionBadge.textContent = "Connecting...";
      this.refs.connectionBadge.className = "badge connecting";
    }
    this.refs.connectionDetail.textContent = this.state.connectionDetail;
  }

  private renderStatus(): void {
    this.refs.statusText.textContent = asString(this.state.status.status_text, "-");
    this.refs.trayColor.textContent = asString(this.state.status.tray_color, "-");
    this.refs.syncProgress.textContent = String(asBoolean(this.state.status.sync_in_progress, false));
    this.refs.syncPaused.textContent = String(asBoolean(this.state.status.sync_paused, false));
    const errors = [
      `LR: ${asNumber(this.state.status.lightroom_monitor_errors, 0)}`,
      `Backup: ${asNumber(this.state.status.backup_monitor_errors, 0)}`,
      `Network: ${asNumber(this.state.status.network_monitor_errors, 0)}`,
      `Lock: ${asNumber(this.state.status.lock_monitor_errors, 0)}`
    ].join(" | ");
    this.refs.monitorErrors.textContent = errors;
  }

  private renderConfig(): void {
    const cfg = this.state.config;
    this.refs.inputBackupFolder.value = asString(cfg.backup_folder);
    this.refs.inputCatalogPath.value = asString(cfg.catalog_path);
    this.refs.chkStartWithWindows.checked = asBoolean(cfg.start_with_windows);
    this.refs.chkStartMinimized.checked = asBoolean(cfg.start_minimized);
    this.refs.chkMinimizeToTray.checked = asBoolean(cfg.minimize_to_tray, true);
    this.refs.chkAutoSync.checked = asBoolean(cfg.auto_sync);
    this.refs.inputHeartbeat.value = String(asNumber(cfg.heartbeat_interval, 30));
    this.refs.inputCheckInterval.value = String(asNumber(cfg.check_interval, 60));
    this.refs.inputLockTimeout.value = String(asNumber(cfg.lock_timeout, 120));
    this.refs.inputMaxBackups.value = String(asNumber(cfg.max_catalog_backups, 5));
    this.refs.chkPresetSyncEnabled.checked = asBoolean(cfg.preset_sync_enabled, true);
    this.state.presetCategoryOptions = this.normalizeCategories([
      ...this.state.presetCategoryOptions,
      ...(cfg.preset_categories ?? [])
    ]);
    this.refs.inputPresetCategories.value = this.normalizeCategories(cfg.preset_categories ?? []).join(", ");
    this.renderPresetChecklist();
  }

  private renderBackups(): void {
    const preferredSize = Math.max(10, Math.min(18, this.state.backups.length + 2));
    this.refs.backupsSelect.size = preferredSize;
    this.refs.backupsSelect.innerHTML = "";
    if (this.state.backups.length === 0) {
      this.refs.backupsSelect.size = 10;
      const option = new Option("(No backups found)", "");
      this.refs.backupsSelect.add(option);
      this.refs.backupsSelect.value = "";
      this.state.selectedBackup = "";
      this.refs.backupsHelper.textContent = "No backups loaded.";
      return;
    }

    this.state.backups.forEach((backup) => {
      const path = asString(backup.path);
      const catalog = asString(backup.catalog_name, "Unknown");
      const size = formatBytes(asNumber(backup.size, 0));
      const mod = formatDate(asString(backup.mod_time));
      const label = `${catalog} | ${size} | ${mod}`;
      const option = new Option(label, path);
      this.refs.backupsSelect.add(option);
    });

    if (!this.state.selectedBackup || !this.state.backups.some((it) => asString(it.path) === this.state.selectedBackup)) {
      this.state.selectedBackup = asString(this.state.backups[0].path);
    }
    this.refs.backupsSelect.value = this.state.selectedBackup;
    this.refs.backupsHelper.textContent = `${this.state.backups.length} backup(s) loaded.`;
  }

  private renderLogs(): void {
    if (this.state.logs.length === 0) {
      this.refs.logsOutput.textContent = "(empty)";
      return;
    }

    const lines = this.state.logs.map((entry) => {
      const ts = formatDate(asString(entry.timestamp));
      const level = asString(entry.level, "INFO");
      const message = asString(entry.message, "");
      return `[${ts}] [${level}] ${message}`;
    });
    this.refs.logsOutput.textContent = lines.join("\n");
  }

  private renderUpdate(): void {
    this.refs.updateCurrentVersion.textContent = asString(this.state.update.current_version, "-");
    this.refs.updateLatestVersion.textContent = asString(this.state.update.latest_version, "-");
    this.refs.updateHasUpdate.textContent = String(asBoolean(this.state.update.has_update, false));
    this.refs.updateNotes.value = asString(this.state.update.release_notes, "");
    this.refs.btnForceDownloadLatest.disabled =
      asString(this.state.update.asset_url, "") === "" ||
      asBoolean(this.state.update.download_in_progress, false);
    this.refs.btnDownloadUpdate.disabled =
      !asBoolean(this.state.update.has_update, false) ||
      asString(this.state.update.asset_url, "") === "" ||
      asBoolean(this.state.update.download_in_progress, false);
  }

  private async invoke(action: string, payload = "", options: InvokeOptions = {}): Promise<ActionEnvelope> {
    const envelope = await executeAction(action, payload);
    if (this.disposed) {
      return envelope;
    }
    this.markRefresh();

    if (!envelope.ok) {
      this.state.disconnectFailCount += 1;
      this.setConnection(false, envelope.error ?? "Agent unavailable");
      if (this.shouldShowDisconnectedState()) {
        this.showAgentOverlay();
      }
      if (!options.quietError) {
        this.setBanner("error", `${action}: ${envelope.error ?? "Failed to reach agent."}`);
      }
      return envelope;
    }

    this.setConnection(true, "Agent reachable via IPC");
    if (!envelope.success) {
      if (!options.quietError) {
        this.setBanner("error", `${action}: ${envelope.error ?? envelope.code ?? "Command failed."}`);
      }
      return envelope;
    }

    if (!options.quietError) {
      this.clearBanner();
    }
    // Reset disconnect counter on success
    this.state.disconnectFailCount = 0;
    if (this.state.overlayDismissed === false) {
      this.hideAgentOverlay();
    }
    return envelope;
  }

  private async withInFlight<T>(key: string, runner: () => Promise<T>): Promise<T | undefined> {
    if (this.disposed || this.inFlight.has(key)) {
      return undefined;
    }
    this.inFlight.add(key);
    try {
      return await runner();
    } finally {
      this.inFlight.delete(key);
    }
  }

  private setActionButtonsDisabled(disabled: boolean): void {
    this.refs.btnRefreshStatus.disabled = disabled;
    this.refs.btnSyncNow.disabled = disabled;
    this.refs.btnPauseSync.disabled = disabled;
    this.refs.btnResumeSync.disabled = disabled;
  }

  private async runStatusAction(action: "sync-now" | "pause-sync" | "resume-sync"): Promise<void> {
    await this.withInFlight(`status-action:${action}`, async () => {
      this.setActionButtonsDisabled(true);
      try {
        await this.invoke(action);
        await this.refreshStatus({ quietError: true, silent: true });
        await this.refreshLogs(false, { quietError: true, silent: true });
      } finally {
        this.setActionButtonsDisabled(false);
      }
    });
  }

  private async refreshStatus(options: RefreshOptions = {}): Promise<void> {
    await this.withInFlight("refresh:status", async () => {
      if (!options.silent) {
        this.refs.btnRefreshStatus.disabled = true;
      }
      try {
        const result = await this.invoke("status", "", { quietError: options.quietError });
        if (!result.ok || !result.success) {
          return;
        }
        const payload = asRecord(result.data);
        if (!payload) {
          this.setBanner("error", "status: invalid payload format.");
          return;
        }
        this.state.status = payload as AppStatus;
        this.renderStatus();
      } finally {
        if (!options.silent) {
          this.refs.btnRefreshStatus.disabled = false;
        }
      }
    });
  }

  private async refreshConfig(options: RefreshOptions = {}): Promise<void> {
    await this.withInFlight("refresh:config", async () => {
      if (!options.silent) {
        this.refs.btnGetConfig.disabled = true;
      }
      try {
        const result = await this.invoke("get-config", "", { quietError: options.quietError });
        if (!result.ok || !result.success) {
          return;
        }
        const payload = asRecord(result.data);
        if (!payload) {
          this.setBanner("error", "get-config: invalid payload format.");
          return;
        }
        this.state.config = payload as ConfigSnapshot;
        this.renderConfig();
      } finally {
        if (!options.silent) {
          this.refs.btnGetConfig.disabled = false;
        }
      }
    });
  }

  private async scanPresets(): Promise<void> {
    await this.withInFlight("scan-presets", async () => {
      this.refs.btnScanPresets.disabled = true;
      this.refs.btnScanPresets.textContent = "Scanning";
      try {
        const presets = await discoverPresets();
        if (presets && presets.length > 0) {
          this.state.presetCategoryOptions = this.normalizeCategories([
            ...this.state.presetCategoryOptions,
            ...presets
          ]);
          this.renderPresetChecklist();
          this.setBanner("info", `Discovered ${presets.length} preset folders. Tick the ones you want, then click Save.`);
        } else {
          this.setBanner("error", "No presets discovered.");
        }
      } catch (err) {
        this.setBanner("error", "Failed to discover presets.");
      } finally {
        this.refs.btnScanPresets.disabled = false;
        this.refs.btnScanPresets.textContent = "Scan";
      }
    });
  }

  private async saveConfig(): Promise<void> {
    const heartbeat = toInt(this.refs.inputHeartbeat.value, 30);
    const checkInterval = toInt(this.refs.inputCheckInterval.value, 60);
    const lockTimeout = toInt(this.refs.inputLockTimeout.value, 120);
    const maxBackups = toInt(this.refs.inputMaxBackups.value, 5);
    const categories = this.parseCategoriesInput(this.refs.inputPresetCategories.value);

    if (lockTimeout < heartbeat) {
      this.setBanner("error", "Lock Timeout must be greater than or equal to Heartbeat Interval.");
      return;
    }
    if (this.refs.chkPresetSyncEnabled.checked && categories.length === 0) {
      this.setBanner("error", "Preset Sync is enabled; at least one category is required.");
      return;
    }

    const payload = {
      backup_folder: this.refs.inputBackupFolder.value.trim(),
      catalog_path: this.refs.inputCatalogPath.value.trim(),
      start_with_windows: this.refs.chkStartWithWindows.checked,
      start_minimized: this.refs.chkStartMinimized.checked,
      minimize_to_tray: this.refs.chkMinimizeToTray.checked,
      auto_sync: this.refs.chkAutoSync.checked,
      heartbeat_interval: heartbeat,
      check_interval: checkInterval,
      lock_timeout: lockTimeout,
      max_catalog_backups: maxBackups,
      preset_sync_enabled: this.refs.chkPresetSyncEnabled.checked,
      preset_categories: categories
    };

    await this.withInFlight("mutate:save-config", async () => {
      this.refs.btnSaveConfig.disabled = true;
      try {
        const result = await this.invoke("save-config", JSON.stringify(payload));
        if (result.ok && result.success) {
          // Sync minimize_to_tray to WailsApp backend
          syncMinimizeToTray(this.refs.chkMinimizeToTray.checked);
          this.setBanner("success", "Configuration saved.");
          await this.refreshStatus({ quietError: true, silent: true });
          await this.refreshConfig({ quietError: true, silent: true });
        }
      } finally {
        this.refs.btnSaveConfig.disabled = false;
      }
    });
  }

  private async refreshBackups(options: RefreshOptions = {}): Promise<void> {
    await this.withInFlight("refresh:backups", async () => {
      if (!options.silent) {
        this.refs.btnRefreshBackups.disabled = true;
      }
      try {
        const result = await this.invoke("get-backups", "", { quietError: options.quietError });
        if (!result.ok || !result.success) {
          return;
        }
        if (!Array.isArray(result.data)) {
          this.setBanner("error", "get-backups: invalid payload format.");
          return;
        }
        this.state.backups = result.data as BackupInfo[];
        this.renderBackups();
      } finally {
        if (!options.silent) {
          this.refs.btnRefreshBackups.disabled = false;
        }
      }
    });
  }

  private async syncSelectedBackup(): Promise<void> {
    const selected = this.refs.backupsSelect.value.trim();
    if (selected === "") {
      this.setBanner("error", "Please choose a backup to sync.");
      return;
    }

    await this.withInFlight("mutate:sync-backup", async () => {
      this.refs.btnSyncSelected.disabled = true;
      try {
        const result = await this.invoke("sync-backup", selected);
        if (result.ok && result.success) {
          this.setBanner("success", "Sync command sent.");
          await this.refreshStatus({ quietError: true, silent: true });
          await this.refreshLogs(false, { quietError: true, silent: true });
        }
      } finally {
        this.refs.btnSyncSelected.disabled = false;
      }
    });
  }

  private async refreshLogs(resetCursor: boolean, options: RefreshOptions = {}): Promise<void> {
    await this.withInFlight("refresh:logs", async () => {
      if (!options.silent) {
        this.refs.btnRefreshLogs.disabled = true;
      }
      try {
        if (resetCursor) {
          this.state.logs = [];
          this.state.logsLastId = 0;
        }

        const payload = JSON.stringify({
          after_id: this.state.logsLastId,
          limit: 120,
          level: this.state.logLevel
        });

        const result = await this.invoke("subscribe-logs", payload, { quietError: options.quietError });
        if (!result.ok || !result.success) {
          return;
        }

        const data = asRecord(result.data) as SubscribeLogsResult | null;
        if (!data || !Array.isArray(data.entries)) {
          this.setBanner("error", "subscribe-logs: invalid payload format.");
          return;
        }

        if (resetCursor) {
          this.state.logs = data.entries;
        } else {
          this.state.logs = this.state.logs.concat(data.entries);
        }
        if (this.state.logs.length > 500) {
          this.state.logs = this.state.logs.slice(this.state.logs.length - 500);
        }
        this.state.logsLastId = asNumber(data.last_id, this.state.logsLastId);
        this.renderLogs();
      } finally {
        if (!options.silent) {
          this.refs.btnRefreshLogs.disabled = false;
        }
      }
    });
  }

  private async refreshUpdate(options: RefreshOptions = {}): Promise<void> {
    await this.withInFlight("refresh:update", async () => {
      if (!options.silent) {
        this.refs.btnCheckUpdate.disabled = true;
        this.refs.btnForceDownloadLatest.disabled = true;
      }
      try {
        const result = await this.invoke("check-update", "", { quietError: options.quietError });
        if (!result.ok || !result.success) {
          return;
        }
        const payload = asRecord(result.data);
        if (!payload) {
          this.setBanner("error", "check-update: invalid payload format.");
          return;
        }
        this.state.update = payload as CheckUpdateResult;
        this.renderUpdate();
      } finally {
        if (!options.silent) {
          this.refs.btnCheckUpdate.disabled = false;
          this.renderUpdate();
        }
      }
    });
  }

  private async downloadUpdate(): Promise<void> {
    const assetUrl = asString(this.state.update.asset_url);
    if (!assetUrl) {
      this.setBanner("error", "No update asset URL available. Please run Check Update first.");
      return;
    }

    const payload = JSON.stringify({
      asset_url: assetUrl,
      asset_name: asString(this.state.update.asset_name)
    });

    await this.withInFlight("mutate:download-update", async () => {
      this.refs.btnDownloadUpdate.disabled = true;
      try {
        const result = await this.invoke("download-update", payload);
        if (result.ok && result.success) {
          const data = asRecord(result.data);
          const destination = asString(data?.destination_path);
          this.setBanner("success", destination ? `Download started: ${destination}` : "Download started.");
          await this.refreshUpdate({ quietError: true, silent: true });
        }
      } finally {
        this.refs.btnDownloadUpdate.disabled = false;
      }
    });
  }

  private async forceDownloadLatest(): Promise<void> {
    await this.withInFlight("mutate:force-download-latest", async () => {
      this.refs.btnForceDownloadLatest.disabled = true;
      try {
        // Always refresh latest release first to force using current latest asset URL.
        const check = await this.invoke("check-update", "", { quietError: true });
        if (!check.ok || !check.success) {
          this.setBanner("error", "Cannot resolve latest release. Check Update failed.");
          return;
        }

        const release = asRecord(check.data);
        if (!release) {
          this.setBanner("error", "Invalid update metadata received.");
          return;
        }
        this.state.update = release as CheckUpdateResult;
        this.renderUpdate();

        const assetUrl = asString(release.asset_url);
        const assetName = asString(release.asset_name);
        if (!assetUrl) {
          this.setBanner("error", "Latest release has no downloadable asset.");
          return;
        }

        const payload = JSON.stringify({
          asset_url: assetUrl,
          asset_name: assetName
        });
        const result = await this.invoke("download-update", payload);
        if (result.ok && result.success) {
          const data = asRecord(result.data);
          const destination = asString(data?.destination_path);
          this.setBanner("success", destination ? `Force download started: ${destination}` : "Force download started.");
          await this.refreshUpdate({ quietError: true, silent: true });
        }
      } finally {
        this.renderUpdate();
      }
    });
  }

  private async bootstrap(): Promise<void> {
    await this.refreshStatus({ quietError: true, silent: true });
    await this.refreshConfig({ quietError: true, silent: true });
    await this.refreshBackups({ quietError: true, silent: true });
    await this.refreshLogs(true, { quietError: true, silent: true });
    await this.refreshUpdate({ quietError: true, silent: true });

    if (this.shouldShowDisconnectedState()) {
      this.showAgentOverlay();
    }
    this.startPolling();
  }

  // --- Agent overlay logic ---

  private showAgentOverlay(): void {
    if (this.state.overlayDismissed) return;
    this.refs.agentOverlay.classList.remove("hidden");
    this.refs.agentOverlayStatus.textContent = "";
  }

  private hideAgentOverlay(): void {
    this.refs.agentOverlay.classList.add("hidden");
  }

  private async handleLaunchAgent(): Promise<void> {
    this.refs.btnLaunchAgent.disabled = true;
    this.refs.agentOverlayStatus.textContent = "Starting agent...";
    try {
      const result = await launchAgent();
      if (result.ok === "true") {
        this.refs.agentOverlayStatus.textContent = "Agent started! Connecting...";
        this.hideAgentOverlay();
        this.setBanner("success", "Agent launched successfully.");
        // Reload all data
        await this.refreshStatus({ quietError: true, silent: true });
        await this.refreshConfig({ quietError: true, silent: true });
        await this.refreshBackups({ quietError: true, silent: true });
        await this.refreshLogs(true, { quietError: true, silent: true });
      } else {
        this.refs.agentOverlayStatus.textContent = `Failed: ${result.error || "Unknown error"}`;
      }
    } catch (err) {
      this.refs.agentOverlayStatus.textContent = `Error: ${err}`;
    } finally {
      this.refs.btnLaunchAgent.disabled = false;
    }
  }
}

export function mountApp(root: HTMLElement): void {
  // Wave 3 baseline shell: fully rendered tabs with offline-safe behavior.
  new FrontendShell(root);
}
