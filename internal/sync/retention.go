package sync

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const preSyncBackupFolderName = "PreSyncBackups"

var catalogArtifactPatterns = []string{
	"*.lrcat",
	"*.lrcat-data",
	"*.lrcat-journal",
	"*.lrcat.lock",
	"*.lrdata",
}

type zipFileInfo struct {
	Path    string
	ModTime time.Time
}

// CreatePreSyncBackup snapshots current catalog artifacts into a zip archive.
// Returns created zip path, or empty path when no catalog artifacts exist.
func CreatePreSyncBackup(ctx context.Context, catalogDir, machine string, maxBackups int, now time.Time) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if catalogDir == "" {
		return "", fmt.Errorf("catalog directory is empty")
	}
	if machine == "" {
		machine = "UNKNOWN"
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	preSyncDir := filepath.Join(catalogDir, preSyncBackupFolderName)
	if err := os.MkdirAll(preSyncDir, 0o755); err != nil {
		return "", fmt.Errorf("create pre-sync directory: %w", err)
	}

	entries, err := discoverCatalogArtifacts(catalogDir)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", nil
	}

	zipName := fmt.Sprintf("presync_%s_%s.zip", sanitizeFileName(machine), now.Format("20060102_150405"))
	zipPath := filepath.Join(preSyncDir, zipName)

	if err := zipSelectedEntries(ctx, catalogDir, entries, zipPath); err != nil {
		return "", err
	}

	_, err = CleanupZipRetention(preSyncDir, maxBackups)
	if err != nil {
		return "", err
	}

	return zipPath, nil
}

// CleanupZipRetention removes oldest zip files until count <= maxBackups.
func CleanupZipRetention(rootDir string, maxBackups int) ([]string, error) {
	if maxBackups <= 0 {
		maxBackups = 1
	}
	if rootDir == "" {
		return nil, nil
	}
	if _, err := os.Stat(rootDir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	files := make([]zipFileInfo, 0, 32)
	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".zip") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files = append(files, zipFileInfo{
			Path:    path,
			ModTime: info.ModTime().UTC(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(files) <= maxBackups {
		return nil, nil
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].ModTime.Equal(files[j].ModTime) {
			return files[i].Path > files[j].Path
		}
		return files[i].ModTime.After(files[j].ModTime)
	})

	toDelete := files[maxBackups:]
	removed := make([]string, 0, len(toDelete))
	for _, file := range toDelete {
		if err := os.Remove(file.Path); err != nil {
			return removed, fmt.Errorf("remove old zip %s: %w", file.Path, err)
		}
		removed = append(removed, file.Path)
	}
	return removed, nil
}

func discoverCatalogArtifacts(catalogDir string) ([]string, error) {
	collected := map[string]struct{}{}
	for _, pattern := range catalogArtifactPatterns {
		matches, err := filepath.Glob(filepath.Join(catalogDir, pattern))
		if err != nil {
			return nil, fmt.Errorf("glob catalog artifacts (%s): %w", pattern, err)
		}
		for _, match := range matches {
			collected[match] = struct{}{}
		}
	}

	out := make([]string, 0, len(collected))
	for path := range collected {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

func zipSelectedEntries(ctx context.Context, baseDir string, entries []string, zipPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	file, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("create pre-sync zip %s: %w", zipPath, err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer zw.Close()

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := os.Stat(entry)
		if err != nil {
			return fmt.Errorf("stat artifact %s: %w", entry, err)
		}
		if info.IsDir() {
			err = filepath.WalkDir(entry, func(path string, d os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if d.IsDir() {
					return nil
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				return addFileToZip(baseDir, path, zw)
			})
			if err != nil {
				return err
			}
			continue
		}

		if err := addFileToZip(baseDir, entry, zw); err != nil {
			return err
		}
	}
	return zw.Close()
}

func addFileToZip(baseDir, srcPath string, zw *zip.Writer) error {
	relPath, err := filepath.Rel(baseDir, srcPath)
	if err != nil {
		return fmt.Errorf("relative artifact path for zip: %w", err)
	}

	info, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("stat file %s: %w", srcPath, err)
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("zip header for %s: %w", srcPath, err)
	}
	header.Name = strings.ReplaceAll(relPath, "\\", "/")
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create zip entry %s: %w", srcPath, err)
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source %s: %w", srcPath, err)
	}
	defer src.Close()

	if _, err := io.Copy(writer, src); err != nil {
		return fmt.Errorf("copy source %s to zip: %w", srcPath, err)
	}
	return nil
}

func sanitizeFileName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "UNKNOWN"
	}
	replacer := strings.NewReplacer(
		"\\", "_",
		"/", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)
	return replacer.Replace(raw)
}
