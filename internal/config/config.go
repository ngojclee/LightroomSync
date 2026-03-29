package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
// Field names and defaults match the Python version's YAML schema exactly.
type Config struct {
	BackupFolder        string   `yaml:"backup_folder"`
	CatalogPath         string   `yaml:"catalog_path"`
	StartWithWindows    bool     `yaml:"start_with_windows"`
	StartMinimized      bool     `yaml:"start_minimized"`
	MinimizeToTray      bool     `yaml:"minimize_to_tray"`
	AutoSync            bool     `yaml:"auto_sync"`
	HeartbeatInterval   int      `yaml:"heartbeat_interval"`
	CheckInterval       int      `yaml:"check_interval"`
	LockTimeout         int      `yaml:"lock_timeout"`
	MaxCatalogBackups   int      `yaml:"max_catalog_backups"`
	PresetSyncEnabled   bool     `yaml:"preset_sync_enabled"`
	PresetCategories    []string `yaml:"preset_categories"`
	LastSyncedTimestamp string   `yaml:"last_synced_timestamp"`
}

var defaultConfig = Config{
	BackupFolder:        "",
	CatalogPath:         "",
	StartWithWindows:    false,
	StartMinimized:      false,
	MinimizeToTray:      true,
	AutoSync:            false,
	HeartbeatInterval:   30,
	CheckInterval:       60,
	LockTimeout:         120,
	MaxCatalogBackups:   5,
	PresetSyncEnabled:   true,
	PresetCategories:    []string{"Export Presets", "Develop Presets", "Watermarks"},
	LastSyncedTimestamp: "",
}

// Manager handles loading/saving configuration with thread safety.
type Manager struct {
	mu       sync.RWMutex
	cfg      Config
	filePath string
}

// DefaultPath returns the standard config file location.
// %LOCALAPPDATA%\LightroomSync\config.yaml
func DefaultPath() (string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return "", fmt.Errorf("LOCALAPPDATA environment variable not set")
	}
	return filepath.Join(localAppData, "LightroomSync", "config.yaml"), nil
}

// NewManager creates a config manager for the given file path.
func NewManager(filePath string) *Manager {
	return &Manager{
		cfg:      defaultConfig,
		filePath: filePath,
	}
}

// Load reads configuration from disk, merging with defaults.
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cfg = defaultConfig

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // use defaults
		}
		return fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, &m.cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	m.applyDefaults()
	return nil
}

// Save writes configuration to disk.
func (m *Manager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(&m.cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	header := "# LightroomSync Configuration\n# This file is saved in your Local AppData directory\n\n"
	return os.WriteFile(m.filePath, []byte(header+string(data)), 0o644)
}

// Get returns a copy of the current configuration.
func (m *Manager) Get() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c := m.cfg
	// deep copy slice
	c.PresetCategories = make([]string, len(m.cfg.PresetCategories))
	copy(c.PresetCategories, m.cfg.PresetCategories)
	return c
}

// Update replaces the configuration and saves to disk.
func (m *Manager) Update(cfg Config) error {
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
	return m.Save()
}

// SetLastSyncedTimestamp updates the timestamp and persists immediately.
func (m *Manager) SetLastSyncedTimestamp(ts string) error {
	m.mu.Lock()
	m.cfg.LastSyncedTimestamp = ts
	m.mu.Unlock()
	return m.Save()
}

func (m *Manager) applyDefaults() {
	if m.cfg.HeartbeatInterval <= 0 {
		m.cfg.HeartbeatInterval = defaultConfig.HeartbeatInterval
	}
	if m.cfg.CheckInterval <= 0 {
		m.cfg.CheckInterval = defaultConfig.CheckInterval
	}
	if m.cfg.LockTimeout <= 0 {
		m.cfg.LockTimeout = defaultConfig.LockTimeout
	}
	if m.cfg.PresetCategories == nil {
		m.cfg.PresetCategories = defaultConfig.PresetCategories
	}
}

// LegacyPaths returns known config locations used by previous Python versions.
// Ordered from most likely to least likely.
func LegacyPaths() []string {
	localAppData := os.Getenv("LOCALAPPDATA")
	exeDir := ""
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}

	paths := make([]string, 0, 6)
	if localAppData != "" {
		paths = append(paths,
			filepath.Join(localAppData, "Lightroom Sync", "LightroomSyncConfig.yaml"),
			filepath.Join(localAppData, "LightroomSync", "LightroomSyncConfig.yaml"),
			filepath.Join(localAppData, "Lightroom Sync", "config.yaml"),
			filepath.Join(localAppData, "LightroomSync", "config.yaml"),
		)
	}

	if exeDir != "" {
		paths = append(paths, filepath.Join(exeDir, "LightroomSyncConfig.yaml"))
	}

	return uniquePaths(paths)
}

// MigrateFromLegacyPaths migrates from the first existing legacy config path
// into the manager's current target path. It keeps a backup copy of the source.
//
// Returns:
// - migrated: true when migration occurred
// - sourcePath: the legacy file used for migration (if any)
func (m *Manager) MigrateFromLegacyPaths(legacyPaths []string) (bool, string, error) {
	m.mu.RLock()
	targetPath := m.filePath
	m.mu.RUnlock()

	if _, err := os.Stat(targetPath); err == nil {
		return false, "", nil
	} else if err != nil && !os.IsNotExist(err) {
		return false, "", fmt.Errorf("stat target config: %w", err)
	}

	for _, candidate := range legacyPaths {
		if candidate == "" {
			continue
		}

		cleanCandidate := filepath.Clean(candidate)
		if samePath(cleanCandidate, targetPath) {
			continue
		}

		data, err := os.ReadFile(cleanCandidate)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, "", fmt.Errorf("read legacy config %s: %w", cleanCandidate, err)
		}

		legacyCfg := defaultConfig
		if err := yaml.Unmarshal(data, &legacyCfg); err != nil {
			return false, "", fmt.Errorf("parse legacy config %s: %w", cleanCandidate, err)
		}

		m.mu.Lock()
		m.cfg = legacyCfg
		m.applyDefaults()
		m.mu.Unlock()

		if err := m.Save(); err != nil {
			return false, "", fmt.Errorf("save migrated config: %w", err)
		}

		if err := m.backupLegacyFile(cleanCandidate); err != nil {
			return false, "", err
		}

		return true, cleanCandidate, nil
	}

	return false, "", nil
}

func (m *Manager) backupLegacyFile(sourcePath string) error {
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read legacy file for backup: %w", err)
	}

	m.mu.RLock()
	targetPath := m.filePath
	m.mu.RUnlock()

	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create target config dir for backup: %w", err)
	}

	stamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("legacy_backup_%s_%s", stamp, filepath.Base(sourcePath))
	backupPath := filepath.Join(targetDir, backupName)

	if err := os.WriteFile(backupPath, sourceData, 0o644); err != nil {
		return fmt.Errorf("write legacy backup %s: %w", backupPath, err)
	}

	return nil
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func uniquePaths(paths []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		key := strings.ToLower(filepath.Clean(p))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return out
}
