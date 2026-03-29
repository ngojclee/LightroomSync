package coordinator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	syncpkg "github.com/ngojclee/lightroom-sync/internal/sync"
)

type fakeManifestStore struct {
	readResult *syncpkg.Manifest
	readErr    error
	written    []syncpkg.Manifest
	writeErr   error
}

func (f *fakeManifestStore) ReadManifest(ctx context.Context) (*syncpkg.Manifest, error) {
	return f.readResult, f.readErr
}

func (f *fakeManifestStore) WriteManifest(ctx context.Context, manifest syncpkg.Manifest) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.written = append(f.written, manifest)
	return nil
}

type fakeWorker struct {
	jobs       []SyncJob
	enqueueErr error
}

func (f *fakeWorker) Enqueue(job SyncJob) error {
	if f.enqueueErr != nil {
		return f.enqueueErr
	}
	f.jobs = append(f.jobs, job)
	return nil
}

func TestOrchestrator_StartUpCheck_QueuesWhenEligible(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, "backup")
	catalogDir := filepath.Join(root, "catalog")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(catalogDir, 0o755); err != nil {
		t.Fatal(err)
	}

	zipName := "remote.zip"
	zipPath := filepath.Join(backupDir, zipName)
	if err := os.WriteFile(zipPath, []byte("zipcontent"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifestStore := &fakeManifestStore{
		readResult: &syncpkg.Manifest{
			Machine:   "OTHER-PC",
			Timestamp: "2026-03-30T12:00:00.000000",
			ZipFile:   zipName,
			ZipSize:   int64(len("zipcontent")),
		},
	}
	worker := &fakeWorker{}
	state := NewAppState()

	orch := NewCatalogSyncOrchestrator(OrchestratorOptions{
		Machine:       "LOCAL-PC",
		CatalogDir:    catalogDir,
		BackupDir:     backupDir,
		AppState:      state,
		Worker:        worker,
		Manifest:      manifestStore,
		GetAutoSync:   func() bool { return true },
		GetLastSynced: func() string { return "" },
	})

	if err := orch.CheckStartupManifest(context.Background()); err != nil {
		t.Fatalf("startup check failed: %v", err)
	}
	if len(worker.jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(worker.jobs))
	}
	if worker.jobs[0].Name != "network_sync_startup" {
		t.Fatalf("job name = %s, want network_sync_startup", worker.jobs[0].Name)
	}
}

func TestOrchestrator_StartUpCheck_PendsWhenLightroomRunning(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, "backup")
	catalogDir := filepath.Join(root, "catalog")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(catalogDir, 0o755); err != nil {
		t.Fatal(err)
	}
	zipName := "a.zip"
	if err := os.WriteFile(filepath.Join(backupDir, zipName), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifestStore := &fakeManifestStore{
		readResult: &syncpkg.Manifest{
			Machine:   "REMOTE",
			Timestamp: "2026-03-30T12:00:00.000000",
			ZipFile:   zipName,
			ZipSize:   1,
		},
	}
	worker := &fakeWorker{}
	state := NewAppState()
	state.SetLightroomRunning(true)

	orch := NewCatalogSyncOrchestrator(OrchestratorOptions{
		Machine:       "LOCAL",
		CatalogDir:    catalogDir,
		BackupDir:     backupDir,
		AppState:      state,
		Worker:        worker,
		Manifest:      manifestStore,
		GetAutoSync:   func() bool { return true },
		GetLastSynced: func() string { return "" },
	})

	_ = orch.CheckStartupManifest(context.Background())
	if len(worker.jobs) != 0 {
		t.Fatalf("jobs = %d, want 0 while Lightroom running", len(worker.jobs))
	}

	state.SetLightroomRunning(false)
	if err := orch.RunPendingIfAny(); err != nil {
		t.Fatalf("run pending failed: %v", err)
	}
	if len(worker.jobs) != 1 {
		t.Fatalf("jobs = %d, want 1 after Lightroom stopped", len(worker.jobs))
	}
}

func TestOrchestrator_OnLocalBackupCreated_WritesManifest(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, "backup")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(backupDir, "local.zip")
	if err := os.WriteFile(zipPath, []byte("zipdata"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifestStore := &fakeManifestStore{}
	orch := NewCatalogSyncOrchestrator(OrchestratorOptions{
		Machine:       "LOCAL",
		CatalogDir:    filepath.Join(root, "catalog"),
		BackupDir:     backupDir,
		AppState:      NewAppState(),
		Worker:        &fakeWorker{},
		Manifest:      manifestStore,
		GetAutoSync:   func() bool { return true },
		GetLastSynced: func() string { return "" },
	})

	if err := orch.OnLocalBackupCreated(context.Background(), zipPath); err != nil {
		t.Fatalf("OnLocalBackupCreated failed: %v", err)
	}
	if len(manifestStore.written) != 1 {
		t.Fatalf("written manifest count = %d, want 1", len(manifestStore.written))
	}
	if manifestStore.written[0].ZipFile != "local.zip" {
		t.Fatalf("zip_file = %s, want local.zip", manifestStore.written[0].ZipFile)
	}
}

func TestOrchestrator_OnSyncCompleted_UpdatesTimestampOnlyForNetworkJobs(t *testing.T) {
	var updated []string
	orch := NewCatalogSyncOrchestrator(OrchestratorOptions{
		SetLastSynced: func(ts string) error {
			updated = append(updated, ts)
			return nil
		},
	})

	if err := orch.OnSyncCompleted("manual_sync_now"); err != nil {
		t.Fatalf("OnSyncCompleted manual error: %v", err)
	}
	if len(updated) != 0 {
		t.Fatalf("updated count = %d, want 0 for manual job", len(updated))
	}

	if err := orch.OnSyncCompleted("network_sync_startup"); err != nil {
		t.Fatalf("OnSyncCompleted network error: %v", err)
	}
	if len(updated) != 1 {
		t.Fatalf("updated count = %d, want 1 for network job", len(updated))
	}
}

func TestOrchestrator_EnqueueFailurePropagates(t *testing.T) {
	root := t.TempDir()
	backupDir := filepath.Join(root, "backup")
	catalogDir := filepath.Join(root, "catalog")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(catalogDir, 0o755); err != nil {
		t.Fatal(err)
	}
	zipName := "existing.zip"
	if err := os.WriteFile(filepath.Join(backupDir, zipName), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifestStore := &fakeManifestStore{
		readResult: &syncpkg.Manifest{
			Machine:   "REMOTE",
			Timestamp: time.Now().Format("2006-01-02T15:04:05.999999"),
			ZipFile:   zipName,
			ZipSize:   2,
		},
	}
	worker := &fakeWorker{enqueueErr: errors.New("queue full")}
	orch := NewCatalogSyncOrchestrator(OrchestratorOptions{
		Machine:       "LOCAL",
		CatalogDir:    catalogDir,
		BackupDir:     backupDir,
		AppState:      NewAppState(),
		Worker:        worker,
		Manifest:      manifestStore,
		GetAutoSync:   func() bool { return true },
		GetLastSynced: func() string { return "" },
	})

	err := orch.CheckStartupManifest(context.Background())
	if err == nil {
		t.Fatal("expected startup check error from enqueue failure")
	}
}
