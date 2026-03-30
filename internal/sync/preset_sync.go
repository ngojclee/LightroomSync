package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	presetRootFolderName       = "Presets"
	presetLogoFolderName       = "Logos"
	defaultPresetSyncTolerance = 2 * time.Second
)

var (
	defaultPresetCategories = []string{
		"Export Presets",
		"Develop Presets",
		"Watermarks",
		"Metadata Presets",
		"Filename Templates",
	}
	excludedPresetDirs = map[string]struct{}{
		"preferences":       {},
		".lightroom-sync":   {},
		"lightroom presets": {},
	}
	watermarkImagePathPattern = regexp.MustCompile(`imagePath\s*=\s*"([^"]+)"`)
)

// PresetSyncOptions configures preset synchronization behavior.
type PresetSyncOptions struct {
	BackupDir         string
	LocalLightroomDir string
	Categories        []string
	StatePath         string
	MTimeTolerance    time.Duration
	Logf              func(format string, args ...any)
}

// PresetSyncSummary contains sync counters for logs/UI.
type PresetSyncSummary struct {
	Pulled      int
	Pushed      int
	Deleted     int
	LogosCopied int
	Tracked     int
}

// PresetSyncManager provides two-way preset synchronization with deletion-aware state.
type PresetSyncManager struct {
	backupDir  string
	localRoot  string
	statePath  string
	categories []string
	tolerance  time.Duration
	logf       func(format string, args ...any)
}

type presetFileMeta struct {
	Path  string
	MTime float64
}

// DefaultLightroomPresetRoot returns %APPDATA%\Adobe\Lightroom.
func DefaultLightroomPresetRoot() (string, error) {
	appData := strings.TrimSpace(os.Getenv("APPDATA"))
	if appData == "" {
		return "", errors.New("APPDATA environment variable not set")
	}
	return filepath.Join(appData, "Adobe", "Lightroom"), nil
}

// DefaultPresetStatePath returns the state file path used by the Python implementation.
func DefaultPresetStatePath(lightroomRoot string) string {
	return filepath.Join(lightroomRoot, ".lightroom-sync", "preset_state.json")
}

// DiscoverPresetCategories scans Lightroom local folders and returns category candidates.
func DiscoverPresetCategories(lightroomRoot string) ([]string, error) {
	defaults := append([]string(nil), defaultPresetCategories...)
	if strings.TrimSpace(lightroomRoot) == "" {
		return defaults, nil
	}

	entries, err := os.ReadDir(lightroomRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return defaults, nil
		}
		return nil, fmt.Errorf("read local preset root: %w", err)
	}

	scanned := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		if _, skip := excludedPresetDirs[strings.ToLower(name)]; skip {
			continue
		}
		if isPresetCategoryName(name) {
			scanned = append(scanned, name)
		}
	}
	sort.Strings(scanned)

	if len(scanned) == 0 {
		return defaults, nil
	}

	return uniqueStringsOrdered(append(scanned, defaults...)), nil
}

// ResolvePresetCategories applies user-configured category filtering.
func ResolvePresetCategories(configured, discovered []string) []string {
	selected := make([]string, 0, len(configured))
	for _, item := range configured {
		name := strings.TrimSpace(item)
		if name != "" {
			selected = append(selected, name)
		}
	}
	selected = uniqueStringsOrdered(selected)
	if len(selected) > 0 {
		return selected
	}

	discovered = uniqueStringsOrdered(discovered)
	if len(discovered) > 0 {
		return discovered
	}

	return append([]string(nil), defaultPresetCategories...)
}

// NewPresetSyncManager creates a preset sync manager.
func NewPresetSyncManager(opts PresetSyncOptions) *PresetSyncManager {
	logf := opts.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}

	tolerance := opts.MTimeTolerance
	if tolerance <= 0 {
		tolerance = defaultPresetSyncTolerance
	}

	statePath := strings.TrimSpace(opts.StatePath)
	if statePath == "" && strings.TrimSpace(opts.LocalLightroomDir) != "" {
		statePath = DefaultPresetStatePath(opts.LocalLightroomDir)
	}

	return &PresetSyncManager{
		backupDir:  strings.TrimSpace(opts.BackupDir),
		localRoot:  strings.TrimSpace(opts.LocalLightroomDir),
		statePath:  statePath,
		categories: append([]string(nil), opts.Categories...),
		tolerance:  tolerance,
		logf:       logf,
	}
}

