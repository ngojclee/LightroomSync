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

	err = ExtractZipSafe(context.Background(), zipPath, dest, false)
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
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, content := range files {
		h := &zip.FileHeader{
			Name:   name,
			Method: zip.Deflate,
		}
		h.SetModTime(time.Now().UTC())
		w, err := zw.CreateHeader(h)
		if err != nil {
			zw.Close()
			return err
		}
		if _, err := w.Write([]byte(content)); err != nil {
			zw.Close()
			return err
		}
	}
	return zw.Close()
}
