package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIntegration_CatalogRestoreAndPresetRoundTrip_OnTempDirs(t *testing.T) {
	root := t.TempDir()

	// --- Catalog restore setup ---
	catalogDir := filepath.Join(root, "catalog_local")
	backupDir := filepath.Join(root, "network_backup")
	localLightroomDir := filepath.Join(root, "lightroom_local")
	statePath := filepath.Join(localLightroomDir, ".lightroom-sync", "preset_state.json")

	if err := os.MkdirAll(catalogDir, 0o755); err != nil {
		t.Fatalf("mkdir catalog dir: %v", err)
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("mkdir backup dir: %v", err)
	}

	oldCatalog := filepath.Join(catalogDir, "Old.lrcat")
	if err := os.WriteFile(oldCatalog, []byte("old-catalog"), 0o644); err != nil {
		t.Fatalf("write old catalog: %v", err)
	}

	networkZip := filepath.Join(backupDir, "CatalogMachineA_20260330.zip")
	err := createZip(networkZip, map[string]string{
		"Wrapper/NewCatalog.lrcat":                     "new-catalog-content",
		"Wrapper/NewCatalog Previews.lrdata/cache.txt": "preview-cache",
	})
	if err != nil {
		t.Fatalf("create catalog zip: %v", err)
	}

	if err := RestoreCatalogFromZip(context.Background(), networkZip, catalogDir, DefaultRestoreOptions()); err != nil {
		t.Fatalf("RestoreCatalogFromZip() error = %v", err)
	}

	if _, err := os.Stat(oldCatalog); !os.IsNotExist(err) {
		t.Fatalf("old catalog should be removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(catalogDir, "NewCatalog.lrcat")); err != nil {
		t.Fatalf("new catalog should exist after restore: %v", err)
	}
	if _, err := os.Stat(filepath.Join(catalogDir, "NewCatalog Previews.lrdata", "cache.txt")); err != nil {
		t.Fatalf("preview artifact should be restored: %v", err)
	}

	// --- Preset sync setup ---
	localPreset := filepath.Join(localLightroomDir, "Develop Presets", "Portrait.xmp")
	if err := os.MkdirAll(filepath.Dir(localPreset), 0o755); err != nil {
		t.Fatalf("mkdir local preset dir: %v", err)
	}
	if err := os.WriteFile(localPreset, []byte("local-v1"), 0o644); err != nil {
		t.Fatalf("write local preset: %v", err)
	}

	mgr := NewPresetSyncManager(PresetSyncOptions{
		BackupDir:         backupDir,
		LocalLightroomDir: localLightroomDir,
		Categories:        []string{"Develop Presets"},
		StatePath:         statePath,
		MTimeTolerance:    2 * time.Second,
	})

	// First cycle should push local preset to network.
	first, err := mgr.Sync(context.Background())
	if err != nil {
		t.Fatalf("preset sync first cycle error = %v", err)
	}
	if first.Pushed == 0 {
		t.Fatalf("expected first preset sync to push at least one file, got %+v", first)
	}

	networkPreset := filepath.Join(backupDir, "Presets", "Develop Presets", "Portrait.xmp")
	if _, err := os.Stat(networkPreset); err != nil {
		t.Fatalf("network preset missing after first sync: %v", err)
	}

	// Simulate remote machine modifies preset; second cycle should pull to local.
	base := time.Now().UTC().Add(-15 * time.Second).Round(time.Second)
	if err := os.Chtimes(localPreset, base, base); err != nil {
		t.Fatalf("set local preset mtime: %v", err)
	}
	remoteTime := base.Add(5 * time.Second)
	if err := os.WriteFile(networkPreset, []byte("remote-v2"), 0o644); err != nil {
		t.Fatalf("write network preset new content: %v", err)
	}
	if err := os.Chtimes(networkPreset, remoteTime, remoteTime); err != nil {
		t.Fatalf("set network preset mtime: %v", err)
	}

	second, err := mgr.Sync(context.Background())
	if err != nil {
		t.Fatalf("preset sync second cycle error = %v", err)
	}
	if second.Pulled == 0 {
		t.Fatalf("expected second preset sync to pull at least one file, got %+v", second)
	}

	gotLocal, err := os.ReadFile(localPreset)
	if err != nil {
		t.Fatalf("read local preset after pull: %v", err)
	}
	if string(gotLocal) != "remote-v2" {
		t.Fatalf("local preset content = %q, want %q", string(gotLocal), "remote-v2")
	}

	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("preset state file should be committed after successful cycles: %v", err)
	}
}
