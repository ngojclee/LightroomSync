package sync

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDiscoverPresetCategories_FiltersAndMergesDefaults(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "Preferences"))
	mustMkdirAll(t, filepath.Join(root, ".lightroom-sync"))
	mustMkdirAll(t, filepath.Join(root, "Lightroom Presets"))
	mustMkdirAll(t, filepath.Join(root, "My Presets"))
	mustMkdirAll(t, filepath.Join(root, "Title Templates"))
	mustMkdirAll(t, filepath.Join(root, "Watermarks"))

	categories, err := DiscoverPresetCategories(root)
	if err != nil {
		t.Fatalf("DiscoverPresetCategories() error = %v", err)
	}

	// Scanned categories should appear and defaults should still be present.
	for _, want := range []string{
		"My Presets",
		"Title Templates",
		"Watermarks",
		"Export Presets",
		"Develop Presets",
		"Metadata Presets",
		"Filename Templates",
	} {
		if !containsString(categories, want) {
			t.Fatalf("categories missing %q: %v", want, categories)
		}
	}
	if containsString(categories, "Preferences") {
		t.Fatalf("excluded directory leaked into categories: %v", categories)
	}
}

func TestResolvePresetCategories_UsesUserSelectionFirst(t *testing.T) {
	discovered := []string{"Export Presets", "Develop Presets", "Watermarks"}
	resolved := ResolvePresetCategories([]string{"  Watermarks  ", "Watermarks", "Metadata Presets"}, discovered)

	want := []string{"Watermarks", "Metadata Presets"}
	if len(resolved) != len(want) {
		t.Fatalf("resolved len=%d want=%d resolved=%v", len(resolved), len(want), resolved)
	}
	for i := range want {
		if resolved[i] != want[i] {
			t.Fatalf("resolved[%d]=%q want=%q", i, resolved[i], want[i])
		}
	}
}

func TestPresetSync_PushThenApplyNetworkDeletion(t *testing.T) {
	root := t.TempDir()
	localRoot := filepath.Join(root, "local")
	backupDir := filepath.Join(root, "network")
	statePath := filepath.Join(localRoot, ".lightroom-sync", "preset_state.json")

	localPreset := filepath.Join(localRoot, "Develop Presets", "Tone.xmp")
	mustWriteFile(t, localPreset, []byte("local-content"))

	mgr := NewPresetSyncManager(PresetSyncOptions{
		BackupDir:         backupDir,
		LocalLightroomDir: localRoot,
		Categories:        []string{"Develop Presets"},
		StatePath:         statePath,
	})

	s1, err := mgr.Sync(context.Background())
	if err != nil {
		t.Fatalf("first sync error = %v", err)
	}
	if s1.Pushed != 1 {
		t.Fatalf("first sync pushed=%d want=1", s1.Pushed)
	}

	networkPreset := filepath.Join(backupDir, "Presets", "Develop Presets", "Tone.xmp")
	if _, err := os.Stat(networkPreset); err != nil {
		t.Fatalf("network preset missing after push: %v", err)
	}

	state := readStateFile(t, statePath)
	if _, ok := state["Develop Presets/Tone.xmp"]; !ok {
		t.Fatalf("state missing expected key after push: %v", state)
	}

	if err := os.Remove(networkPreset); err != nil {
		t.Fatalf("remove network preset: %v", err)
	}

	s2, err := mgr.Sync(context.Background())
	if err != nil {
		t.Fatalf("second sync error = %v", err)
	}
	if s2.Deleted == 0 {
		t.Fatalf("second sync should report deletion, got %+v", s2)
	}
	if _, err := os.Stat(localPreset); !os.IsNotExist(err) {
		t.Fatalf("local preset should be deleted after network deletion, stat err=%v", err)
	}

	state2 := readStateFile(t, statePath)
	if _, ok := state2["Develop Presets/Tone.xmp"]; ok {
		t.Fatalf("state should remove deleted key, got=%v", state2)
	}
}

func TestPresetSync_MTimeToleranceAvoidsFalseConflict(t *testing.T) {
	root := t.TempDir()
	localRoot := filepath.Join(root, "local")
	backupDir := filepath.Join(root, "network")
	statePath := filepath.Join(localRoot, ".lightroom-sync", "preset_state.json")

	localPath := filepath.Join(localRoot, "Develop Presets", "SplitTone.xmp")
	networkPath := filepath.Join(backupDir, "Presets", "Develop Presets", "SplitTone.xmp")

	mustWriteFile(t, localPath, []byte("local"))
	mustWriteFile(t, networkPath, []byte("network"))

	base := time.Now().UTC().Add(-10 * time.Second).Round(time.Second)
	if err := os.Chtimes(localPath, base, base); err != nil {
		t.Fatalf("set local time: %v", err)
	}
	networkTime := base.Add(1 * time.Second) // lower than default tolerance 2s
	if err := os.Chtimes(networkPath, networkTime, networkTime); err != nil {
		t.Fatalf("set network time: %v", err)
	}

	mgr := NewPresetSyncManager(PresetSyncOptions{
		BackupDir:         backupDir,
		LocalLightroomDir: localRoot,
		Categories:        []string{"Develop Presets"},
		StatePath:         statePath,
	})

	summary, err := mgr.Sync(context.Background())
	if err != nil {
		t.Fatalf("sync error = %v", err)
	}
	if summary.Pulled != 0 || summary.Pushed != 0 {
		t.Fatalf("expected no transfer due to tolerance, got %+v", summary)
	}

	gotLocal := mustReadString(t, localPath)
	if gotLocal != "local" {
		t.Fatalf("local content changed unexpectedly: %q", gotLocal)
	}
}