// Sync runs one complete pull+push preset synchronization cycle.
func (m *PresetSyncManager) Sync(ctx context.Context) (PresetSyncSummary, error) {
	var summary PresetSyncSummary

	if strings.TrimSpace(m.backupDir) == "" {
		return summary, errors.New("backup directory is empty")
	}
	if strings.TrimSpace(m.localRoot) == "" {
		return summary, errors.New("local lightroom directory is empty")
	}
	if err := ctx.Err(); err != nil {
		return summary, err
	}

	networkDir := filepath.Join(m.backupDir, presetRootFolderName)
	if err := os.MkdirAll(networkDir, 0o755); err != nil {
		return summary, fmt.Errorf("create network preset directory: %w", err)
	}

	discoveredCategories, err := DiscoverPresetCategories(m.localRoot)
	if err != nil {
		return summary, err
	}
	categories := ResolvePresetCategories(m.categories, discoveredCategories)
	if len(categories) == 0 {
		return summary, nil
	}

	if err := ensureRemoteCategoryBootstrap(networkDir, categories); err != nil {
		return summary, err
	}

	state, err := readPresetState(m.statePath)
	if err != nil {
		m.logf("[WARN] failed to read preset state, continuing with empty state: %v", err)
		state = map[string]float64{}
	}
	newState := clonePresetState(state)

	selectedCategories := toCategorySet(categories)
	toleranceSec := m.tolerance.Seconds()

	networkFiles, err := scanPresetFiles(ctx, networkDir, categories, false)
	if err != nil {
		return summary, err
	}

	var cycleErr error

	tombstonePath := filepath.Join(networkDir, ".sync_deleted.json")
	tombstone, tombErr := readPresetState(tombstonePath)
	if tombErr != nil {
		m.logf("[WARN] failed to read tombstone state, continuing with empty: %v", tombErr)
		tombstone = map[string]float64{}
	}
	tombstoneModified := false

	// PULL A: apply network-side deletions.
	toRemove := make([]string, 0, len(state))
	for relPath, lastMTime := range state {
		if !isTrackedCategory(relPath, selectedCategories) {
			continue
		}
		if _, exists := networkFiles[relPath]; exists {
			continue
		}

		localPath := filepath.Join(m.localRoot, filepath.FromSlash(relPath))
		info, statErr := os.Stat(localPath)
		if statErr == nil {
			if math.Abs(fileModSeconds(info.ModTime())-lastMTime) < toleranceSec {
				if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
					cycleErr = firstError(cycleErr, fmt.Errorf("delete local preset %s: %w", relPath, err))
				} else if err == nil {
					summary.Deleted++
				}
			}
		} else if !os.IsNotExist(statErr) {
			cycleErr = firstError(cycleErr, fmt.Errorf("stat local preset %s: %w", relPath, statErr))
		}
		toRemove = append(toRemove, relPath)
	}
	for _, relPath := range toRemove {
		delete(newState, relPath)
	}

	// PULL B: pull network additions/modifications.
	for relPath, netMeta := range networkFiles {
		if err := ctx.Err(); err != nil {
			return summary, err
		}

		localPath := filepath.Join(m.localRoot, filepath.FromSlash(relPath))
		needsPull := false
		localInfo, statErr := os.Stat(localPath)
		switch {
		case statErr == nil:
			if netMeta.MTime > fileModSeconds(localInfo.ModTime())+toleranceSec {
				needsPull = true
			}
		case os.IsNotExist(statErr):
			lastSeenMTime, tracked := state[relPath]
			if tracked {
				needsPull = netMeta.MTime > lastSeenMTime+toleranceSec
			} else {
				needsPull = true
			}
		default:
			cycleErr = firstError(cycleErr, fmt.Errorf("stat local preset %s: %w", relPath, statErr))
			continue
		}

		if !needsPull {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
			cycleErr = firstError(cycleErr, fmt.Errorf("create local preset directory for %s: %w", relPath, err))
			continue
		}

		logosCopied := 0
		if isWatermarkCategory(relPath) {
			logosCopied, err = m.pullWatermark(netMeta.Path, localPath, networkDir)
		} else {
			err = copyFileWithMTime(netMeta.Path, localPath)
		}
		if err != nil {
			cycleErr = firstError(cycleErr, fmt.Errorf("pull preset %s: %w", relPath, err))
			continue
		}

		localInfo, statErr = os.Stat(localPath)
		if statErr != nil {
			cycleErr = firstError(cycleErr, fmt.Errorf("stat pulled local preset %s: %w", relPath, statErr))
			continue
		}
		newState[relPath] = fileModSeconds(localInfo.ModTime())
		summary.Pulled++
		summary.LogosCopied += logosCopied
	}

	// PUSH scan local.
	localFiles, err := scanPresetFiles(ctx, m.localRoot, categories, true)
	if err != nil {
		return summary, err
	}

	// PUSH A: apply local deletions.
	toRemove = toRemove[:0]
	for relPath := range newState {
		if !isTrackedCategory(relPath, selectedCategories) {
			continue
		}
		if _, exists := localFiles[relPath]; exists {
			continue
		}

		networkPath := filepath.Join(networkDir, filepath.FromSlash(relPath))
		if _, statErr := os.Stat(networkPath); statErr == nil {
			if err := os.Remove(networkPath); err != nil && !os.IsNotExist(err) {
				cycleErr = firstError(cycleErr, fmt.Errorf("delete network preset %s: %w", relPath, err))
			} else if err == nil {
				summary.Deleted++
				tombstone[relPath] = fileModSeconds(time.Now())
				tombstoneModified = true
			}
		} else if !os.IsNotExist(statErr) {
			cycleErr = firstError(cycleErr, fmt.Errorf("stat network preset %s: %w", relPath, statErr))
		}

		toRemove = append(toRemove, relPath)
	}
	for _, relPath := range toRemove {
		delete(newState, relPath)
	}

	// PUSH B: push local additions/modifications.
	for relPath, localMeta := range localFiles {
		if err := ctx.Err(); err != nil {
			return summary, err
		}

		networkPath := filepath.Join(networkDir, filepath.FromSlash(relPath))
		localMTime := localMeta.MTime
		needsPush := false

		networkInfo, statErr := os.Stat(networkPath)
		switch {
		case statErr == nil:
			lastSeenMTime, tracked := newState[relPath]
			if !tracked || localMTime > lastSeenMTime+toleranceSec {
				if localMTime > fileModSeconds(networkInfo.ModTime()) {
					needsPush = true
				}
			}
		case os.IsNotExist(statErr):
			needsPush = true
		default:
			cycleErr = firstError(cycleErr, fmt.Errorf("stat network preset %s: %w", relPath, statErr))
			continue
		}

		if !needsPush {
			continue
		}

		if deleteMTime, exists := tombstone[relPath]; exists {
			if localMTime <= deleteMTime+toleranceSec {
				m.logf("[INFO] preventing zombie push for %s (mtime %v <= tombstone %v)", relPath, localMTime, deleteMTime)
				localPath := filepath.Join(m.localRoot, filepath.FromSlash(relPath))
				if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
					cycleErr = firstError(cycleErr, fmt.Errorf("delete zombie local preset %s: %w", relPath, err))
				} else {
					summary.Deleted++
				}
				continue
			}
			delete(tombstone, relPath)
			tombstoneModified = true
		}

		if err := os.MkdirAll(filepath.Dir(networkPath), 0o755); err != nil {
			cycleErr = firstError(cycleErr, fmt.Errorf("create network preset directory for %s: %w", relPath, err))
			continue
		}

		logosCopied := 0
		if isWatermarkCategory(relPath) {
			logosCopied, err = m.pushWatermark(localMeta.Path, networkPath, networkDir)
		} else {
			err = copyFileWithMTime(localMeta.Path, networkPath)
		}
		if err != nil {
			cycleErr = firstError(cycleErr, fmt.Errorf("push preset %s: %w", relPath, err))
			continue
		}

		localInfo, statErr := os.Stat(localMeta.Path)
		if statErr != nil {
			cycleErr = firstError(cycleErr, fmt.Errorf("stat local preset after push %s: %w", relPath, statErr))
			continue
		}

		newState[relPath] = fileModSeconds(localInfo.ModTime())
		summary.Pushed++
		summary.LogosCopied += logosCopied
	}

	if cycleErr != nil {
		return summary, cycleErr
	}

	if err := writePresetState(m.statePath, newState); err != nil {
		return summary, err
	}
	if tombstoneModified {
		if err := writePresetState(tombstonePath, tombstone); err != nil {
			m.logf("[WARN] failed to write tombstone state: %v", err)
		}
	}
	summary.Tracked = len(newState)

	return summary, nil
}

