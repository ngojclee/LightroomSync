export const appTemplate = `
<!-- SideNavBar -->
<aside class="h-screen w-64 fixed left-0 top-0 bg-[#0e0e0f]/80 backdrop-blur-xl flex flex-col py-8 shadow-[40px_0_40px_-20px_rgba(123,44,191,0.1)] z-50 transition-colors">
  <div class="px-6 mb-10">
    <h1 class="text-xl font-bold bg-gradient-to-br from-[#7B2CBF] to-[#deb7ff] bg-clip-text text-transparent font-headline tracking-tight">LightroomSync</h1>
    <p class="text-xs text-on-surface-variant/50 font-label mt-1">Creative Suite</p>
  </div>
  <nav class="flex-1 space-y-1 tabs-nav">
    <button class="tab-button w-full flex items-center space-x-3 py-3 px-4 bg-[#2a2a2b] text-[#00F5FF] border-l-2 border-[#00F5FF] shadow-[0_0_15px_rgba(0,245,255,0.2)] font-headline transition-transform active:scale-[0.98] is-active" data-tab="status">
      <span class="material-symbols-outlined" data-icon="analytics">analytics</span>
      <span class="text-sm font-semibold">Status</span>
    </button>
    <button id="nav-onboarding" class="tab-button w-full flex items-center space-x-3 py-3 px-4 text-[#e5e2e3]/60 hover:text-[#e5e2e3] hover:bg-[#201f20] transition-all duration-300 font-headline active:scale-[0.98] hidden" data-tab="onboarding">
      <span class="material-symbols-outlined text-[#7B2CBF]" data-icon="explore">explore</span>
      <span class="text-sm font-bold text-[#deb7ff]">Onboarding</span>
    </button>
    <button class="tab-button w-full flex items-center space-x-3 py-3 px-4 text-[#e5e2e3]/60 hover:text-[#e5e2e3] hover:bg-[#201f20] transition-all duration-300 font-headline active:scale-[0.98]" data-tab="settings">
      <span class="material-symbols-outlined" data-icon="settings">settings</span>
      <span class="text-sm">Settings</span>
    </button>
    <button class="tab-button w-full flex items-center space-x-3 py-3 px-4 text-[#e5e2e3]/60 hover:text-[#e5e2e3] hover:bg-[#201f20] transition-all duration-300 font-headline active:scale-[0.98]" data-tab="backups">
      <span class="material-symbols-outlined" data-icon="backup">backup</span>
      <span class="text-sm">Backups</span>
    </button>
    <button class="tab-button w-full flex items-center space-x-3 py-3 px-4 text-[#e5e2e3]/60 hover:text-[#e5e2e3] hover:bg-[#201f20] transition-all duration-300 font-headline active:scale-[0.98]" data-tab="logs">
      <span class="material-symbols-outlined" data-icon="history">history</span>
      <span class="text-sm">Logs</span>
    </button>
    <button class="tab-button w-full flex items-center space-x-3 py-3 px-4 text-[#e5e2e3]/60 hover:text-[#e5e2e3] hover:bg-[#201f20] transition-all duration-300 font-headline active:scale-[0.98]" data-tab="update">
      <span class="material-symbols-outlined" data-icon="update">update</span>
      <span class="text-sm">Update</span>
    </button>
    <button class="tab-button w-full flex items-center space-x-3 py-3 px-4 text-[#e5e2e3]/60 hover:text-[#e5e2e3] hover:bg-[#201f20] transition-all duration-300 font-headline active:scale-[0.98]" data-tab="about">
      <span class="material-symbols-outlined" data-icon="info">info</span>
      <span class="text-sm">About</span>
    </button>
  </nav>
  <div class="px-6 mt-auto space-y-2">
    <p class="text-[10px] text-on-surface-variant/40 text-center mb-2 font-mono" id="sidebar-version">v2.0.6.202604071807</p>
    <button id="btn-exit-app" class="w-full flex items-center space-x-2 justify-center text-error/70 hover:text-error font-semibold py-2 px-4 rounded-xl text-xs hover:bg-error/10 transition-colors" title="Stop Agent + close UI">
      <span class="material-symbols-outlined text-sm">power_settings_new</span>
      <span>Exit All</span>
    </button>
  </div>
</aside>

<!-- TopNavBar -->
<header class="h-16 fixed top-0 right-0 left-64 z-40 flex items-center justify-between px-8 bg-[#131314]/60 backdrop-blur-md no-border bg-gradient-to-b from-[#131314] to-transparent">
  <div class="flex items-center space-x-4 w-1/3">
    <span class="text-on-surface/40 text-xs font-label">System / Dashboard / <span class="text-on-surface" id="view-title">Status</span></span>
  </div>
  <div class="flex items-center justify-center space-x-6 w-1/3">
    <div class="flex items-center gap-4 text-xs">
        <span id="connection-badge" class="badge connecting">Connecting...</span>
        <span id="connection-detail" class="text-on-surface-variant max-w-xs truncate hidden"></span>
        <span class="text-on-surface-variant">Last: <strong id="last-refresh">-</strong></span>
    </div>
  </div>
  <div class="w-1/3 flex justify-end"></div>
</header>

<div id="banner" class="fixed top-20 right-8 z-50 p-4 rounded-lg shadow-lg flex items-center gap-3 transition-transform banner-hidden hidden max-w-md">
  <span id="banner-text" class="text-sm font-semibold text-center"></span>
</div>

<!-- Agent Disconnected Overlay -->
<div id="agent-overlay" class="fixed inset-0 z-[100] bg-[#0e0e0f]/90 backdrop-blur-md flex items-center justify-center hidden">
  <div class="text-center max-w-sm">
    <div class="w-16 h-16 mx-auto mb-6 rounded-full bg-error/20 flex items-center justify-center">
      <span class="material-symbols-outlined text-error text-3xl">cloud_off</span>
    </div>
    <h2 class="text-xl font-headline font-extrabold text-on-surface mb-2">Agent Unreachable</h2>
    <p class="text-sm text-on-surface-variant mb-6">The background sync agent is not running. Would you like to start it?</p>
    <div class="flex gap-3 justify-center">
      <button id="btn-launch-agent" class="flex items-center space-x-2 bg-gradient-to-br from-primary-container to-primary text-on-primary font-bold py-2.5 px-6 rounded-lg text-xs shadow-[0_0_12px_rgba(123,44,191,0.4)] hover:scale-[1.02] transition-transform">
        <span class="material-symbols-outlined text-sm">play_arrow</span>
        <span>Launch Agent</span>
      </button>
      <button id="btn-dismiss-overlay" class="flex items-center space-x-2 bg-surface-container-high border border-outline-variant/20 text-on-surface font-semibold py-2.5 px-6 rounded-lg text-xs hover:bg-surface-container-highest transition-colors">
        <span>Continue Offline</span>
      </button>
    </div>
    <p id="agent-overlay-status" class="text-xs text-on-surface-variant/50 mt-4 h-4"></p>
  </div>
</div>

<!-- Main Content Canvas -->
<main class="ml-64 pt-24 pb-8 px-12 h-screen overflow-y-auto relative box-border">

  <!-- Status Panel -->
  <div id="panel-status" class="tab-panel">
    <!-- Hero Section: Connection Status -->
    <section class="mb-12 flex flex-col md:flex-row md:items-end justify-between gap-6">
      <div>
        <div class="inline-flex items-center space-x-2 px-3 py-1 rounded-full bg-secondary-container/10 border border-secondary-container/20 mb-3 neon-glow-secondary">
          <span class="relative flex h-2 w-2">
            <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-secondary-fixed opacity-75"></span>
            <span class="relative inline-flex rounded-full h-2 w-2 bg-secondary-fixed"></span>
          </span>
          <span class="text-[10px] font-bold text-secondary-fixed tracking-widest uppercase" id="hero-connection-status">Agent Active</span>
        </div>
        <h2 class="text-2xl font-extrabold font-headline tracking-tight text-on-surface mb-1">Workspace Health</h2>
        <p class="text-on-surface-variant text-sm max-w-md">Your Lightroom catalogs are being synchronized.</p>
      </div>
      <!-- Quick Actions -->
      <div class="flex items-center space-x-2">
        <button id="btn-sync-now" class="flex items-center space-x-1.5 bg-gradient-to-br from-primary-container to-primary text-on-primary font-bold py-2 px-4 rounded-lg text-xs shadow-[0_0_12px_rgba(123,44,191,0.4)] hover:scale-[1.02] transition-transform disabled:opacity-50">
          <span class="material-symbols-outlined text-sm">sync</span>
          <span>Sync Now</span>
        </button>
        <button id="btn-pause-sync" class="flex items-center space-x-1.5 bg-surface-container-high border border-outline-variant/20 text-on-surface font-semibold py-2 px-4 rounded-lg text-xs hover:bg-surface-container-highest transition-colors disabled:opacity-50">
          <span class="material-symbols-outlined text-sm">pause</span>
          <span>Pause</span>
        </button>
        <button id="btn-resume-sync" class="flex items-center space-x-1.5 bg-surface-container-high border border-outline-variant/20 text-on-surface font-semibold py-2 px-4 rounded-lg text-xs hover:bg-surface-container-highest transition-colors disabled:opacity-50">
          <span class="material-symbols-outlined text-sm">play_arrow</span>
          <span>Resume</span>
        </button>
      </div>
    </section>

    <!-- Bento Grid Layout -->
    <div class="grid grid-cols-12 gap-5">
      <!-- Main Sync Progress Card -->
      <div class="col-span-12 glass-panel rounded-2xl p-6 border border-outline-variant/10 relative overflow-hidden bg-surface-container-low/80 backdrop-blur-xl">
        <div class="absolute top-0 right-0 w-64 h-64 bg-primary/5 blur-[100px] rounded-full -mr-20 -mt-20"></div>
        
        <div class="flex items-center justify-between mb-6 relative z-10">
          <div class="flex items-center gap-3">
            <h3 class="text-lg font-bold font-headline text-on-surface">Overall Status</h3>
            <p class="text-sm font-semibold text-on-surface-variant bg-surface-container px-3 py-1 rounded-md" id="status-text">-</p>
          </div>
          <button id="btn-refresh-status" class="flex items-center space-x-1 text-on-surface-variant hover:text-on-surface text-xs border border-outline-variant/20 rounded-lg px-3 py-1.5 hover:bg-surface-container-high transition-colors disabled:opacity-50">
            <span class="material-symbols-outlined text-sm">refresh</span>
            <span>Refresh</span>
          </button>
        </div>

        <div class="relative z-10 mt-4 border-t border-outline-variant/10 pt-4">
          <span id="status-sync-paused" class="hidden">-</span>
          <span id="status-sync-progress" class="hidden">-</span>
          <span id="status-tray-color" class="hidden">-</span>
          <div>
            <p class="text-[10px] text-error font-label uppercase tracking-widest mb-1">Monitor Errors</p>
            <p class="text-base font-bold text-error truncate" id="status-monitor-errors">-</p>
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- Onboarding Panel -->
  <div id="panel-onboarding" class="tab-panel is-hidden hidden">
      <header class="mb-8">
        <h2 class="text-3xl font-extrabold font-headline tracking-tight text-primary mb-2">Welcome to LightroomSync</h2>
        <p class="text-on-surface-variant text-sm max-w-2xl text-balance">
          It looks like this is the first time you are connecting this machine. To prevent data loss or catalog conflicts, please choose how you want to initialize the synchronization.
        </p>
      </header>
      
      <div class="glass-panel p-6 rounded-2xl mb-8 space-y-4 shadow-md bg-surface-container-low/50">
        <h3 class="text-xl font-bold text-on-surface">Step 1: Merge catalogs cleanly</h3>
        <p class="text-on-surface-variant text-sm">
          If you have photos on this machine that are NOT on your main network catalog yet, you must manually merge them first:
        </p>
        <ul class="list-disc pl-5 text-sm text-on-surface-variant space-y-1">
          <li>Select the new folders/collections in Lightroom.</li>
          <li>Click File -> <strong class="text-on-surface">Export as Catalog</strong> and save it to a safe backup folder.</li>
          <li>After choosing to "Pull from Network", open that network catalog.</li>
          <li>Click File -> <strong class="text-on-surface">Import from Another Catalog</strong> and select the exported catalog.</li>
        </ul>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div class="glass-panel p-8 rounded-2xl flex flex-col items-start border-l-4 border-primary bg-surface-container-lowest/80 transition-transform hover:-translate-y-1 hover:shadow-xl">
          <div class="p-3 bg-primary/10 rounded-full mb-4">
            <span class="material-symbols-outlined text-primary text-3xl">download</span>
          </div>
          <h3 class="text-xl font-bold font-headline mb-2 text-on-surface">Pull from Network</h3>
          <p class="text-sm text-on-surface-variant mb-6 flex-1">
            Overwrite your local catalog with the latest version from the network. Ideal if this machine is new or outdated.
          </p>
          <button id="btn-onboard-pull" class="w-full bg-gradient-to-br from-primary-container to-primary text-on-primary font-bold py-3 px-4 rounded-xl shadow-[0_0_15px_rgba(123,44,191,0.3)] hover:scale-[1.02] transition-transform flex items-center justify-center gap-2">
            <span class="material-symbols-outlined text-sm">download</span> Pull & Replace Local
          </button>
        </div>

        <div class="glass-panel p-8 rounded-2xl flex flex-col items-start border-l-4 border-secondary-fixed bg-surface-container-lowest/80 transition-transform hover:-translate-y-1 hover:shadow-xl">
          <div class="p-3 bg-secondary-fixed/10 rounded-full mb-4">
            <span class="material-symbols-outlined text-secondary-fixed text-3xl">upload</span>
          </div>
          <h3 class="text-xl font-bold font-headline mb-2 text-on-surface">Push Local as Master</h3>
          <p class="text-sm text-on-surface-variant mb-6 flex-1">
            Force your current local catalog to overwrite the network backup. Only do this if you are absolutely sure this machine has the best version.
          </p>
          <button id="btn-onboard-push" class="w-full bg-surface-container-high border border-secondary-fixed/50 hover:bg-secondary-fixed/20 hover:border-secondary-fixed text-secondary-fixed font-bold py-3 px-4 rounded-xl transition-colors flex items-center justify-center gap-2">
            <span class="material-symbols-outlined text-sm">upload</span> Push & Overwrite Network
          </button>
        </div>
      </div>
  </div>

  <!-- Settings Panel -->
  <div id="panel-settings" class="tab-panel is-hidden hidden">
      <header class="mb-4 flex items-center justify-between">
        <div>
          <h1 class="font-headline text-xl font-extrabold tracking-tight text-on-surface">System Settings</h1>
          <p class="text-on-surface/50 text-xs">Configure synchronization, paths, and behavior.</p>
        </div>
        <div class="flex gap-3">
            <button id="btn-get-config" class="px-4 py-1.5 rounded-lg text-xs font-semibold text-on-surface hover:bg-surface-container transition-colors disabled:opacity-50 border border-outline-variant/30">
                Reload
            </button>
            <button id="btn-save-config" class="px-5 py-1.5 bg-gradient-to-br from-primary-container to-primary text-on-primary font-bold rounded-lg text-xs shadow-[0_0_12px_rgba(123,44,191,0.3)] hover:scale-[1.02] transition-all disabled:opacity-50">
                Save
            </button>
        </div>
      </header>

      <div class="grid grid-cols-12 gap-4">
        <!-- Paths -->
        <div class="col-span-12 lg:col-span-7 glass-card p-5 rounded-xl bg-surface-container-low/80">
            <h3 class="font-headline text-sm font-bold mb-3 flex items-center gap-2">
              <span class="material-symbols-outlined text-primary text-base">folder_open</span>
              Paths
            </h3>
            <div class="space-y-3">
              <div>
                <label class="block text-[10px] font-bold text-on-surface-variant uppercase tracking-widest mb-1">Catalog Source</label>
                <div class="flex gap-1.5">
                  <input id="cfg-catalog-path" class="flex-1 bg-surface-container-lowest border border-outline-variant/30 rounded-lg py-2 px-3 text-xs text-on-surface font-mono" type="text" />
                  <button id="btn-browse-catalog" class="px-3 bg-surface-container-highest hover:bg-surface-bright rounded-lg text-xs font-semibold border border-outline-variant/30">Browse</button>
                </div>
              </div>
              <div>
                <label class="block text-[10px] font-bold text-on-surface-variant uppercase tracking-widest mb-1">Backup Destination</label>
                <div class="flex gap-1.5">
                  <input id="cfg-backup-folder" class="flex-1 bg-surface-container-lowest border border-outline-variant/30 rounded-lg py-2 px-3 text-xs text-on-surface font-mono" type="text" />
                  <button id="btn-browse-backup" class="px-3 bg-surface-container-highest hover:bg-surface-bright rounded-lg text-xs font-semibold border border-outline-variant/30">Browse</button>
                </div>
              </div>
            </div>
        </div>

        <!-- Behavior -->
        <div class="col-span-12 lg:col-span-5 glass-card p-5 rounded-xl bg-surface-container-low/80">
            <h3 class="font-headline text-sm font-bold mb-3">Behavior</h3>
            <div class="space-y-2.5">
              <label class="flex items-center gap-2.5 cursor-pointer">
                <input type="checkbox" id="cfg-start-with-windows" class="rounded border-outline-variant/30 text-primary focus:ring-primary/50 bg-surface-container-lowest h-4 w-4" />
                <span class="text-xs text-on-surface">Start with Windows</span>
              </label>
              <label class="flex items-center gap-2.5 cursor-pointer">
                <input type="checkbox" id="cfg-start-minimized" class="rounded border-outline-variant/30 text-primary focus:ring-primary/50 bg-surface-container-lowest h-4 w-4" />
                <span class="text-xs text-on-surface">Start minimized</span>
              </label>
              <label class="flex items-center gap-2.5 cursor-pointer">
                <input type="checkbox" id="cfg-minimize-to-tray" class="rounded border-outline-variant/30 text-primary focus:ring-primary/50 bg-surface-container-lowest h-4 w-4" />
                <span class="text-xs text-on-surface">Minimize to tray</span>
              </label>
              <label class="flex items-center gap-2.5 cursor-pointer">
                <input type="checkbox" id="cfg-auto-sync" class="rounded border-outline-variant/30 text-primary focus:ring-primary/50 bg-surface-container-lowest h-4 w-4" />
                <span class="text-xs text-on-surface">Auto background sync</span>
              </label>
              <label class="flex items-center gap-2.5 cursor-pointer">
                <input type="checkbox" id="cfg-preset-sync-enabled" class="rounded border-outline-variant/30 text-primary focus:ring-primary/50 bg-surface-container-lowest h-4 w-4" />
                <span class="text-xs text-on-surface">Preset sync enabled</span>
              </label>
            </div>
        </div>

        <!-- Timing & Limits -->
        <div class="col-span-12 glass-card p-5 rounded-xl bg-surface-container-low/80">
            <h3 class="font-headline text-sm font-bold mb-3">Timing & Limits</h3>
            <div class="grid grid-cols-2 md:grid-cols-5 gap-3">
                <div>
                  <label class="block text-[10px] font-bold text-on-surface-variant uppercase tracking-widest mb-1">Heartbeat (s)</label>
                  <input id="cfg-heartbeat" class="w-full bg-surface-container-lowest border border-outline-variant/30 rounded-lg py-2 px-3 text-xs font-mono" type="number" />
                </div>
                <div>
                  <label class="block text-[10px] font-bold text-on-surface-variant uppercase tracking-widest mb-1">Check (s)</label>
                  <input id="cfg-check-interval" class="w-full bg-surface-container-lowest border border-outline-variant/30 rounded-lg py-2 px-3 text-xs font-mono" type="number" />
                </div>
                <div>
                  <label class="block text-[10px] font-bold text-on-surface-variant uppercase tracking-widest mb-1">Lock Timeout (s)</label>
                  <input id="cfg-lock-timeout" class="w-full bg-surface-container-lowest border border-outline-variant/30 rounded-lg py-2 px-3 text-xs font-mono" type="number" />
                </div>
                <div>
                  <label class="block text-[10px] font-bold text-on-surface-variant uppercase tracking-widest mb-1">Max Backups</label>
                  <input id="cfg-max-backups" class="w-full bg-surface-container-lowest border border-outline-variant/30 rounded-lg py-2 px-3 text-xs font-mono" type="number" />
                </div>
                <div class="col-span-2 md:col-span-5">
                  <label class="block text-[10px] font-bold text-on-surface-variant uppercase tracking-widest mb-1">Preset Categories</label>
                  <div class="flex gap-1.5 mb-2">
                    <input id="cfg-preset-categories" class="flex-1 bg-surface-container-lowest border border-outline-variant/30 rounded-lg py-2 px-3 text-xs font-mono" type="text" placeholder="Develop Presets, Export Presets" />
                    <button id="btn-scan-presets" class="px-3 bg-surface-container-highest hover:bg-surface-bright rounded-lg text-xs font-semibold border border-outline-variant/30">Scan</button>
                  </div>
                  <div class="flex gap-2 mb-2">
                    <button id="btn-preset-select-all" class="px-3 py-1 bg-surface-container-high border border-outline-variant/20 rounded-md text-[11px] font-semibold text-on-surface-variant hover:text-on-surface transition-colors">Select All</button>
                    <button id="btn-preset-clear" class="px-3 py-1 bg-surface-container-high border border-outline-variant/20 rounded-md text-[11px] font-semibold text-on-surface-variant hover:text-on-surface transition-colors">Clear</button>
                  </div>
                  <div id="cfg-preset-checklist" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-1.5 max-h-44 overflow-y-auto rounded-lg border border-outline-variant/20 bg-surface-container-lowest p-2">
                    <p class="text-[11px] text-on-surface-variant/70">Click Scan to load preset folders.</p>
                  </div>
                </div>
            </div>
        </div>
      </div>
  </div>

  <!-- Backups Panel -->
  <div id="panel-backups" class="tab-panel is-hidden hidden">
      <header class="mb-6">
        <h1 class="font-headline text-xl font-extrabold tracking-tight text-on-surface mb-1">Backups Archive</h1>
        <p class="text-on-surface/50 text-xs">Select a backup to inspect or restore.</p>
      </header>
      <div class="glass-panel p-6 rounded-xl border border-outline-variant/10 bg-surface-container-low/80 min-h-[360px] h-[calc(100vh-250px)] flex flex-col">
        <select id="backups-list" size="12" class="w-full flex-1 min-h-[260px] bg-surface-container-lowest border border-outline-variant/30 rounded-lg p-3 text-xs font-mono text-on-surface focus:ring-2 focus:ring-primary/50 overflow-y-auto mb-3 appearance-none"></select>
        <div id="restore-progress-container" class="hidden mb-3">
          <div class="flex items-center justify-between text-xs text-on-surface-variant mb-1">
            <span id="restore-progress-label">Restoring...</span>
            <span id="restore-progress-pct">0%</span>
          </div>
          <div class="h-1.5 bg-surface-container-lowest rounded-full overflow-hidden">
            <div id="restore-progress-bar" class="h-full bg-gradient-to-r from-primary to-secondary transition-all duration-300 rounded-full" style="width: 0%"></div>
          </div>
        </div>
        <div id="backups-helper" class="text-xs text-on-surface-variant mb-4">No backups loaded.</div>
        <div class="flex justify-end gap-3">
          <button id="btn-refresh-backups" class="px-4 py-2 rounded-lg text-xs font-semibold text-on-surface hover:bg-surface-container transition-colors border border-outline-variant/30 disabled:opacity-50">Query Archive</button>
          <button id="btn-sync-selected" class="px-5 py-2 bg-secondary-container text-on-secondary font-bold rounded-lg text-xs hover:scale-[1.02] transition-all disabled:opacity-50 shadow-[0_0_12px_rgba(0,245,255,0.2)]">Restore Selected</button>
        </div>
      </div>
  </div>

  <!-- Logs Panel -->
  <div id="panel-logs" class="tab-panel is-hidden hidden">
      <header class="mb-4 flex justify-between items-end">
        <div>
            <h1 class="font-headline text-xl font-extrabold tracking-tight text-on-surface mb-1">Event Logs</h1>
            <p class="text-on-surface/50 text-xs">Real-time system console.</p>
        </div>
        <div class="flex gap-3 items-center">
            <label class="text-xs text-on-surface-variant">Level:</label>
            <select id="logs-level" class="bg-surface-container-lowest border border-outline-variant/30 rounded-lg py-1.5 px-2 text-xs text-on-surface font-bold">
                <option>ALL</option>
                <option>INFO</option>
                <option>WARN</option>
                <option>ERROR</option>
                <option>DEBUG</option>
            </select>
            <button id="btn-refresh-logs" class="px-3 py-1.5 bg-surface-container hover:bg-surface-container-high rounded text-xs transition-colors border border-outline-variant/20 disabled:opacity-50">Reload</button>
            <button id="btn-clear-logs" class="px-3 py-1.5 bg-error/10 text-error hover:bg-error/20 rounded text-xs font-bold transition-colors">Purge</button>
        </div>
      </header>
      <div class="glass-panel rounded-xl border border-outline-variant/10 bg-[#0e0e0f] overflow-hidden max-h-[calc(100vh-200px)] flex">
        <pre id="logs-output" class="w-full h-full p-4 overflow-y-auto text-[11px] font-mono text-on-surface/80 leading-relaxed">(idle output buffer)</pre>
      </div>
  </div>

  <!-- Update Panel -->
  <div id="panel-update" class="tab-panel is-hidden hidden">
      <header class="mb-6">
        <h1 class="font-headline text-xl font-extrabold tracking-tight text-on-surface mb-1">Software Update</h1>
      </header>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
          <div class="glass-panel rounded-xl p-5 bg-surface-container-low/80 border border-outline-variant/10">
              <div class="text-[10px] text-on-surface-variant uppercase tracking-widest mb-1 font-bold">Current</div>
              <div id="upd-current-version" class="text-xl font-headline font-extrabold text-on-surface">-</div>
          </div>
          <div class="glass-panel rounded-xl p-5 bg-surface-container-low/80 border border-outline-variant/10">
              <div class="text-[10px] text-on-surface-variant uppercase tracking-widest mb-1 font-bold">Latest</div>
              <div id="upd-latest-version" class="text-xl font-headline font-extrabold text-secondary-fixed">-</div>
          </div>
          <div class="glass-panel rounded-xl p-5 bg-surface-container-low/80 border border-outline-variant/10 flex flex-col justify-center">
              <div class="text-[10px] text-on-surface-variant uppercase tracking-widest mb-1 font-bold">Status</div>
              <div id="upd-has-update" class="text-lg font-bold text-primary">-</div>
          </div>
      </div>
      <div class="mb-6">
          <label class="block text-xs font-bold text-on-surface-variant uppercase tracking-widest mb-2">Release Notes</label>
          <textarea id="upd-release-notes" readonly class="w-full bg-[#0e0e0f] border border-outline-variant/20 rounded-xl p-4 text-xs font-mono text-on-surface/80 h-48 focus:ring-0 resize-none"></textarea>
      </div>
      <div class="flex justify-end gap-4">
          <button id="btn-check-update" class="px-5 py-2 rounded-lg text-xs font-semibold text-on-surface hover:bg-surface-container transition-colors border border-outline-variant/30 disabled:opacity-50">Check for Update</button>
          <button id="btn-force-download-latest" class="px-5 py-2 rounded-lg text-xs font-semibold text-on-surface hover:bg-surface-container transition-colors border border-outline-variant/30 disabled:opacity-50">Force Latest Download</button>
          <button id="btn-download-update" class="px-6 py-2 bg-gradient-to-br from-primary-container to-primary text-on-primary font-bold rounded-lg text-xs shadow-[0_0_12px_rgba(123,44,191,0.3)] hover:scale-[1.02] transition-all disabled:opacity-50">Download Update</button>
      </div>
  </div>

  <!-- About Panel -->
  <div id="panel-about" class="tab-panel is-hidden hidden">
      <div class="flex flex-col items-center justify-center py-12">
        <div class="w-20 h-20 rounded-2xl bg-gradient-to-br from-primary-container to-primary flex items-center justify-center mb-6 shadow-[0_0_30px_rgba(123,44,191,0.4)]">
          <span class="material-symbols-outlined text-on-primary text-4xl">sync</span>
        </div>
        <h1 class="font-headline text-2xl font-extrabold text-on-surface mb-1">Lightroom Sync</h1>
        <p class="text-on-surface-variant text-sm mb-6" id="about-version">Version 2.0.6.202604071807</p>

        <div class="glass-card p-6 rounded-xl bg-surface-container-low/80 max-w-md w-full text-center mb-6">
          <p class="text-xs text-on-surface-variant leading-relaxed">
            A professional catalog & preset synchronization tool for Adobe Lightroom Classic.
            Keeps your catalogs backed up and your presets synchronized across multiple workstations.
          </p>
        </div>

        <div class="grid grid-cols-2 gap-4 max-w-sm w-full mb-8">
          <div class="glass-panel rounded-xl p-4 border border-outline-variant/10 text-center">
            <div class="text-[10px] text-on-surface-variant uppercase tracking-widest mb-1 font-bold">Developer</div>
            <div class="text-sm font-bold text-on-surface">Le Ngoc</div>
          </div>
          <div class="glass-panel rounded-xl p-4 border border-outline-variant/10 text-center">
            <div class="text-[10px] text-on-surface-variant uppercase tracking-widest mb-1 font-bold">License</div>
            <div class="text-sm font-bold text-on-surface">Proprietary</div>
          </div>
        </div>

        <p class="text-[10px] text-on-surface-variant/40 font-mono" id="about-copyright">&copy; 2026 Le Ngoc. All rights reserved.</p>
      </div>
  </div>

  <!-- Decorative Background Elements -->
  <div class="absolute bottom-0 right-0 w-[500px] h-[500px] bg-primary-container/10 blur-[150px] -z-10 rounded-full opacity-30 pointer-events-none"></div>
  <div class="absolute top-0 left-0 w-[300px] h-[300px] bg-secondary-container/5 blur-[120px] -z-10 rounded-full opacity-20 pointer-events-none"></div>
</main>
`;

