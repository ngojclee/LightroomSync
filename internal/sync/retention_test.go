package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreatePreSyncBackup_CreatesZipAndAppliesRetention(t *testing.T) {
	root := t.TempDir()
	catalogDir := filepath.Join(root, "catalog")
	if err := os.MkdirAll(catalogDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(catalogDir, "Main Catalog.lrcat"), []byte("catalog"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(catalogDir, "Main Catalog Previews.lrdata"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(catalogDir, "Main Catalog Previews.lrdata", "p.txt"), []byte("preview"), 0o644); err != nil {
		t.Fatal(err)
	}

	created1, err := CreatePreSyncBackup(context.Background(), catalogDir, "PC-1", 1, time.Date(2026, 3, 30, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("CreatePreSyncBackup #1 failed: %v", err)
	}
	if created1 == "" {
		t.Fatal("expected first pre-sync zip path")
	}
	if _, err := os.Stat(created1); err != nil {
		t.Fatalf("expected first pre-sync zip exists: %v", err)
	}

	created2, err := CreatePreSyncBackup(context.Background(), catalogDir, "PC-1", 1, time.Date(2026, 3, 30, 1, 0, 1, 0, time.UTC))
	if err != nil {
		t.Fatalf("CreatePreSyncBackup #2 failed: %v", err)
	}
	if created2 == "" {
		t.Fatal("expected second pre-sync zip path")
	}

	if _, err := os.Stat(created1); !os.IsNotExist(err) {
		t.Fatalf("first zip should be removed by retention, stat err=%v", err)
	}
	if _, err := os.Stat(created2); err != nil {
		t.Fatalf("second zip should remain: %v", err)
	}
}

func TestCreatePreSyncBackup_NoArtifactsReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	catalogDir := filepath.Join(root, "catalog")
	if err := os.MkdirAll(catalogDir, 0o755); err != nil {
		t.Fatal(err)
	}

	created, err := CreatePreSyncBackup(context.Background(), catalogDir, "PC-1", 3, time.Now())
	if err != nil {
		t.Fatalf("CreatePreSyncBackup failed: %v", err)
	}
	if created != "" {
		t.Fatalf("created path = %q, want empty when no artifacts", created)
	}
}

func TestCleanupZipRetention_RemovesOldest(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	zipA := filepath.Join(root, "a.zip")
	zipB := filepath.Join(root, "b.zip")
	zipC := filepath.Join(root, "c.zip")
	for _, path := range []string{zipA, zipB, zipC} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tA := time.Date(2026, 3, 30, 1, 0, 0, 0, time.UTC)
	tB := time.Date(2026, 3, 30, 1, 0, 1, 0, time.UTC)
	tC := time.Date(2026, 3, 30, 1, 0, 2, 0, time.UTC)
	_ = os.Chtimes(zipA, tA, tA)
	_ = os.Chtimes(zipB, tB, tB)
	_ = os.Chtimes(zipC, tC, tC)

	removed, err := CleanupZipRetention(root, 2)
	if err != nil {
		t.Fatalf("CleanupZipRetention failed: %v", err)
	}
	if len(removed) != 1 {
		t.Fatalf("removed count = %d, want 1", len(removed))
	}
	if removed[0] != zipA {
		t.Fatalf("removed[0] = %s, want %s", removed[0], zipA)
	}
}