func TestPresetSync_WatermarkPushRewritesTemplateAndCopiesLogo(t *testing.T) {
	root := t.TempDir()
	localRoot := filepath.Join(root, "local")
	backupDir := filepath.Join(root, "network")
	statePath := filepath.Join(localRoot, ".lightroom-sync", "preset_state.json")

	localLogo := filepath.Join(localRoot, "Watermarks", "Logos", "brand.png")
	localTemplate := filepath.Join(localRoot, "Watermarks", "Brand.lrtemplate")

	mustWriteFile(t, localLogo, []byte("logo-data"))
	mustWriteFile(t, localTemplate, []byte(`imagePath = "`+escapeTemplatePathForTest(localLogo)+`"`))

	mgr := NewPresetSyncManager(PresetSyncOptions{
		BackupDir:         backupDir,
		LocalLightroomDir: localRoot,
		Categories:        []string{"Watermarks"},
		StatePath:         statePath,
	})

	summary, err := mgr.Sync(context.Background())
	if err != nil {
		t.Fatalf("sync error = %v", err)
	}
	if summary.Pushed == 0 {
		t.Fatalf("expected watermark push, got %+v", summary)
	}
	if summary.LogosCopied == 0 {
		t.Fatalf("expected logo copy, got %+v", summary)
	}

	networkLogo := filepath.Join(backupDir, "Presets", "Logos", "brand.png")
	if _, err := os.Stat(networkLogo); err != nil {
		t.Fatalf("network logo missing: %v", err)
	}

	networkTemplate := filepath.Join(backupDir, "Presets", "Watermarks", "Brand.lrtemplate")
	content := mustReadString(t, networkTemplate)
	wantPath := escapeTemplatePathForTest(networkLogo)
	if !strings.Contains(content, wantPath) {
		t.Fatalf("network template was not rewritten to network logo path:\nwant contains: %s\ncontent: %s", wantPath, content)
	}

	localContent := mustReadString(t, localTemplate)
	if !strings.Contains(localContent, wantPath) {
		t.Fatalf("local template should be updated to rewritten path:\nwant contains: %s\ncontent: %s", wantPath, localContent)
	}
}

func TestPresetSync_WatermarkPullRewritesToLocalLogoPath(t *testing.T) {
	root := t.TempDir()
	localRoot := filepath.Join(root, "local")
	backupDir := filepath.Join(root, "network")
	statePath := filepath.Join(localRoot, ".lightroom-sync", "preset_state.json")

	networkLogo := filepath.Join(backupDir, "Presets", "Logos", "remote.png")
	networkTemplate := filepath.Join(backupDir, "Presets", "Watermarks", "Remote.lrsmv")
	mustWriteFile(t, networkLogo, []byte("remote-logo"))
	mustWriteFile(t, networkTemplate, []byte(`imagePath = "`+escapeTemplatePathForTest(networkLogo)+`"`))

	mgr := NewPresetSyncManager(PresetSyncOptions{
		BackupDir:         backupDir,
		LocalLightroomDir: localRoot,
		Categories:        []string{"Watermarks"},
		StatePath:         statePath,
	})

	summary, err := mgr.Sync(context.Background())
	if err != nil {
		t.Fatalf("sync error = %v", err)
	}
	if summary.Pulled == 0 {
		t.Fatalf("expected watermark pull, got %+v", summary)
	}
	if summary.LogosCopied == 0 {
		t.Fatalf("expected pulled logo copy, got %+v", summary)
	}

	localLogo := filepath.Join(localRoot, "Watermarks", "Logos", "remote.png")
	if _, err := os.Stat(localLogo); err != nil {
		t.Fatalf("local logo missing: %v", err)
	}

	localTemplate := filepath.Join(localRoot, "Watermarks", "Remote.lrsmv")
	localContent := mustReadString(t, localTemplate)
	wantPath := escapeTemplatePathForTest(localLogo)
	if !strings.Contains(localContent, wantPath) {
		t.Fatalf("local watermark template was not rewritten to local logo path:\nwant contains: %s\ncontent: %s", wantPath, localContent)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func mustReadString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return string(data)
}

func readStateFile(t *testing.T, path string) map[string]float64 {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state file %s: %v", path, err)
	}
	state := map[string]float64{}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("parse state file %s: %v", path, err)
	}
	return state
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func escapeTemplatePathForTest(path string) string {
	normalized := strings.ReplaceAll(filepath.Clean(path), "/", `\`)
	return strings.ReplaceAll(normalized, `\`, `\\`)
}
