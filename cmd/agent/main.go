package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/ngojclee/lightroom-sync/internal/config"
	"github.com/ngojclee/lightroom-sync/internal/coordinator"
	"github.com/ngojclee/lightroom-sync/internal/ipc"
	"github.com/ngojclee/lightroom-sync/internal/monitor"
	winplatform "github.com/ngojclee/lightroom-sync/internal/platform/windows"
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
			err := syncWorker.Enqueue(coordinator.SyncJob{
				Name:           "manual_sync_now",
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
