package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ngojclee/lightroom-sync/internal/config"
	"github.com/ngojclee/lightroom-sync/internal/coordinator"
	"github.com/ngojclee/lightroom-sync/internal/ipc"
	"github.com/ngojclee/lightroom-sync/internal/monitor"
	winplatform "github.com/ngojclee/lightroom-sync/internal/platform/windows"
	syncpkg "github.com/ngojclee/lightroom-sync/internal/sync"
	"github.com/ngojclee/lightroom-sync/internal/tray"
)

var Version = "dev"

func main() {
	minimized := flag.Bool("minimized", false, "Start minimized to tray")
	flag.Parse()

	// --- Single instance guard ---
	mutex := winplatform.NewSingleInstance("LightroomSyncAgent_Mutex")
	acquired, err := mutex.TryAcquire()
	if err != nil {
		log.Fatalf("Failed to create mutex: %v", err)
	}
	if !acquired {
		log.Println("Another Agent instance is already running. Exiting.")
		os.Exit(0)
	}
	defer mutex.Release()

	// --- Load config ---
	cfgPath, err := config.DefaultPath()
	if err != nil {
		log.Fatalf("Config path error: %v", err)
	}

	cfgMgr := config.NewManager(cfgPath)
	if err := cfgMgr.Load(); err != nil {
		log.Printf("[WARN] Failed to load config, using defaults: %v", err)
	}
	migrationHint := ""
	if migrated, source, err := cfgMgr.MigrateFromLegacyPaths(config.LegacyPaths()); err != nil {
		log.Printf("[WARN] Legacy config migration failed: %v", err)
	} else if migrated {
		log.Printf("[INFO] Migrated legacy config from: %s", source)
		migrationHint = "Đã migrate config cũ từ: " + source
	}

	cfg := cfgMgr.Get()
	exePath, exeErr := os.Executable()
	if exeErr != nil {
		log.Printf("[WARN] Unable to resolve executable path for startup registry: %v", exeErr)
	} else {
		startupMgr := winplatform.NewStartupManager()
		if err := startupMgr.SetEnabled(cfg.StartWithWindows, exePath, cfg.StartMinimized); err != nil {
			log.Printf("[WARN] Failed to apply startup registry setting: %v", err)
		}
	}

	// --- Initialize core components ---
	eventBus := coordinator.NewEventBus(64)
	appState := coordinator.NewAppState()
	appState.SetAutoSync(cfg.AutoSync)
	if migrationHint != "" {
		appState.SetMigrationHint(migrationHint)
	}
	trayStatusPath, trayStatusErr := tray.DefaultStatusPath()
	if trayStatusErr != nil {
		log.Printf("[WARN] Tray status path unavailable: %v", trayStatusErr)
	}
	presetLocalRoot, presetRootErr := syncpkg.DefaultLightroomPresetRoot()
	if presetRootErr != nil {
		log.Printf("[WARN] Preset sync local root unavailable: %v", presetRootErr)
	}

	var orchestrator *coordinator.CatalogSyncOrchestrator

	// --- Context and goroutine lifecycle ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	shutdownMachine := hostNameOrUnknown()

	// --- Start event loop ---
	startManaged(ctx, &wg, "event-bus", func(ctx context.Context) {
		eventBus.Run(ctx)
	})

	// --- Start single-flight sync worker ---
	syncWorker := coordinator.NewSyncWorker(16, appState, eventBus)
	watchdog := coordinator.NewWatchdog(250*time.Millisecond, func(alert coordinator.WatchdogAlert) {
		log.Printf("[WARN] operation timeout op_id=%s name=%s started_at=%s deadline=%s",
			alert.OperationID, alert.OperationName, alert.StartedAt.Format(time.RFC3339), alert.DeadlineAt.Format(time.RFC3339))
		appState.SetWarning("Một thao tác đồng bộ đang chậm bất thường")
	})
	syncWorker.SetWatchdog(watchdog)
	startManaged(ctx, &wg, "sync-worker", func(ctx context.Context) {
		syncWorker.Run(ctx)
	})
	startManaged(ctx, &wg, "operation-watchdog", func(ctx context.Context) {
		watchdog.Run(ctx)
	})
	if strings.TrimSpace(trayStatusPath) != "" {
		startManaged(ctx, &wg, "tray-status-publisher", func(ctx context.Context) {
			writeTraySnapshot := func() {
				snap := appState.Snapshot()
				err := tray.WriteStatus(trayStatusPath, tray.StatusPayload{
					StatusText:     snap.StatusText,
					TrayColor:      snap.TrayColor,
					SyncInProgress: snap.SyncInProgress,
					SyncPaused:     snap.SyncPaused,
					AutoSync:       snap.AutoSync,
				})
				if err != nil {
					log.Printf("[WARN] Failed to publish tray status snapshot: %v", err)
				}
			}
			writeTraySnapshot()
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					writeTraySnapshot()
				}
			}
		})
	}

	enqueuePresetSync := func(trigger string) {
		current := cfgMgr.Get()
		if !current.PresetSyncEnabled {
			return
		}
		if strings.TrimSpace(current.BackupFolder) == "" {
			return
		}
		if strings.TrimSpace(presetLocalRoot) == "" {
			log.Printf("[WARN] Skip preset sync (%s): APPDATA Lightroom path unavailable", trigger)
			return
		}

		manager := syncpkg.NewPresetSyncManager(syncpkg.PresetSyncOptions{
			BackupDir:         current.BackupFolder,
			LocalLightroomDir: presetLocalRoot,
			Categories:        current.PresetCategories,
			MTimeTolerance:    2 * time.Second,
			Logf:              log.Printf,
		})

		jobName := "preset_sync_" + strings.ToLower(strings.TrimSpace(trigger))
		opID := fmt.Sprintf("%s_%d", jobName, time.Now().UTC().UnixNano())
		err := syncWorker.Enqueue(coordinator.SyncJob{
			Name:           jobName,
			OperationID:    opID,
			MaxRunDuration: 90 * time.Second,
			Execute: func(ctx context.Context) error {
				summary, err := manager.Sync(ctx)
				if err != nil {
					return err
				}
				log.Printf(
					"[INFO] Preset sync done trigger=%s pull=%d push=%d delete=%d logos=%d tracked=%d",
					trigger,
					summary.Pulled,
					summary.Pushed,
					summary.Deleted,
					summary.LogosCopied,
					summary.Tracked,
				)
				return nil
			},
		})
		if err != nil {
			log.Printf("[WARN] Failed to enqueue preset sync (%s): %v", trigger, err)
		}
	}

	if strings.TrimSpace(cfg.CatalogPath) != "" && strings.TrimSpace(cfg.BackupFolder) != "" {
		orchestrator = coordinator.NewCatalogSyncOrchestrator(coordinator.OrchestratorOptions{
			Machine:    hostNameOrUnknown(),
			CatalogDir: cfg.CatalogPath,
			BackupDir:  cfg.BackupFolder,
			AppState:   appState,
			Worker:     syncWorker,
			Manifest:   syncpkg.NewManifestManager(cfg.CatalogPath),
			GetAutoSync: func() bool {
				return cfgMgr.Get().AutoSync
			},
			GetLastSynced: func() string {
				return cfgMgr.Get().LastSyncedTimestamp
			},
			GetMaxBackups: func() int {
				return cfgMgr.Get().MaxCatalogBackups
			},
			SetLastSynced: cfgMgr.SetLastSyncedTimestamp,
			Logf:          log.Printf,
		})

		eventBus.On(coordinator.EvtLightroomStopped, func(evt coordinator.InternalEvent) {
			go func() {
				if err := orchestrator.RunPendingIfAny(); err != nil {
					log.Printf("[WARN] pending sync enqueue failed: %v", err)
				}
			}()
		})

		eventBus.On(coordinator.EvtNewBackupDetected, func(evt coordinator.InternalEvent) {
			zipPath, ok := evt.Payload.(string)
			if !ok || strings.TrimSpace(zipPath) == "" {
				return
			}

			go func(path string) {
				writeCtx, writeCancel := context.WithTimeout(context.Background(), 4*time.Second)
				defer writeCancel()
				if err := orchestrator.OnLocalBackupCreated(writeCtx, path); err != nil {
					log.Printf("[WARN] failed to write manifest for local backup %s: %v", path, err)
				}
			}(zipPath)
		})

		eventBus.On(coordinator.EvtSyncCompleted, func(evt coordinator.InternalEvent) {
			result, ok := evt.Payload.(coordinator.SyncResult)
			if !ok {
				return
			}
			if err := orchestrator.OnSyncCompleted(result.JobName); err != nil {
				log.Printf("[WARN] failed to update last_synced_timestamp: %v", err)
			}
			if isNetworkSyncJobName(result.JobName) {
				go enqueuePresetSync("after_network_sync")
			}
		})

		startManaged(ctx, &wg, "startup-manifest-check", func(ctx context.Context) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1200 * time.Millisecond):
			}
			checkCtx, checkCancel := context.WithTimeout(ctx, 8*time.Second)
			defer checkCancel()
			if err := orchestrator.CheckStartupManifest(checkCtx); err != nil {
				log.Printf("[WARN] startup manifest orchestration failed: %v", err)
			}
		})
	}

	eventBus.On(coordinator.EvtLightroomStopped, func(evt coordinator.InternalEvent) {
		go enqueuePresetSync("lightroom_stopped")
	})

	// --- Start Lightroom process monitor ---
	processDetector := winplatform.NewProcessDetector()
	lightroomMonitor := monitor.NewLightroomMonitor(
		processDetector,
		time.Duration(cfg.CheckInterval)*time.Second,
		[]string{"Lightroom.exe"},
		monitor.LightroomHooks{
			OnStarted: func() {
				appState.SetLightroomRunning(true)
				eventBus.Emit(coordinator.InternalEvent{Type: coordinator.EvtLightroomStarted})
			},
			OnStopped: func() {
				appState.SetLightroomRunning(false)
				eventBus.Emit(coordinator.InternalEvent{Type: coordinator.EvtLightroomStopped})
			},
			OnError: func(err error) {
				log.Printf("[WARN] lightroom monitor error: %v", err)
				appState.IncLightroomMonitorError()
			},
		},
	)
	startManaged(ctx, &wg, "lightroom-monitor", func(ctx context.Context) {
		lightroomMonitor.Run(ctx)
	})

	startManaged(ctx, &wg, "startup-preset-sync", func(ctx context.Context) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
		if appState.Snapshot().LightroomRunning {
			return
		}
		enqueuePresetSync("startup")
	})

	// --- Start network share health monitor (circuit breaker + recovery) ---
	var shareProbe monitor.NetworkProbe
	sharePaths := make([]string, 0, 2)
	if cfg.CatalogPath != "" {
		sharePaths = append(sharePaths, cfg.CatalogPath)
	}
	if cfg.BackupFolder != "" {
		sharePaths = append(sharePaths, cfg.BackupFolder)
	}
	if len(sharePaths) > 0 {
		shareProbe = monitor.NewPathProbe(sharePaths)
		shareHealth := monitor.NewShareHealthMonitor(monitor.ShareHealthConfig{
			CheckInterval:    time.Duration(cfg.CheckInterval) * time.Second,
			ProbeTimeout:     2 * time.Second,
			FailureThreshold: 3,
			OpenTimeout:      2 * time.Duration(cfg.CheckInterval) * time.Second,
		}, shareProbe, monitor.ShareHealthHooks{
			OnNetworkLost: func(err error) {
				log.Printf("[WARN] network share unstable: %v", err)
				appState.IncNetworkMonitorError()
				appState.SetWarning("Mất kết nối network share")
				eventBus.Emit(coordinator.InternalEvent{
					Type:    coordinator.EvtNetworkLost,
					Payload: err.Error(),
				})
			},
			OnNetworkRecovered: func() {
				log.Printf("[INFO] network share recovered")
				appState.RefreshDerivedStatus()
				eventBus.Emit(coordinator.InternalEvent{
					Type: coordinator.EvtNetworkAvailable,
				})
			},
		})
		startManaged(ctx, &wg, "share-health-monitor", func(ctx context.Context) {
			shareHealth.Run(ctx)
		})
	}

	// --- Start sleep/resume detector ---
	resumeDetector := monitor.NewResumeDetector(5*time.Second, 20*time.Second, monitor.ResumeHooks{
		OnResume: func(gap time.Duration) {
			log.Printf("[INFO] resume detected after gap=%s; revalidating network state", gap.Round(time.Second))
			appState.SetLastResumeGapSeconds(int(gap.Seconds()))
			appState.SetWarning("Đang kiểm tra lại network sau sleep/resume")

			if shareProbe == nil {
				appState.RefreshDerivedStatus()
				return
			}

			revalidateCtx, revalidateCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer revalidateCancel()
			if err := shareProbe(revalidateCtx); err != nil {
				log.Printf("[WARN] network revalidation after resume failed: %v", err)
				appState.IncNetworkMonitorError()
				appState.SetWarning("Network share chưa sẵn sàng sau resume")
				eventBus.Emit(coordinator.InternalEvent{
					Type:    coordinator.EvtNetworkLost,
					Payload: err.Error(),
				})
				return
			}

			log.Printf("[INFO] network revalidation after resume succeeded")
			appState.RefreshDerivedStatus()
			eventBus.Emit(coordinator.InternalEvent{
				Type:    coordinator.EvtNetworkAvailable,
				Payload: "resume_revalidated",
			})
		},
	})
	startManaged(ctx, &wg, "resume-detector", func(ctx context.Context) {
		resumeDetector.Run(ctx)
	})

	// --- Start backup monitor ---
	if cfg.BackupFolder != "" {
		backupMonitor := monitor.NewBackupMonitor(cfg.BackupFolder, time.Duration(cfg.CheckInterval)*time.Second, monitor.BackupHooks{
			OnNewBackup: func(info monitor.BackupInfo) {
				label := filepath.Base(info.Path)
				appState.SetLastBackup(label)
				eventBus.Emit(coordinator.InternalEvent{
					Type:    coordinator.EvtNewBackupDetected,
					Payload: info.Path,
				})
				log.Printf("[INFO] New backup detected: %s", info.Path)
			},
			OnError: func(err error) {
				log.Printf("[WARN] backup monitor error: %v", err)
				appState.IncBackupMonitorError()
			},
		})
		startManaged(ctx, &wg, "backup-monitor", func(ctx context.Context) {
			backupMonitor.Run(ctx)
		})
	}

	// --- Start lock heartbeat manager ---
	var lockMgr *monitor.LockManager
	if cfg.CatalogPath != "" {
		lockMgr = monitor.NewLockManager(cfg.CatalogPath)
		heartbeat := monitor.NewHeartbeatManager(lockMgr, shutdownMachine, monitor.HeartbeatConfig{
			Interval:        time.Duration(cfg.HeartbeatInterval) * time.Second,
			RetryBase:       500 * time.Millisecond,
			RetryMax:        5 * time.Second,
			MaxRetries:      3,
			ShutdownTimeout: 2 * time.Second,
		}, monitor.HeartbeatHooks{
			OnHeartbeat: func(info monitor.LockInfo) {
				appState.SetLock(info.Machine, string(info.Status))
			},
			OnError: func(err error) {
				log.Printf("[WARN] lock heartbeat error: %v", err)
				appState.IncLockMonitorError()
				appState.SetLock(shutdownMachine, "ERROR")
			},
		})

		startManaged(ctx, &wg, "lock-heartbeat", func(ctx context.Context) {
			heartbeat.Run(ctx)
		})
	}

	// --- Start IPC server (UI <-> Agent) ---
	ipcServer := ipc.NewServer(ipc.PipeName, ipc.DefaultRequestTimeout, func(reqCtx context.Context, req ipc.Request) ipc.Response {
		switch req.Command {
		case ipc.CmdPing:
			return ipc.Response{
				Success: true,
				Data:    map[string]string{"message": "pong"},
				Code:    ipc.CodeOK,
			}
		case ipc.CmdGetStatus:
			return ipc.Response{
				Success: true,
				Data:    appState.Snapshot(),
				Code:    ipc.CodeOK,
			}
		case ipc.CmdGetConfig:
			return ipc.Response{
				Success: true,
				Data:    configToIPCSnapshot(cfgMgr.Get()),
				Code:    ipc.CodeOK,
			}
		case ipc.CmdSaveConfig:
			payload, err := decodePayload[ipc.SaveConfigPayload](req.Payload)
			if err != nil {
				return ipc.Response{
					Success: false,
					Error:   fmt.Sprintf("invalid save_config payload: %v", err),
					Code:    ipc.CodeBadRequest,
				}
			}
			if isEmptyConfigPatch(payload) {
				return ipc.Response{
					Success: false,
					Error:   "save_config payload is empty",
					Code:    ipc.CodeBadRequest,
				}
			}
			if err := applyConfigPatch(cfgMgr, payload); err != nil {
				return ipc.Response{
					Success: false,
					Error:   err.Error(),
					Code:    ipc.CodeBadRequest,
				}
			}

			updated := cfgMgr.Get()
			appState.SetAutoSync(updated.AutoSync)
			if exePath != "" {
				startupMgr := winplatform.NewStartupManager()
				if err := startupMgr.SetEnabled(updated.StartWithWindows, exePath, updated.StartMinimized); err != nil {
					log.Printf("[WARN] Failed to apply startup registry setting after save_config: %v", err)
				}
			}

			return ipc.Response{
				Success: true,
				Data:    configToIPCSnapshot(updated),
				Code:    ipc.CodeOK,
			}
		case ipc.CmdGetBackups:
			current := cfgMgr.Get()
			if strings.TrimSpace(current.BackupFolder) == "" {
				return ipc.Response{
					Success: false,
					Error:   "backup folder is not configured",
					Code:    ipc.CodeBadRequest,
				}
			}
			backups, err := monitor.ListZipBackups(reqCtx, current.BackupFolder)
			if err != nil {
				return ipc.Response{
					Success: false,
					Error:   err.Error(),
					Code:    ipc.CodeInternalError,
				}
			}
			result := make([]ipc.BackupInfo, 0, len(backups))
			for _, item := range backups {
				result = append(result, ipc.BackupInfo{
					Path:        item.Path,
					CatalogName: strings.TrimSuffix(filepath.Base(item.Path), filepath.Ext(item.Path)),
					Size:        item.Size,
					ModTime:     item.ModTime,
				})
			}
			return ipc.Response{
				Success: true,
				Data:    result,
				Code:    ipc.CodeOK,
			}
		case ipc.CmdSyncNow:
			current := cfgMgr.Get()
			if strings.TrimSpace(current.BackupFolder) == "" || strings.TrimSpace(current.CatalogPath) == "" {
				return ipc.Response{
					Success: false,
					Error:   "backup/catalog paths are not configured",
					Code:    ipc.CodeBadRequest,
				}
			}

			jobName := "manual_sync_now_latest_backup"
			err := syncWorker.Enqueue(coordinator.SyncJob{
				Name:           jobName,
				OperationID:    "manual_sync_now",
				MaxRunDuration: 120 * time.Second,
				Execute: func(ctx context.Context) error {
					backups, err := monitor.ListZipBackups(ctx, current.BackupFolder)
					if err != nil {
						return err
					}
					if len(backups) == 0 {
						return errors.New("no backup zip found")
					}
					zipPath := backups[0].Path
					return syncpkg.RestoreCatalogFromZip(ctx, zipPath, current.CatalogPath, syncpkg.DefaultRestoreOptions())
				},
			})
			if err != nil {
				return ipc.Response{
					Success: false,
					Error:   err.Error(),
					Code:    ipc.CodeInternalError,
				}
			}
			return ipc.Response{
				Success: true,
				Data:    map[string]string{"queued": "true"},
				Code:    ipc.CodeOK,
			}
		case ipc.CmdSyncBackup:
			current := cfgMgr.Get()
			if strings.TrimSpace(current.CatalogPath) == "" {
				return ipc.Response{
					Success: false,
					Error:   "catalog path is not configured",
					Code:    ipc.CodeBadRequest,
				}
			}
			payload, err := decodePayload[ipc.SyncBackupPayload](req.Payload)
			if err != nil {
				return ipc.Response{
					Success: false,
					Error:   fmt.Sprintf("invalid sync_backup payload: %v", err),
					Code:    ipc.CodeBadRequest,
				}
			}
			zipPath := strings.TrimSpace(payload.ZipPath)
			if zipPath == "" {
				return ipc.Response{
					Success: false,
					Error:   "zip_path is required",
					Code:    ipc.CodeBadRequest,
				}
			}
			if !strings.EqualFold(filepath.Ext(zipPath), ".zip") {
				return ipc.Response{
					Success: false,
					Error:   "zip_path must point to a .zip file",
					Code:    ipc.CodeBadRequest,
				}
			}
			if _, err := os.Stat(zipPath); err != nil {
				return ipc.Response{
					Success: false,
					Error:   fmt.Sprintf("backup zip not accessible: %v", err),
					Code:    ipc.CodeBadRequest,
				}
			}
			jobName := "manual_sync_selected_backup"
			err = syncWorker.Enqueue(coordinator.SyncJob{
				Name:           jobName,
				OperationID:    fmt.Sprintf("manual_sync_backup_%d", time.Now().UTC().UnixNano()),
				MaxRunDuration: 120 * time.Second,
				Execute: func(ctx context.Context) error {
					return syncpkg.RestoreCatalogFromZip(ctx, zipPath, current.CatalogPath, syncpkg.DefaultRestoreOptions())
				},
			})
			if err != nil {
				return ipc.Response{
					Success: false,
					Error:   err.Error(),
					Code:    ipc.CodeInternalError,
				}
			}
			return ipc.Response{
				Success: true,
				Data: map[string]string{
					"queued":   "true",
					"zip_path": zipPath,
				},
				Code: ipc.CodeOK,
			}
		default:
			return ipc.Response{
				Success: false,
				Error:   "unsupported command",
				Code:    ipc.CodeUnknownCmd,
			}
		}
	})
	startManaged(ctx, &wg, "ipc-server", func(ctx context.Context) {
		if err := ipcServer.Start(ctx); err != nil {
			log.Printf("[ERROR] IPC server stopped: %v", err)
		}
	})

	// --- Start tray host ---
	uiExecutable := resolveUIExecutable(exePath)
	trayManager := tray.NewManager(tray.Options{
		AppName:      "Lightroom Sync",
		AgentPID:     os.Getpid(),
		UIExecutable: uiExecutable,
		PipeName:     ipc.PipeName,
		StatusPath:   trayStatusPath,
	})
	if err := trayManager.Start(ctx); err != nil {
		log.Printf("[WARN] Tray bootstrap failed: %v", err)
	} else {
		log.Printf("[INFO] Tray bootstrap started (ui=%s)", uiExecutable)
	}
	defer func() {
		if err := trayManager.Stop(); err != nil {
			log.Printf("[WARN] Tray shutdown error: %v", err)
		}
	}()

	log.Printf("[INFO] LightroomSync Agent %s started (minimized=%v)", Version, *minimized)
	log.Printf("[INFO] Config: %s", cfgPath)
	log.Printf("[INFO] State: %s", appState.Snapshot().StatusText)

	// --- Wait for shutdown signal ---
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	log.Println("[INFO] Shutting down Agent...")

	// Stop managed loops.
	cancel()

	// Ensure listener unblocks quickly.
	_ = ipcServer.Close()

	// Best-effort OFFLINE write in case shutdown races with heartbeat cancel.
	if lockMgr != nil {
		lockCtx, lockCancel := context.WithTimeout(context.Background(), 2*time.Second)
		writeErr := lockMgr.WriteLock(lockCtx, monitor.LockInfo{
			Status:    monitor.LockOffline,
			Machine:   shutdownMachine,
			Timestamp: time.Now().UTC(),
		})
		lockCancel()
		if writeErr != nil {
			log.Printf("[WARN] Failed to write OFFLINE lock during shutdown: %v", writeErr)
		}
	}

	if !waitGroupWithTimeout(&wg, 5*time.Second) {
		log.Println("[WARN] Shutdown timed out waiting for background workers.")
	}

	log.Println("[INFO] Agent stopped.")
}