func (m *PresetSyncManager) pullWatermark(networkPath, localPath, networkPresetDir string) (int, error) {
	if !isWatermarkTemplateFile(localPath) {
		return 0, copyFileWithMTime(networkPath, localPath)
	}

	contentRaw, err := os.ReadFile(networkPath)
	if err != nil {
		return 0, err
	}
	if !utf8.Valid(contentRaw) {
		return 0, copyFileWithMTime(networkPath, localPath)
	}

	content := string(contentRaw)
	networkLogosDir := filepath.Join(networkPresetDir, "Watermarks", presetLogoFolderName)
	localLogosDir := filepath.Join(m.localRoot, "Watermarks", presetLogoFolderName)
	logosCopied := 0
	var rewriteErr error

	rewritten := watermarkImagePathPattern.ReplaceAllStringFunc(content, func(match string) string {
		if rewriteErr != nil {
			return match
		}

		sub := watermarkImagePathPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}

		rawPath := unescapeTemplatePath(sub[1])
		fileName := filepath.Base(rawPath)
		if fileName == "" || fileName == "." {
			return match
		}

		networkLogoPath := filepath.Join(networkLogosDir, fileName)
		if _, statErr := os.Stat(networkLogoPath); statErr == nil {
			if err := os.MkdirAll(localLogosDir, 0o755); err != nil {
				rewriteErr = err
				return match
			}
			localLogoPath := filepath.Join(localLogosDir, fileName)
			copied, copyErr := copyIfSizeDiff(networkLogoPath, localLogoPath)
			if copyErr != nil {
				rewriteErr = copyErr
				return match
			}
			if copied {
				logosCopied++
			}
			return fmt.Sprintf(`imagePath = "%s"`, escapeTemplatePath(localLogoPath))
		}

		m.logf("[WARN] watermark logo not found on network: %s", fileName)
		return fmt.Sprintf(`imagePath = "%s"`, escapeTemplatePath(networkLogoPath))
	})
	if rewriteErr != nil {
		return logosCopied, rewriteErr
	}

	srcInfo, err := os.Stat(networkPath)
	if err != nil {
		return logosCopied, err
	}
	if err := writeTextWithMTime(localPath, rewritten, srcInfo.ModTime().UTC()); err != nil {
		return logosCopied, err
	}
	return logosCopied, nil
}

