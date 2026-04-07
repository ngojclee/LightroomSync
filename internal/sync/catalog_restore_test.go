package sync

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRestoreCatalogFromZip_FlattensSingleRootAndCleansOldArtifacts(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "backup.zip")
	dest := filepath.Join(root, "catalog")

	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	oldCatalog := filepath.Join(dest, "Old Catalog.lrcat")
	if err := os.WriteFile(oldCatalog, []byte("old"), 0o644); err != nil {
		t.Fatalf("write old catalog: %v", err)
	}

	err := createZip(zipPath, map[string]string{
		"Wrapper/New Catalog.lrcat":                      "new-catalog",
		"Wrapper/New Catalog Previews.lrdata/marker.txt": "preview-data",
	})
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}

	if err := RestoreCatalogFromZip(context.Background(), zipPath, dest, DefaultRestoreOptions()); err != nil {
		t.Fatalf("restore catalog: %v", err)
	}

	if _, err := os.Stat(oldCatalog); !os.IsNotExist(err) {
		t.Fatalf("old catalog should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "New Catalog.lrcat")); err != nil {
		t.Fatalf("new catalog missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "New Catalog Previews.lrdata", "marker.txt")); err != nil {
		t.Fatalf("flattened preview dir missing: %v", err)
	}
}

func TestRestoreCatalogFromZip_CleansNestedLockArtifactsBeforeAndAfterExtract(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "backup.zip")
	dest := filepath.Join(root, "catalog")

	nestedLockDir := filepath.Join(dest, "New Catalog Helper.lrdata", "sync")
	if err := os.MkdirAll(nestedLockDir, 0o755); err != nil {
		t.Fatalf("mkdir nested lock dir: %v", err)
	}
	staleWal := filepath.Join(nestedLockDir, "helper-wal")
	if err := os.WriteFile(staleWal, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale wal: %v", err)
	}

	err := createZip(zipPath, map[string]string{
		"Wrapper/New Catalog.lrcat":                            "new-catalog",
		"Wrapper/New Catalog Helper.lrdata/data.bin":           "helper-data",
		"Wrapper/New Catalog Helper.lrdata/cache/catalog-shm":  "zip-shm",
		"Wrapper/New Catalog Helper.lrdata/cache/catalog-lock": "zip-lock",
		"Wrapper/New Catalog Helper.lrdata/cache/catalog-wal":  "zip-wal",
		"Wrapper/New Catalog Smart Previews.lrdata/marker.txt": "preview-data",
	})
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}

	if err := RestoreCatalogFromZip(context.Background(), zipPath, dest, DefaultRestoreOptions()); err != nil {
		t.Fatalf("restore catalog: %v", err)
	}

	for _, unwanted := range []string{
		staleWal,
		filepath.Join(dest, "New Catalog Helper.lrdata", "cache", "catalog-shm"),
		filepath.Join(dest, "New Catalog Helper.lrdata", "cache", "catalog-lock"),
		filepath.Join(dest, "New Catalog Helper.lrdata", "cache", "catalog-wal"),
	} {
		if _, err := os.Stat(unwanted); !os.IsNotExist(err) {
			t.Fatalf("expected cleanup to remove %s, stat err=%v", unwanted, err)
		}
	}

	if _, err := os.Stat(filepath.Join(dest, "New Catalog.lrcat")); err != nil {
		t.Fatalf("restored catalog missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "New Catalog Helper.lrdata", "data.bin")); err != nil {
		t.Fatalf("helper data missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "New Catalog Smart Previews.lrdata", "marker.txt")); err != nil {
		t.Fatalf("preview data missing: %v", err)
	}
}

func TestRestoreCatalogFromZip_MakesCatalogWritableEvenIfZipEntryIsReadOnly(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "backup.zip")
	dest := filepath.Join(root, "catalog")

	err := createZipWithModes(zipPath, map[string]zipFixture{
		"Wrapper/ReadOnly Catalog.lrcat": {
			Content: "catalog",
			Mode:    0o444,
		},
		"Wrapper/ReadOnly Catalog Previews.lrdata/marker.txt": {
			Content: "preview",
			Mode:    0o444,
		},
	})
	if err != nil {
		t.Fatalf("create zip with modes: %v", err)
	}

	if err := RestoreCatalogFromZip(context.Background(), zipPath, dest, DefaultRestoreOptions()); err != nil {
		t.Fatalf("restore catalog: %v", err)
	}

	catalogPath := filepath.Join(dest, "ReadOnly Catalog.lrcat")
	info, err := os.Stat(catalogPath)
	if err != nil {
		t.Fatalf("stat restored catalog: %v", err)
	}
	if info.Mode().Perm()&0o200 == 0 {
		t.Fatalf("restored catalog must be writable, mode=%#o", info.Mode().Perm())
	}

	f, err := os.OpenFile(catalogPath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("restored catalog should be openable for write: %v", err)
	}
	_ = f.Close()
}

func TestExtractZipSafe_BlocksZipSlip(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "evil.zip")
	dest := filepath.Join(root, "dest")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}

	err := createZip(zipPath, map[string]string{
		"../outside.txt": "blocked",
	})
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}

	err = ExtractZipSafe(context.Background(), zipPath, dest, false, nil)
	if err == nil {
		t.Fatal("expected zip-slip error")
	}
}

func TestValidateZipIntegrity_DetectsCorruptedZip(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "bad.zip")
	if err := os.WriteFile(zipPath, []byte("not-a-zip"), 0o644); err != nil {
		t.Fatalf("write bad zip: %v", err)
	}

	err := ValidateZipIntegrity(context.Background(), zipPath)
	if err == nil {
		t.Fatal("expected integrity check to fail")
	}
}

func TestRestoreCatalogFromZip_RespectsContextCancel(t *testing.T) {
	root := t.TempDir()
	zipPath := filepath.Join(root, "backup.zip")
	dest := filepath.Join(root, "dest")
	if err := createZip(zipPath, map[string]string{"Catalog.lrcat": "x"}); err != nil {
		t.Fatalf("create zip: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := RestoreCatalogFromZip(ctx, zipPath, dest, DefaultRestoreOptions())
	if err == nil {
		t.Fatal("expected canceled context error")
	}
}

func createZip(path string, files map[string]string) error {
	fixtures := make(map[string]zipFixture, len(files))
	for name, content := range files {
		fixtures[name] = zipFixture{Content: content, Mode: 0o644}
	}
	return createZipWithModes(path, fixtures)
}

type zipFixture struct {
	Content string
	Mode    os.FileMode
}

func createZipWithModes(path string, files map[string]zipFixture) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, fixture := range files {
		h := &zip.FileHeader{
			Name:   name,
			Method: zip.Deflate,
		}
		h.SetModTime(time.Now().UTC())
		if fixture.Mode != 0 {
			h.SetMode(fixture.Mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			zw.Close()
			return err
		}
		if _, err := w.Write([]byte(fixture.Content)); err != nil {
			zw.Close()
			return err
		}
	}
	return zw.Close()
}
