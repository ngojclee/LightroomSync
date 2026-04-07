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
)

// ProgressCallback is called with (current, total) file count during extraction.
// Return false to abort the operation.
type ProgressCallback func(current, total int) bool

// RestoreOptions controls catalog restore behavior from backup zip.
type RestoreOptions struct {
	// CleanupPatterns are glob patterns (relative to destination dir) removed before extraction.
	CleanupPatterns []string
	// FlattenSingleRoot strips the top-level folder if zip is wrapped in a single root directory.
	FlattenSingleRoot bool
	// Progress, if provided, is called during extraction with file progress.
	Progress ProgressCallback
}

// DefaultRestoreOptions returns safe defaults for Lightroom catalog restore.
func DefaultRestoreOptions() RestoreOptions {
	return RestoreOptions{
		CleanupPatterns: []string{
			"*.lrcat",
			"*.lrcat-data",
			"*.lrcat-journal",
			"*.lrcat.lock",
			"*.lrdata",
			"lightroom_lock.txt",
			"*-lock",
			"*-shm",
			"*-wal",
		},
		FlattenSingleRoot: true,
	}
}

// ValidateZipIntegrity verifies the zip can be opened and entries can be read.
func ValidateZipIntegrity(ctx context.Context, zipPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s: %w", file.Name, err)
		}
		if _, err := io.Copy(io.Discard, rc); err != nil {
			rc.Close()
			return fmt.Errorf("read zip entry %s: %w", file.Name, err)
		}
		if err := rc.Close(); err != nil {
			return fmt.Errorf("close zip entry %s: %w", file.Name, err)
		}
	}
	return nil
}

// CleanupCatalogArtifacts removes old catalog artifacts before extracting a new backup.
func CleanupCatalogArtifacts(ctx context.Context, destDir string, patterns []string) error {
	matches, err := collectCleanupMatches(ctx, destDir, patterns)
	if err != nil {
		return err
	}
	for _, match := range matches {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := os.RemoveAll(match); err != nil {
			return fmt.Errorf("remove artifact %s: %w", match, err)
		}
	}
	return nil
}

// RestoreCatalogFromZip validates, cleans, and extracts backup zip safely.
func RestoreCatalogFromZip(ctx context.Context, zipPath string, destDir string, options RestoreOptions) error {
	if options.CleanupPatterns == nil {
		options = DefaultRestoreOptions()
	}

	if err := ValidateZipIntegrity(ctx, zipPath); err != nil {
		return err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create destination dir: %w", err)
	}
	if err := CleanupCatalogArtifacts(ctx, destDir, options.CleanupPatterns); err != nil {
		return err
	}
	if err := ExtractZipSafe(ctx, zipPath, destDir, options.FlattenSingleRoot, options.Progress); err != nil {
		return err
	}
	if err := CleanupCatalogArtifacts(ctx, destDir, transientCleanupPatterns(options.CleanupPatterns)); err != nil {
		return err
	}
	return nil
}

// ExtractZipSafe extracts a zip file with zip-slip protection.
func ExtractZipSafe(ctx context.Context, zipPath, destDir string, flattenSingleRoot bool, progress ProgressCallback) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()

	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("resolve destination path: %w", err)
	}

	rootPrefix, hasSingleRoot := detectSingleRootPrefix(reader.File)

	// Count extractable files (skip directories)
	totalFiles := 0
	for _, file := range reader.File {
		if !file.FileInfo().IsDir() {
			totalFiles++
		}
	}

	currentFile := 0
	for _, file := range reader.File {
		if err := ctx.Err(); err != nil {
			return err
		}

		relative := normalizeZipPath(file.Name)
		if relative == "" {
			continue
		}
		if flattenSingleRoot && hasSingleRoot {
			relative = stripRootPrefix(relative, rootPrefix)
			if relative == "" {
				continue
			}
		}
		if isUnsafeRelativePath(relative) {
			return fmt.Errorf("zip-slip blocked for entry: %s", file.Name)
		}

		targetPath := filepath.Join(destDir, filepath.FromSlash(relative))
		targetAbs, err := filepath.Abs(targetPath)
		if err != nil {
			return fmt.Errorf("resolve target path for entry %s: %w", file.Name, err)
		}
		if !isWithinBase(targetAbs, destAbs) {
			return fmt.Errorf("zip-slip blocked for entry: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetAbs, 0o755); err != nil {
				return fmt.Errorf("create directory %s: %w", targetAbs, err)
			}
			if err := ensureWritablePath(targetAbs, true, file.Mode()); err != nil {
				return fmt.Errorf("normalize directory attributes %s: %w", targetAbs, err)
			}
			continue
		}

		currentFile++
		if progress != nil && !progress(currentFile, totalFiles) {
			return ctx.Err()
		}

		if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
			return fmt.Errorf("create parent directory for %s: %w", targetAbs, err)
		}

		src, err := file.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s: %w", file.Name, err)
		}

		dst, err := os.OpenFile(targetAbs, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, writableMode(file.Mode(), false))
		if err != nil {
			src.Close()
			return fmt.Errorf("create extracted file %s: %w", targetAbs, err)
		}

		_, copyErr := io.Copy(dst, src)
		closeDstErr := dst.Close()
		closeSrcErr := src.Close()

		if copyErr != nil {
			return fmt.Errorf("copy entry %s: %w", file.Name, copyErr)
		}
		if closeDstErr != nil {
			return fmt.Errorf("close extracted file %s: %w", targetAbs, closeDstErr)
		}
		if closeSrcErr != nil {
			return fmt.Errorf("close zip entry %s: %w", file.Name, closeSrcErr)
		}
		if err := ensureWritablePath(targetAbs, false, file.Mode()); err != nil {
			return fmt.Errorf("normalize file attributes %s: %w", targetAbs, err)
		}
	}

	return nil
}

