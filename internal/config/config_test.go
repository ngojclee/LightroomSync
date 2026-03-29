package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoad_PythonFixture verifies Go can parse Python-generated config YAML.
func TestLoad_PythonFixture(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "testdata", "fixtures", "config.yaml")

	mgr := NewManager(fixturePath)
	if err := mgr.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}

	cfg := mgr.Get()

	if cfg.BackupFolder != `Z:\LightroomSync\Catalog` {
		t.Errorf("backup_folder = %q", cfg.BackupFolder)
	}
	if cfg.CatalogPath != `C:\Users\Username\Pictures\Lightroom` {
		t.Errorf("catalog_path = %q", cfg.CatalogPath)
	}
	if !cfg.StartWithWindows {
		t.Error("start_with_windows should be true")
	}
	if cfg.StartMinimized {
		t.Error("start_minimized should be false")
	}
	if !cfg.MinimizeToTray {
		t.Error("minimize_to_tray should be true")
	}
	if !cfg.AutoSync {
		t.Error("auto_sync should be true")
	}
	if cfg.HeartbeatInterval != 30 {
		t.Errorf("heartbeat_interval = %d", cfg.HeartbeatInterval)
	}
	if cfg.CheckInterval != 60 {
		t.Errorf("check_interval = %d", cfg.CheckInterval)
	}
	if cfg.LockTimeout != 120 {
		t.Errorf("lock_timeout = %d", cfg.LockTimeout)
	}
	if cfg.MaxCatalogBackups != 5 {
		t.Errorf("max_catalog_backups = %d", cfg.MaxCatalogBackups)
	}
	if !cfg.PresetSyncEnabled {
		t.Error("preset_sync_enabled should be true")
	}
	if len(cfg.PresetCategories) != 3 {
		t.Errorf("preset_categories len = %d, want 3", len(cfg.PresetCategories))
	}
	if cfg.LastSyncedTimestamp != "2026-03-29T15:42:30.123456" {
		t.Errorf("last_synced_timestamp = %q", cfg.LastSyncedTimestamp)
	}
}

// TestSave_Roundtrip verifies config survives save→load cycle.
func TestSave_Roundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_config.yaml")

	mgr := NewManager(path)
	cfg := mgr.Get()
	cfg.BackupFolder = `\\server\share\Lightroom`
	cfg.AutoSync = true
	cfg.PresetCategories = []string{"Develop Presets", "Watermarks"}

	if err := mgr.Update(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Reload
	mgr2 := NewManager(path)
	if err := mgr2.Load(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	got := mgr2.Get()
	if got.BackupFolder != `\\server\share\Lightroom` {
		t.Errorf("backup_folder roundtrip = %q", got.BackupFolder)
	}
	if !got.AutoSync {
		t.Error("auto_sync should survive roundtrip")
	}
	if len(got.PresetCategories) != 2 {
		t.Errorf("preset_categories len = %d", len(got.PresetCategories))
	}
}

// TestLoad_MissingFile_UsesDefaults verifies defaults when no config exists.
func TestLoad_MissingFile_UsesDefaults(t *testing.T) {
	mgr := NewManager(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err := mgr.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}

	cfg := mgr.Get()
	if cfg.MinimizeToTray != true {
		t.Error("default minimize_to_tray should be true")
	}
	if cfg.HeartbeatInterval != 30 {
		t.Errorf("default heartbeat = %d", cfg.HeartbeatInterval)
	}
}

// TestSave_CreatesDirectory verifies config dir is auto-created.
func TestSave_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "nested", "config.yaml")

	mgr := NewManager(path)
	if err := mgr.Save(); err != nil {
		t.Fatalf("save with nested dir: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("config file should exist: %v", err)
	}
}

func TestMigrateFromLegacyPaths_MigratesAndBacksUp(t *testing.T) {
	tmpDir := t.TempDir()

	targetPath := filepath.Join(tmpDir, "new", "config.yaml")
	legacyPath := filepath.Join(tmpDir, "legacy", "LightroomSyncConfig.yaml")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}

	legacyYAML := "" +
		"backup_folder: \"Z:\\\\Legacy\\\\Catalog\"\n" +
		"catalog_path: \"C:\\\\Legacy\\\\Lightroom\"\n" +
		"auto_sync: true\n"
	if err := os.WriteFile(legacyPath, []byte(legacyYAML), 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	mgr := NewManager(targetPath)
	migrated, sourcePath, err := mgr.MigrateFromLegacyPaths([]string{legacyPath})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !migrated {
		t.Fatal("expected migrated=true")
	}
	if sourcePath != legacyPath {
		t.Fatalf("sourcePath = %q, want %q", sourcePath, legacyPath)
	}

	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("target config should exist after migration: %v", err)
	}

	cfg := mgr.Get()
	if cfg.BackupFolder != `Z:\Legacy\Catalog` {
		t.Fatalf("backup_folder = %q", cfg.BackupFolder)
	}
	if !cfg.AutoSync {
		t.Fatal("auto_sync should be migrated as true")
	}

	backupGlob := filepath.Join(filepath.Dir(targetPath), "legacy_backup_*_LightroomSyncConfig.yaml")
	backups, err := filepath.Glob(backupGlob)
	if err != nil {
		t.Fatalf("glob legacy backups: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected 1 legacy backup file, got %d", len(backups))
	}
}

func TestMigrateFromLegacyPaths_SkipsWhenTargetExists(t *testing.T) {
	tmpDir := t.TempDir()

	targetPath := filepath.Join(tmpDir, "new", "config.yaml")
	legacyPath := filepath.Join(tmpDir, "legacy", "LightroomSyncConfig.yaml")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}

	if err := os.WriteFile(legacyPath, []byte("backup_folder: \"Z:\\\\Legacy\\\\Catalog\"\n"), 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	mgr := NewManager(targetPath)
	cfg := mgr.Get()
	cfg.BackupFolder = `C:\Current\Config`
	if err := mgr.Update(cfg); err != nil {
		t.Fatalf("write target config: %v", err)
	}

	migrated, _, err := mgr.MigrateFromLegacyPaths([]string{legacyPath})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if migrated {
		t.Fatal("expected migrated=false when target config already exists")
	}

	got := mgr.Get()
	if !strings.EqualFold(got.BackupFolder, `C:\Current\Config`) {
		t.Fatalf("target config should be preserved, got %q", got.BackupFolder)
	}
}
