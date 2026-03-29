package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
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

	// --- Context for graceful shutdown ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- Start event loop ---
	go eventBus.Run(ctx)

	// --- Start single-flight sync worker ---
	syncWorker := coordinator.NewSyncWorker(16, appState, eventBus)
	go syncWorker.Run(ctx)

	// --- Start lock heartbeat manager ---
	if cfg.CatalogPath != "" {
		machine, err := os.Hostname()
		if err != nil || machine == "" {
			machine = "UNKNOWN"
		}
		lockMgr := monitor.NewLockManager(cfg.CatalogPath)
		heartbeat := monitor.NewHeartbeatManager(lockMgr, machine, monitor.HeartbeatConfig{
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
				appState.SetLock(machine, "ERROR")
			},
		})

		go heartbeat.Run(ctx)
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
				Name: "manual_sync_now",
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
	go func() {
		if err := ipcServer.Start(ctx); err != nil {
			log.Printf("[ERROR] IPC server stopped: %v", err)
		}
	}()

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
	cancel()
	_ = ipcServer.Close()

	// TODO: Write OFFLINE lock, stop heartbeat, cleanup
	log.Println("[INFO] Agent stopped.")
}