func (m *PresetSyncManager) pushWatermark(localPath, networkPath, networkPresetDir string) (int, error) {
	if !isWatermarkTemplateFile(localPath) {
		return 0, copyFileWithMTime(localPath, networkPath)
	}

	contentRaw, err := os.ReadFile(localPath)
	if err != nil {
		return 0, err
	}
	if !utf8.Valid(contentRaw) {
		return 0, copyFileWithMTime(localPath, networkPath)
	}

	localInfo, err := os.Stat(localPath)
	if err != nil {
		return 0, err
	}
	originalMTime := localInfo.ModTime().UTC()

	content := string(contentRaw)
	networkLogosDir := filepath.Join(networkPresetDir, "Watermarks", presetLogoFolderName)
	if err := os.MkdirAll(networkLogosDir, 0o755); err != nil {
		return 0, err
	}

	logosCopied := 0
	var rewriteErr error

	rewritten := watermarkImagePathPattern.ReplaceAllStringFunc(content, func(match string) string {
		if rewriteErr != nil {
			return match
		}

		sub := watermarkImagePathPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}

		rawPath := unescapeTemplatePath(sub[1])
		localLogoPath := filepath.Clean(rawPath)
		if _, statErr := os.Stat(localLogoPath); statErr != nil {
			m.logf("[WARN] local watermark logo not found: %s", localLogoPath)
			return match
		}

		fileName := filepath.Base(localLogoPath)
		networkLogoPath := filepath.Join(networkLogosDir, fileName)
		copied, copyErr := copyIfSizeDiff(localLogoPath, networkLogoPath)
		if copyErr != nil {
			rewriteErr = copyErr
			return match
		}
		if copied {
			logosCopied++
		}
		return fmt.Sprintf(`imagePath = "%s"`, escapeTemplatePath(networkLogoPath))
	})
	if rewriteErr != nil {
		return logosCopied, rewriteErr
	}

	if err := writeTextWithMTime(networkPath, rewritten, originalMTime); err != nil {
		return logosCopied, err
	}
	if rewritten != content {
		if err := writeTextWithMTime(localPath, rewritten, originalMTime); err != nil {
			return logosCopied, err
		}
	}

	return logosCopied, nil
}

func ensureRemoteCategoryBootstrap(networkDir string, categories []string) error {
	for _, category := range categories {
		category = strings.TrimSpace(category)
		if category == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Join(networkDir, category), 0o755); err != nil {
			return fmt.Errorf("create network category %s: %w", category, err)
		}
	}
	return nil
}

