package coordinator

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	syncpkg "github.com/ngojclee/lightroom-sync/internal/sync"
)

type manifestStore interface {
	ReadManifest(ctx context.Context) (*syncpkg.Manifest, error)
	WriteManifest(ctx context.Context, manifest syncpkg.Manifest) error
}

type syncEnqueuer interface {
	Enqueue(job SyncJob) error
}

// CatalogSyncOrchestrator coordinates startup/pending sync and manifest updates.
type CatalogSyncOrchestrator struct {
	mu sync.Mutex

	machine    string
	catalogDir string
	backupDir  string
	appState   *AppState
	worker     syncEnqueuer
	manifest   manifestStore

	getAutoSync   func() bool
	getLastSynced func() string
	getMaxBackups func() int
	setLastSynced func(string) error
	logf          func(string, ...any)

	pendingManifest *syncpkg.Manifest
}

type OrchestratorOptions struct {
	Machine       string
	CatalogDir    string
	BackupDir     string
	AppState      *AppState
	Worker        syncEnqueuer
	Manifest      manifestStore
	GetAutoSync   func() bool
	GetLastSynced func() string
	GetMaxBackups func() int
	SetLastSynced func(string) error
	Logf          func(string, ...any)
}

func NewCatalogSyncOrchestrator(opts OrchestratorOptions) *CatalogSyncOrchestrator {
	logf := opts.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}

	return &CatalogSyncOrchestrator{
		machine:         opts.Machine,
		catalogDir:      opts.CatalogDir,
		backupDir:       opts.BackupDir,
		appState:        opts.AppState,
		worker:          opts.Worker,
		manifest:        opts.Manifest,
		getAutoSync:     opts.GetAutoSync,
		getLastSynced:   opts.GetLastSynced,
		getMaxBackups:   opts.GetMaxBackups,
		setLastSynced:   opts.SetLastSynced,
		logf:            logf,
		pendingManifest: nil,
	}
}

// CheckStartupManifest checks remote manifest on startup and queues/pends sync if needed.
func (o *CatalogSyncOrchestrator) CheckStartupManifest(ctx context.Context) error {
	if o.manifest == nil || o.worker == nil || o.getAutoSync == nil || o.getLastSynced == nil {
		return nil
	}
	if !o.getAutoSync() {
		o.logf("[INFO] AutoSync disabled; skipping startup manifest check")
		return nil
	}

	manifest, err := o.manifest.ReadManifest(ctx)
	if err != nil {
		return fmt.Errorf("read startup manifest: %w", err)
	}
	if manifest == nil {
		return nil
	}

	shouldSync, reason := syncpkg.ShouldSyncFromNetwork(manifest, o.machine, o.getLastSynced(), o.backupDir)
	if !shouldSync {
		o.logf("[INFO] Startup manifest sync skipped: %s", reason)
		return nil
	}

	return o.queueOrPend(*manifest, "startup")
}

// RunPendingIfAny enqueues pending sync when Lightroom is no longer running.
func (o *CatalogSyncOrchestrator) RunPendingIfAny() error {
	if o.worker == nil {
		return nil
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	if o.pendingManifest == nil {
		return nil
	}
	if o.appState != nil && o.appState.Snapshot().LightroomRunning {
		return nil
	}

	manifest := *o.pendingManifest
	if err := o.enqueueManifestLocked(manifest, "pending"); err != nil {
		return err
	}
	o.pendingManifest = nil
	return nil
}

// OnLocalBackupCreated writes network manifest for the detected local backup zip.
func (o *CatalogSyncOrchestrator) OnLocalBackupCreated(ctx context.Context, zipPath string) error {
	if o.manifest == nil {
		return nil
	}
	if o.backupDir == "" {
		return fmt.Errorf("backup directory is empty")
	}

	manifest, err := syncpkg.NewManifestForBackup(o.machine, zipPath, o.backupDir)
	if err != nil {
		return err
	}
	if err := o.manifest.WriteManifest(ctx, manifest); err != nil {
		return err
	}
	o.logf("[INFO] Wrote network manifest for local backup: %s", filepath.Base(zipPath))
	return nil
}

// OnSyncCompleted persists last_synced timestamp for successful network sync jobs.
func (o *CatalogSyncOrchestrator) OnSyncCompleted(jobName string) error {
	if o.setLastSynced == nil {
		return nil
	}
	if !isNetworkSyncJob(jobName) {
		return nil
	}
	ts := time.Now().Format("2006-01-02T15:04:05.999999")
	return o.setLastSynced(ts)
}

func (o *CatalogSyncOrchestrator) queueOrPend(manifest syncpkg.Manifest, reason string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.appState != nil && o.appState.Snapshot().LightroomRunning {
		m := manifest
		o.pendingManifest = &m
		o.appState.SetWarning("Đã phát hiện backup mới, chờ Lightroom đóng để sync")
		o.logf("[INFO] Manifest sync is pending until Lightroom stops")
		return nil
	}

	if err := o.enqueueManifestLocked(manifest, reason); err != nil {
		return err
	}
	o.pendingManifest = nil
	return nil
}

func (o *CatalogSyncOrchestrator) enqueueManifestLocked(manifest syncpkg.Manifest, reason string) error {
	if o.backupDir == "" || o.catalogDir == "" {
		return fmt.Errorf("orchestrator requires backup and catalog directories")
	}

	jobName := fmt.Sprintf("network_sync_%s", reason)
	operationID := fmt.Sprintf("%s_%d", jobName, time.Now().UTC().UnixNano())

	err := o.worker.Enqueue(SyncJob{
		Name:           jobName,
		OperationID:    operationID,
		MaxRunDuration: 120 * time.Second,
		Execute: func(ctx context.Context) error {
			zipPath := filepath.Join(o.backupDir, filepath.FromSlash(manifest.ZipFile))

			// Pre-validate network backup to prevent overwriting with corrupted files
			if err := syncpkg.ValidateZipIntegrity(ctx, zipPath); err != nil {
				if o.appState != nil {
					o.appState.SetError("Network Backup Corrupted!")
				}
				return fmt.Errorf("network backup corrupted, sync aborted: %w", err)
			}
			if o.appState != nil {
				o.appState.ClearError()
			}

			maxBackups := 5
			if o.getMaxBackups != nil && o.getMaxBackups() > 0 {
				maxBackups = o.getMaxBackups()
			}

			if _, err := syncpkg.CreatePreSyncBackup(ctx, o.catalogDir, o.machine, maxBackups, time.Now().UTC()); err != nil {
				return fmt.Errorf("create pre-sync backup: %w", err)
			}

			if _, err := syncpkg.CleanupZipRetention(o.backupDir, maxBackups); err != nil {
				o.logf("[WARN] network backup retention cleanup failed: %v", err)
			}

			return syncpkg.RestoreCatalogFromZip(ctx, zipPath, o.catalogDir, syncpkg.DefaultRestoreOptions())
		},
	})
	if err != nil {
		return err
	}

	o.logf("[INFO] Enqueued network manifest sync job: %s", jobName)
	if o.appState != nil {
		o.appState.RefreshDerivedStatus()
	}
	return nil
}

func isNetworkSyncJob(jobName string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(jobName)), "network_sync_")
}