func detectSingleRootPrefix(files []*zip.File) (string, bool) {
	roots := map[string]struct{}{}
	hasRootLevelFile := false

	for _, file := range files {
		normalized := normalizeZipPath(file.Name)
		if normalized == "" {
			continue
		}
		parts := strings.Split(normalized, "/")
		if len(parts) == 1 {
			// File at root means no wrapper folder layout.
			if !file.FileInfo().IsDir() {
				hasRootLevelFile = true
				break
			}
			continue
		}
		roots[parts[0]] = struct{}{}
		if len(roots) > 1 {
			return "", false
		}
	}

	if hasRootLevelFile || len(roots) != 1 {
		return "", false
	}

	for root := range roots {
		return root + "/", true
	}
	return "", false
}

func normalizeZipPath(name string) string {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimPrefix(name, "./")
	return strings.TrimSpace(name)
}

func stripRootPrefix(path, rootPrefix string) string {
	if strings.HasPrefix(path, rootPrefix) {
		return strings.TrimPrefix(path, rootPrefix)
	}
	return path
}

func isUnsafeRelativePath(path string) bool {
	clean := filepath.Clean(filepath.FromSlash(path))
	return clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator))
}

func isWithinBase(targetAbs, baseAbs string) bool {
	if targetAbs == baseAbs {
		return true
	}
	baseWithSep := baseAbs + string(os.PathSeparator)
	return strings.HasPrefix(targetAbs, baseWithSep)
}

func collectCleanupMatches(ctx context.Context, destDir string, patterns []string) ([]string, error) {
	collected := make(map[string]struct{})
	err := filepath.WalkDir(destDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == destDir {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		relPath, err := filepath.Rel(destDir, path)
		if err != nil {
			return fmt.Errorf("relative cleanup path %s: %w", path, err)
		}
		relPath = filepath.ToSlash(relPath)

		for _, pattern := range patterns {
			matched, err := cleanupPatternMatches(pattern, relPath, d.Name())
			if err != nil {
				return fmt.Errorf("match cleanup pattern %s: %w", pattern, err)
			}
			if matched {
				collected[path] = struct{}{}
				return nil
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(collected))
	for path := range collected {
		out = append(out, path)
	}
	sort.Slice(out, func(i, j int) bool {
		depthI := strings.Count(filepath.ToSlash(out[i]), "/")
		depthJ := strings.Count(filepath.ToSlash(out[j]), "/")
		if depthI != depthJ {
			return depthI > depthJ
		}
		return out[i] > out[j]
	})
	return out, nil
}

func cleanupPatternMatches(pattern, relPath, baseName string) (bool, error) {
	target := baseName
	if strings.Contains(pattern, "/") || strings.Contains(pattern, "\\") {
		target = relPath
	}

	matched, err := filepath.Match(pattern, target)
	if err != nil {
		return false, err
	}
	if matched {
		return true, nil
	}

	lowerPattern := strings.ToLower(filepath.ToSlash(pattern))
	lowerTarget := strings.ToLower(filepath.ToSlash(target))
	return filepath.Match(lowerPattern, lowerTarget)
}

func transientCleanupPatterns(patterns []string) []string {
	if len(patterns) == 0 {
		return nil
	}

	out := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		switch strings.ToLower(strings.TrimSpace(pattern)) {
		case "*.lrcat.lock", "*.lrcat-journal", "lightroom_lock.txt", "*-lock", "*-shm", "*-wal":
			out = append(out, pattern)
		}
	}
	return out
}

func ensureWritablePath(path string, isDir bool, original os.FileMode) error {
	return os.Chmod(path, writableMode(original, isDir))
}

func writableMode(mode os.FileMode, isDir bool) os.FileMode {
	perm := mode.Perm()
	if isDir {
		if perm == 0 {
			perm = 0o755
		}
		perm |= 0o700
		return perm
	}
	if perm == 0 {
		perm = 0o644
	}
	perm |= 0o600
	return perm
}
