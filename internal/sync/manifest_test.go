package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestReadManifest_PythonFixture verifies Go can parse Python-generated manifest.
func TestReadManifest_PythonFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "sync_manifest.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var raw manifestJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Python writes "zip_file" key
	if raw.ZipFile != "2026-03-29/catalog_backup_20260329_154230.zip" {
		t.Errorf("zip_file = %q, want %q", raw.ZipFile, "2026-03-29/catalog_backup_20260329_154230.zip")
	}
	if raw.Machine != "DESKTOP-ABC123" {
		t.Errorf("machine = %q, want %q", raw.Machine, "DESKTOP-ABC123")
	}
	if raw.Timestamp != "2026-03-29T15:42:30.123456" {
		t.Errorf("timestamp = %q", raw.Timestamp)
	}
	if raw.ZipSize != 1048576000 {
		t.Errorf("zip_size = %d, want %d", raw.ZipSize, 1048576000)
	}
}

// TestManifest_Roundtrip verifies Go-written manifest can be re-parsed.
func TestManifest_Roundtrip(t *testing.T) {
	original := Manifest{
		Machine:   "MY-PC",
		Timestamp: "2026-01-15T10:30:00.000000",
		ZipFile:   "2026-01-15/backup.zip",
		ZipSize:   500000000,
	}

	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed Manifest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed.Machine != original.Machine {
		t.Errorf("machine roundtrip: got %q, want %q", parsed.Machine, original.Machine)
	}
	if parsed.ZipFile != original.ZipFile {
		t.Errorf("zip_file roundtrip: got %q, want %q", parsed.ZipFile, original.ZipFile)
	}
	if parsed.ZipSize != original.ZipSize {
		t.Errorf("zip_size roundtrip: got %d, want %d", parsed.ZipSize, original.ZipSize)
	}
}

// TestManifest_WritesZipFile verifies Go writer uses "zip_file" key (not "zip_path").
func TestManifest_WritesZipFile(t *testing.T) {
	m := Manifest{
		Machine:   "TEST",
		Timestamp: "2026-01-01T00:00:00",
		ZipFile:   "test/backup.zip",
		ZipSize:   100,
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	json.Unmarshal(data, &raw)

	if _, ok := raw["zip_file"]; !ok {
		t.Error("writer must use 'zip_file' key for Python compatibility")
	}
	if _, ok := raw["zip_path"]; ok {
		t.Error("writer must NOT include 'zip_path' key")
	}
}

// TestManifest_ReaderAcceptsZipPath verifies reader handles future "zip_path" key.
func TestManifest_ReaderAcceptsZipPath(t *testing.T) {
	jsonData := `{"machine":"TEST","timestamp":"2026-01-01T00:00:00","zip_path":"test/backup.zip","zip_size":100}`

	var raw manifestJSON
	if err := json.Unmarshal([]byte(jsonData), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	zipFile := raw.ZipFile
	if zipFile == "" {
		zipFile = raw.ZipPath
	}

	if zipFile != "test/backup.zip" {
		t.Errorf("reader should fall back to zip_path when zip_file is empty, got %q", zipFile)
	}
}

// TestShouldSyncFromNetwork_SelfBackup tests Rule 1.
func TestShouldSyncFromNetwork_SelfBackup(t *testing.T) {
	m := &Manifest{Machine: "MY-PC", Timestamp: "2026-01-01T00:00:00", ZipFile: "test.zip"}
	ok, reason := ShouldSyncFromNetwork(m, "MY-PC", "", "")
	if ok {
		t.Error("should skip self backup")
	}
	if reason != SkipSelfBackup {
		t.Errorf("reason = %q, want %q", reason, SkipSelfBackup)
	}
}

// TestShouldSyncFromNetwork_AlreadySynced tests Rule 2.
func TestShouldSyncFromNetwork_AlreadySynced(t *testing.T) {
	m := &Manifest{Machine: "OTHER-PC", Timestamp: "2026-01-01T00:00:00", ZipFile: "test.zip"}
	ok, reason := ShouldSyncFromNetwork(m, "MY-PC", "2026-01-01T00:00:00", "")
	if ok {
		t.Error("should skip already synced")
	}
	if reason != SkipAlreadySynced {
		t.Errorf("reason = %q, want %q", reason, SkipAlreadySynced)
	}
}