func startManaged(ctx context.Context, wg *sync.WaitGroup, name string, run func(context.Context)) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		run(ctx)
		log.Printf("[DEBUG] worker stopped: %s", name)
	}()
}

func waitGroupWithTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func hostNameOrUnknown() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "UNKNOWN"
	}
	return host
}

func isNetworkSyncJobName(jobName string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(jobName)), "network_sync_")
}

func configToIPCSnapshot(cfg config.Config) ipc.ConfigSnapshot {
	return ipc.ConfigSnapshot{
		BackupFolder:        cfg.BackupFolder,
		CatalogPath:         cfg.CatalogPath,
		StartWithWindows:    cfg.StartWithWindows,
		StartMinimized:      cfg.StartMinimized,
		MinimizeToTray:      cfg.MinimizeToTray,
		AutoSync:            cfg.AutoSync,
		HeartbeatInterval:   cfg.HeartbeatInterval,
		CheckInterval:       cfg.CheckInterval,
		LockTimeout:         cfg.LockTimeout,
		MaxCatalogBackups:   cfg.MaxCatalogBackups,
		PresetSyncEnabled:   cfg.PresetSyncEnabled,
		PresetCategories:    append([]string(nil), cfg.PresetCategories...),
		LastSyncedTimestamp: cfg.LastSyncedTimestamp,
	}
}

