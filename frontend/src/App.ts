import { executeAction } from "./bridge";
import type {
  ActionEnvelope,
  AppStatus,
  BackupInfo,
  CheckUpdateResult,
  ConfigSnapshot,
  StreamLogEntry,
  SubscribeLogsResult
} from "./types";

type TabKey = "status" | "settings" | "backups" | "logs" | "update";
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
  btnGetConfig: HTMLButtonElement;
  btnSaveConfig: HTMLButtonElement;
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
  btnDownloadUpdate: HTMLButtonElement;
  btnThemeToggle: HTMLButtonElement;
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
      update: {}
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
    return `
      <main class="app-shell">
        <header class="topbar">
          <div>
            <h1>Lightroom Sync</h1>
            <p class="subtitle">Wave 3 Frontend Shell (Wails UI)</p>
          </div>
          <div class="runtime-meta">
            <div><strong>Version:</strong> ${this.version}</div>
            <div><strong>Pipe:</strong> ${this.pipeName}</div>
          </div>
        </header>

        <section class="statusline">
          <span id="connection-badge" class="badge disconnected">Disconnected</span>
          <span id="connection-detail" class="connection-detail">Bridge unavailable</span>
          <span class="spacer"></span>
          <span class="last-refresh">Last refresh: <strong id="last-refresh">-</strong></span>
        </section>
      <div class="app-layout">
        <!-- Sidebar -->
        <aside class="sidebar">
          <div class="brand">
            <div class="brand-icon">❖</div>
            <div class="brand-name">Lightroom Sync</div>
          </div>
          <nav class="tabs-nav">
            <button class="tab-button is-active" data-tab="status">
              <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>
              Status
            </button>
            <button class="tab-button" data-tab="settings">
              <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
              Settings
            </button>
            <button class="tab-button" data-tab="backups">
              <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
              Backups
            </button>
            <button class="tab-button" data-tab="logs">
              <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>
              Logs
            </button>
            <button class="tab-button" data-tab="update">
              <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10"/></svg>
              Update
            </button>
          </nav>
          <div class="sidebar-footer">
            <button id="btn-theme-toggle" class="theme-toggle">Try Light Mode</button>
          </div>
        </aside>

        <!-- Main Workspace -->
        <main class="main-area">
          <header class="topbar">
            <div class="topbar-left">
              <h1 id="view-title">Dashboard Status</h1>
              <span id="connection-badge" class="badge disconnected">Disconnected</span>
              <span id="connection-detail" class="connection-detail" style="display:none;"></span>
            </div>
            <div class="runtime-meta">
              <div>Version: <strong>${this.version}</strong></div>
              <div style="font-size: 0.75rem; color: var(--text-muted); margin-top: 4px;">Pipe: ${this.pipeName}</div>
            </div>
          </header>

          <div class="content-wrapper">
            <div id="banner" class="banner banner-hidden">
              <span id="banner-text"></span>
            </div>

            <!-- Panel: Status -->
            <div id="panel-status" class="tab-panel">
              <div class="card status-grid">
                <div class="status-card">
                  <div class="status-label">Overall Status</div>
                  <div class="status-value" id="status-text">-</div>
                </div>
                <div class="status-card">
                  <div class="status-label">Tray Status Color</div>
                  <div class="status-value" id="status-tray-color">-</div>
                </div>
                <div class="status-card">
                  <div class="status-label">Sync Progress</div>
                  <div class="status-value syncing" id="status-sync-progress">-</div>
                </div>
                <div class="status-card">
                  <div class="status-label">Paused State</div>
                  <div class="status-value" id="status-sync-paused">-</div>
                </div>
                <div class="status-card" style="grid-column: 1 / -1;">
                  <div class="status-label">Monitor Errors</div>
                  <div class="status-value" id="status-monitor-errors" style="color: var(--danger); font-size: 1rem;">-</div>
                </div>
              </div>

              <div class="actions">
                <button id="btn-sync-now" class="primary">Sync Now</button>
                <button id="btn-pause-sync">Pause Sync</button>
                <button id="btn-resume-sync">Resume Sync</button>
                <button id="btn-refresh-status" style="margin-left: auto;">Refresh Status</button>
                <span id="last-refresh" style="align-self: flex-end; font-size: 0.8rem; color: var(--text-muted); display:inline-block; margin-bottom: 12px; margin-left:12px;">-</span>
              </div>
            </div>

            <!-- Panel: Settings -->
            <div id="panel-settings" class="tab-panel is-hidden">
              <div class="card">
                <h2>Configuration Profile</h2>
                <div class="grid two-col">
                  <div class="full">
                    <label for="cfg-backup-folder">Backup Destination Directory</label>
                    <input id="cfg-backup-folder" type="text" placeholder="D:\\Backups" />
                  </div>
                  <div class="full">
                    <label for="cfg-catalog-path">Lightroom Catalog Source Path</label>
                    <input id="cfg-catalog-path" type="text" placeholder="E:\\Lightroom\\Master.lrcat" />
                  </div>
                  <div>
                    <label for="cfg-heartbeat">Heartbeat Send Interval (s)</label>
                    <input id="cfg-heartbeat" type="number" min="1" />
                  </div>
                  <div>
                    <label for="cfg-check-interval">File Watch Check Interval (s)</label>
                    <input id="cfg-check-interval" type="number" min="1" />
                  </div>
                  <div>
                    <label for="cfg-lock-timeout">Catalog Lock Timeout (s)</label>
                    <input id="cfg-lock-timeout" type="number" min="1" />
                  </div>
                  <div>
                    <label for="cfg-max-backups">Max Retained Backups limit</label>
                    <input id="cfg-max-backups" type="number" min="1" />
                  </div>
                  <div class="full">
                    <label for="cfg-preset-categories">Preset Subject Categories (comma-listed)</label>
                    <input id="cfg-preset-categories" type="text" placeholder="Wedding, Studio, Street" />
                  </div>
                  <div class="checks">
                    <label><input id="cfg-start-with-windows" type="checkbox" /> Launch cleanly on Windows Startup</label>
                    <label><input id="cfg-start-minimized" type="checkbox" /> Keep Window Hidden at Launch</label>
                    <label><input id="cfg-minimize-to-tray" type="checkbox" /> Route Close button to System Tray</label>
                    <label><input id="cfg-auto-sync" type="checkbox" /> Enable Automatic Background Syncing</label>
                    <label><input id="cfg-preset-sync-enabled" type="checkbox" /> Activate Secondary Preset Sync System</label>
                  </div>
                </div>
                <div class="actions" style="margin-top: 32px; justify-content: flex-end;">
                  <button id="btn-get-config">Re-fetch Profile</button>
                  <button id="btn-save-config" class="primary">Save Configuration</button>
                </div>
              </div>
            </div>

            <!-- Panel: Backups -->
            <div id="panel-backups" class="tab-panel is-hidden">
              <div class="card">
                <h2>Generated Backups Archive</h2>
                <div class="backup-list-container">
                  <select id="backups-list" size="9"></select>
                </div>
                <div class="helper" id="backups-helper" style="margin-top: 12px; margin-bottom: 24px;">No backups cataloged.</div>
                <div class="actions">
                  <button id="btn-refresh-backups">Query Backups Archive</button>
                  <button id="btn-sync-selected" class="primary">Initiate Restore for Selected</button>
                </div>
              </div>
            </div>

            <!-- Panel: Logs -->
            <div id="panel-logs" class="tab-panel is-hidden">
              <div class="card">
                <h2>Transmission Event Logs</h2>
                <div class="log-filters">
                  <label for="logs-level" style="margin: 0; align-self: center;">Verbosity</label>
                  <select id="logs-level" style="width: auto;">
                    <option>ALL</option>
                    <option>INFO</option>
                    <option>WARN</option>
                    <option>ERROR</option>
                    <option>DEBUG</option>
                  </select>
                  <button id="btn-refresh-logs">Reload Output</button>
                  <button id="btn-clear-logs">Purge Viewport</button>
                </div>
                <pre id="logs-output" class="logs-console">(idle output buffer)</pre>
              </div>
            </div>

            <!-- Panel: Update -->
            <div id="panel-update" class="tab-panel is-hidden">
               <div class="card">
                <h2>Software Releases Lifecycle</h2>
                <div class="grid two-col" style="margin-bottom: 24px;">
                  <div class="status-card">
                    <div class="status-label">Running Build</div>
                    <div class="status-value" id="upd-current-version">-</div>
                  </div>
                  <div class="status-card">
                    <div class="status-label">Upstream Release</div>
                    <div class="status-value" id="upd-latest-version">-</div>
                  </div>
                  <div class="status-card" style="grid-column: 1 / -1; align-items:center;">
                    <div class="status-label">Upgrade Viability</div>
                    <div class="status-value" id="upd-has-update">-</div>
                  </div>
                </div>
                <label for="upd-release-notes">Changelog & Patch Notes</label>
                <textarea id="upd-release-notes" rows="8" readonly style="margin-bottom: 24px;"></textarea>
                <div class="actions" style="justify-content: flex-end;">
                  <button id="btn-check-update">Determine Upgrade Path</button>
                  <button id="btn-download-update" class="primary">Deploy Software Update</button>
                </div>
               </div>
            </div>

          </div>
        </main>
      </div>
    `;
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
      btnGetConfig: byId<HTMLButtonElement>("btn-get-config"),
      btnSaveConfig: byId<HTMLButtonElement>("btn-save-config"),
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
      btnDownloadUpdate: byId<HTMLButtonElement>("btn-download-update"),
      btnThemeToggle: byId<HTMLButtonElement>("btn-theme-toggle")
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
    this.refs.btnDownloadUpdate.addEventListener("click", () => void this.downloadUpdate());

    this.refs.btnThemeToggle.addEventListener("click", () => {
      const docEl = document.documentElement;
      const isLight = docEl.getAttribute("data-theme") === "light";
      if (isLight) {
        docEl.removeAttribute("data-theme");
        this.refs.btnThemeToggle.textContent = "Try Light Mode";
      } else {
        docEl.setAttribute("data-theme", "light");
        this.refs.btnThemeToggle.textContent = "Try Dark Mode";
      }
    });

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

  private renderConnection(): void {
    this.refs.connectionBadge.textContent = this.state.connected ? "Connected" : "Disconnected";
    this.refs.connectionBadge.className = this.state.connected ? "badge connected" : "badge disconnected";
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
    this.refs.inputPresetCategories.value = (cfg.preset_categories ?? []).join(", ");
  }

  private renderBackups(): void {
    this.refs.backupsSelect.innerHTML = "";
    if (this.state.backups.length === 0) {
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
      this.setConnection(false, envelope.error ?? "Agent unavailable");
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

  private async saveConfig(): Promise<void> {
    const heartbeat = toInt(this.refs.inputHeartbeat.value, 30);
    const checkInterval = toInt(this.refs.inputCheckInterval.value, 60);
    const lockTimeout = toInt(this.refs.inputLockTimeout.value, 120);
    const maxBackups = toInt(this.refs.inputMaxBackups.value, 5);
    const categories = this.refs.inputPresetCategories.value
      .split(",")
      .map((item) => item.trim())
      .filter((item) => item.length > 0);

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
          this.setBanner("success", "Download started.");
          await this.refreshUpdate({ quietError: true, silent: true });
        }
      } finally {
        this.refs.btnDownloadUpdate.disabled = false;
      }
    });
  }

  private async bootstrap(): Promise<void> {
    await this.refreshStatus({ quietError: true, silent: true });
    await this.refreshConfig({ quietError: true, silent: true });
    await this.refreshBackups({ quietError: true, silent: true });
    await this.refreshLogs(true, { quietError: true, silent: true });
    await this.refreshUpdate({ quietError: true, silent: true });

    if (!this.state.connected) {
      this.setBanner("info", "Agent is offline. You can keep using tabs; data refresh will recover automatically.");
    }
    this.startPolling();
  }
}

export function mountApp(root: HTMLElement): void {
  // Wave 3 baseline shell: fully rendered tabs with offline-safe behavior.
  new FrontendShell(root);
}
