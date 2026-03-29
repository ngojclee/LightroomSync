package main

import (
	"context"
	"flag"
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

	if orchestrator != nil {
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
			},
		},
	)
	startManaged(ctx, &wg, "lightroom-monitor", func(ctx context.Context) {
		lightroomMonitor.Run(ctx)
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
			appState.SetWarning("Đang kiểm tra lại network sau sleep/resume")

			if shareProbe == nil {
				appState.RefreshDerivedStatus()
				return
			}

			revalidateCtx, revalidateCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer revalidateCancel()
			if err := shareProbe(revalidateCtx); err != nil {
				log.Printf("[WARN] network revalidation after resume failed: %v", err)
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
		case ipc.CmdSyncNow:
			jobName := "manual_sync_now"
			if appState.Snapshot().LightroomRunning {
				jobName = "manual_sync_now_pending_lrcat_open"
			}
			err := syncWorker.Enqueue(coordinator.SyncJob{
				Name:           jobName,
				OperationID:    "manual_sync_now",
				MaxRunDuration: 5 * time.Second,
				Execute: func(ctx context.Context) error {
					// Placeholder sync action for Phase 3 queue wiring.
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(50 * time.Millisecond):
						return nil
					}
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

	// --- Start tray icon ---
	// TODO(phase1.2): Wire real tray with systray library
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