func applyConfigPatch(cfgMgr *config.Manager, patch ipc.SaveConfigPayload) error {
	cfg := cfgMgr.Get()

	if patch.BackupFolder != nil {
		cfg.BackupFolder = strings.TrimSpace(*patch.BackupFolder)
	}
	if patch.CatalogPath != nil {
		cfg.CatalogPath = strings.TrimSpace(*patch.CatalogPath)
	}
	if patch.StartWithWindows != nil {
		cfg.StartWithWindows = *patch.StartWithWindows
	}
	if patch.StartMinimized != nil {
		cfg.StartMinimized = *patch.StartMinimized
	}
	if patch.MinimizeToTray != nil {
		cfg.MinimizeToTray = *patch.MinimizeToTray
	}
	if patch.AutoSync != nil {
		cfg.AutoSync = *patch.AutoSync
	}
	if patch.HeartbeatInterval != nil {
		if *patch.HeartbeatInterval <= 0 {
			return errors.New("heartbeat_interval must be > 0")
		}
		cfg.HeartbeatInterval = *patch.HeartbeatInterval
	}
	if patch.CheckInterval != nil {
		if *patch.CheckInterval <= 0 {
			return errors.New("check_interval must be > 0")
		}
		cfg.CheckInterval = *patch.CheckInterval
	}
	if patch.LockTimeout != nil {
		if *patch.LockTimeout <= 0 {
			return errors.New("lock_timeout must be > 0")
		}
		cfg.LockTimeout = *patch.LockTimeout
	}
	if patch.MaxCatalogBackups != nil {
		if *patch.MaxCatalogBackups <= 0 {
			return errors.New("max_catalog_backups must be > 0")
		}
		cfg.MaxCatalogBackups = *patch.MaxCatalogBackups
	}
	if patch.PresetSyncEnabled != nil {
		cfg.PresetSyncEnabled = *patch.PresetSyncEnabled
	}
	if patch.PresetCategories != nil {
		cfg.PresetCategories = normalizeCategories(*patch.PresetCategories)
	}
	if patch.LastSyncedTimestamp != nil {
		cfg.LastSyncedTimestamp = strings.TrimSpace(*patch.LastSyncedTimestamp)
	}

	if len(cfg.PresetCategories) == 0 {
		cfg.PresetCategories = []string{"Export Presets", "Develop Presets", "Watermarks"}
	}

	return cfgMgr.Update(cfg)
}

func normalizeCategories(raw []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	return out
}

func decodePayload[T any](raw any) (T, error) {
	var out T
	if raw == nil {
		return out, nil
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return out, err
	}
	if len(data) == 0 || string(data) == "null" {
		return out, nil
	}

	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	return out, nil
}

func isEmptyConfigPatch(p ipc.SaveConfigPayload) bool {
	return p.BackupFolder == nil &&
		p.CatalogPath == nil &&
		p.StartWithWindows == nil &&
		p.StartMinimized == nil &&
		p.MinimizeToTray == nil &&
		p.AutoSync == nil &&
		p.HeartbeatInterval == nil &&
		p.CheckInterval == nil &&
		p.LockTimeout == nil &&
		p.MaxCatalogBackups == nil &&
		p.PresetSyncEnabled == nil &&
		p.PresetCategories == nil &&
		p.LastSyncedTimestamp == nil
}

func resolveUIExecutable(agentExePath string) string {
	if strings.TrimSpace(agentExePath) == "" {
		return ""
	}
	dir := filepath.Dir(agentExePath)
	candidates := []string{
		filepath.Join(dir, "LightroomSyncUI.exe"),
		filepath.Join(dir, "ui.exe"),
		filepath.Join(dir, "LightroomSyncUI"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}