func scanPresetFiles(ctx context.Context, root string, categories []string, skipJSON bool) (map[string]presetFileMeta, error) {
	files := make(map[string]presetFileMeta, 256)
	for _, category := range categories {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		category = strings.TrimSpace(category)
		if category == "" {
			continue
		}

		categoryDir := filepath.Join(root, category)
		if _, err := os.Stat(categoryDir); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat category %s: %w", category, err)
		}

		walkErr := filepath.WalkDir(categoryDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == presetLogoFolderName {
					return filepath.SkipDir
				}
				return nil
			}
			if err := ctx.Err(); err != nil {
				return err
			}

			ext := strings.ToLower(filepath.Ext(d.Name()))
			if ext == ".tmp" || ext == ".lock" {
				return nil
			}
			if skipJSON && ext == ".json" {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return err
			}

			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			files[rel] = presetFileMeta{
				Path:  path,
				MTime: fileModSeconds(info.ModTime()),
			}
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("scan category %s: %w", category, walkErr)
		}
	}
	return files, nil
}

func readPresetState(path string) (map[string]float64, error) {
	state := map[string]float64{}
	if strings.TrimSpace(path) == "" {
		return state, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return nil, fmt.Errorf("read preset state: %w", err)
	}
	if len(data) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse preset state: %w", err)
	}
	return state, nil
}

func writePresetState(path string, state map[string]float64) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create preset state dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal preset state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write preset state: %w", err)
	}
	return nil
}

func copyFileWithMTime(srcPath, dstPath string) error {
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("stat source %s: %w", srcPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create destination directory for %s: %w", dstPath, err)
	}

	tmpPath := dstPath + ".tmp"
	if err := copyFileRaw(srcPath, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := replaceFile(tmpPath, dstPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace file %s -> %s: %w", tmpPath, dstPath, err)
	}

	modTime := srcInfo.ModTime().UTC()
	if err := os.Chtimes(dstPath, modTime, modTime); err != nil {
		return fmt.Errorf("set file time for %s: %w", dstPath, err)
	}
	return nil
}

func copyFileRaw(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source file %s: %w", srcPath, err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("create destination file %s: %w", dstPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", srcPath, dstPath, err)
	}
	return nil
}

func copyIfSizeDiff(srcPath, dstPath string) (bool, error) {
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return false, err
	}

	dstInfo, err := os.Stat(dstPath)
	if err == nil {
		if dstInfo.Size() == srcInfo.Size() && !dstInfo.ModTime().Before(srcInfo.ModTime()) {
			return false, nil
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}

	if err := copyFileWithMTime(srcPath, dstPath); err != nil {
		return false, err
	}
	return true, nil
}

func writeTextWithMTime(path, content string, modTime time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := replaceFile(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		return err
	}
	return nil
}

func replaceFile(srcPath, dstPath string) error {
	if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(srcPath, dstPath)
}

func isPresetCategoryName(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "preset") ||
		strings.Contains(lower, "template") ||
		strings.EqualFold(name, "Watermarks")
}

func isTrackedCategory(relPath string, categorySet map[string]struct{}) bool {
	parts := strings.Split(relPath, "/")
	if len(parts) == 0 || parts[0] == "" {
		return false
	}
	_, ok := categorySet[strings.ToLower(parts[0])]
	return ok
}

func isWatermarkCategory(relPath string) bool {
	parts := strings.Split(relPath, "/")
	if len(parts) == 0 || parts[0] == "" {
		return false
	}
	return strings.EqualFold(parts[0], "Watermarks")
}

func isWatermarkTemplateFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".lrtemplate" || ext == ".lrsmv"
}

func escapeTemplatePath(path string) string {
	normalized := strings.ReplaceAll(filepath.Clean(path), "/", `\`)
	return strings.ReplaceAll(normalized, `\`, `\\`)
}

func unescapeTemplatePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.ReplaceAll(path, `\\`, `\`)
	return filepath.Clean(path)
}

func fileModSeconds(ts time.Time) float64 {
	return float64(ts.UTC().UnixNano()) / float64(time.Second)
}

func toCategorySet(categories []string) map[string]struct{} {
	set := make(map[string]struct{}, len(categories))
	for _, item := range categories {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		set[strings.ToLower(item)] = struct{}{}
	}
	return set
}

func clonePresetState(state map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(state))
	for k, v := range state {
		out[k] = v
	}
	return out
}

func uniqueStringsOrdered(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func firstError(existing, candidate error) error {
	if existing != nil {
		return existing
	}
	return candidate
}
