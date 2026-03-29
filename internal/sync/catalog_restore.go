package sync

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// RestoreOptions controls catalog restore behavior from backup zip.
type RestoreOptions struct {
	// CleanupPatterns are glob patterns (relative to destination dir) removed before extraction.
	CleanupPatterns []string
	// FlattenSingleRoot strips the top-level folder if zip is wrapped in a single root directory.
	FlattenSingleRoot bool
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
	for _, pattern := range patterns {
		if err := ctx.Err(); err != nil {
			return err
		}
		patternPath := filepath.Join(destDir, pattern)
		matches, err := filepath.Glob(patternPath)
		if err != nil {
			return fmt.Errorf("glob pattern %s: %w", pattern, err)
		}
		for _, match := range matches {
			if err := os.RemoveAll(match); err != nil {
				return fmt.Errorf("remove artifact %s: %w", match, err)
			}
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
	if err := ExtractZipSafe(ctx, zipPath, destDir, options.FlattenSingleRoot); err != nil {
		return err
	}
	return nil
}

// ExtractZipSafe extracts a zip file with zip-slip protection.
func ExtractZipSafe(ctx context.Context, zipPath, destDir string, flattenSingleRoot bool) error {
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
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
			return fmt.Errorf("create parent directory for %s: %w", targetAbs, err)
		}

		src, err := file.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s: %w", file.Name, err)
		}

		dst, err := os.OpenFile(targetAbs, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
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
