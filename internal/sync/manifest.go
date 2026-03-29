package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const manifestFileName = "sync_manifest.json"

// Manifest represents the sync_manifest.json on the network.
// Wire format: JSON with keys machine, timestamp, zip_file, zip_size.
type Manifest struct {
	Machine   string `json:"machine"`
	Timestamp string `json:"timestamp"`
	ZipFile   string `json:"zip_file"` // relative path, forward slashes
	ZipSize   int64  `json:"zip_size"`
}

// manifestJSON is the raw JSON representation for flexible parsing.
// Reader accepts both "zip_file" (legacy Python) and "zip_path" (future).
type manifestJSON struct {
	Machine   string `json:"machine"`
	Timestamp string `json:"timestamp"`
	ZipFile   string `json:"zip_file"`
	ZipPath   string `json:"zip_path"`
	ZipSize   int64  `json:"zip_size"`
}

// ManifestManager handles reading/writing the sync manifest.
type ManifestManager struct {
	catalogDir string // network path e.g. \\server\share\Catalog
}

func NewManifestManager(catalogDir string) *ManifestManager {
	return &ManifestManager{catalogDir: catalogDir}
}

func (m *ManifestManager) ManifestPath() string {
	return filepath.Join(m.catalogDir, manifestFileName)
}

// ReadManifest reads and parses the manifest file.
// Returns nil if file doesn't exist.
func (m *ManifestManager) ReadManifest(ctx context.Context) (*Manifest, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(m.ManifestPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var raw manifestJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	// Accept both zip_file (Python legacy) and zip_path (future).
	zipFile := raw.ZipFile
	if zipFile == "" {
		zipFile = raw.ZipPath
	}

	return &Manifest{
		Machine:   raw.Machine,
		Timestamp: raw.Timestamp,
		ZipFile:   zipFile,
		ZipSize:   raw.ZipSize,
	}, nil
}

// WriteManifest writes the manifest to disk.
// Writer always uses "zip_file" key for backward compatibility with Python.
func (m *ManifestManager) WriteManifest(ctx context.Context, manifest Manifest) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	dir := filepath.Dir(m.ManifestPath())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create manifest dir: %w", err)
	}

	return os.WriteFile(m.ManifestPath(), data, 0o644)
}

// SkipReason describes why a sync was skipped.
type SkipReason string

const (
	SkipSelfBackup    SkipReason = "self_backup"     // manifest.machine == local machine
	SkipAlreadySynced SkipReason = "already_synced"   // last_synced >= manifest.timestamp
	SkipZipMissing    SkipReason = "zip_missing"      // zip file doesn't exist
	SkipZipSizeBad    SkipReason = "zip_size_mismatch" // zip size doesn't match
)

// ShouldSyncFromNetwork implements the 3-rule anti-self-sync engine.
// Returns (shouldSync, skipReason).
func ShouldSyncFromNetwork(manifest *Manifest, localMachine string, lastSyncedTimestamp string, backupDir string) (bool, SkipReason) {
	if manifest == nil {
		return false, "no_manifest"
	}

	// Rule 1: Skip if manifest was written by this machine
	if strings.EqualFold(manifest.Machine, localMachine) {
		return false, SkipSelfBackup
	}

	// Rule 2: Skip if already synced (timestamp comparison)
	if lastSyncedTimestamp != "" && lastSyncedTimestamp >= manifest.Timestamp {
		return false, SkipAlreadySynced
	}

	// Rule 3: Verify zip exists and size matches
	zipPath := filepath.Join(backupDir, filepath.FromSlash(manifest.ZipFile))
	info, err := os.Stat(zipPath)
	if err != nil {
		return false, SkipZipMissing
	}
	if manifest.ZipSize > 0 && info.Size() != manifest.ZipSize {
		return false, SkipZipSizeBad
	}

	return true, ""
}

// NewManifestForBackup creates a manifest entry for a local backup.
func NewManifestForBackup(machine string, zipPath string, backupDir string) (Manifest, error) {
	rel, err := filepath.Rel(backupDir, zipPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("compute relative path: %w", err)
	}

	info, err := os.Stat(zipPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("stat zip: %w", err)
	}

	return Manifest{
		Machine:   machine,
		Timestamp: time.Now().Format("2006-01-02T15:04:05.999999"),
		ZipFile:   strings.ReplaceAll(rel, "\\", "/"),
		ZipSize:   info.Size(),
	}, nil
}
