package monitor

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupInfo describes a discovered backup zip.
type BackupInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
}

// BackupHooks receives callbacks from BackupMonitor.
type BackupHooks struct {
	OnNewBackup func(info BackupInfo)
	OnError     func(err error)
}

type backupScanFunc func(ctx context.Context) (*BackupInfo, string, error)

// BackupMonitor scans backup folders and emits only when the latest backup changes.
type BackupMonitor struct {
	root      string
	interval  time.Duration
	hooks     BackupHooks
	scanFn    backupScanFunc
	ready     bool
	lastSig   string
	lastKnown *BackupInfo
}

// NewBackupMonitor creates a backup monitor for recursive zip discovery.
func NewBackupMonitor(root string, interval time.Duration, hooks BackupHooks) *BackupMonitor {
	if interval <= 0 {
		interval = 60 * time.Second
	}

	m := &BackupMonitor{
		root:     root,
		interval: interval,
		hooks:    hooks,
	}
	m.scanFn = m.scanLatest
	return m
}

// Run blocks until context cancellation.
func (m *BackupMonitor) Run(ctx context.Context) {
	m.scanAndDispatch(ctx) // baseline

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.scanAndDispatch(ctx)
		}
	}
}

func (m *BackupMonitor) scanAndDispatch(ctx context.Context) {
	info, sig, err := m.scanFn(ctx)
	if err != nil {
		if m.hooks.OnError != nil {
			m.hooks.OnError(err)
		}
		return
	}

	if !m.ready {
		m.ready = true
		m.lastSig = sig
		m.lastKnown = info
		return
	}

	// No backups currently discovered; reset signature so next discovery can emit.
	if info == nil || sig == "" {
		m.lastSig = ""
		m.lastKnown = nil
		return
	}

	if sig == m.lastSig {
		return
	}

	m.lastSig = sig
	m.lastKnown = info
	if m.hooks.OnNewBackup != nil {
		m.hooks.OnNewBackup(*info)
	}
}

func (m *BackupMonitor) scanLatest(ctx context.Context) (*BackupInfo, string, error) {
	backups, err := ListZipBackups(ctx, m.root)
	if err != nil {
		return nil, "", err
	}
	if len(backups) == 0 {
		return nil, "", nil
	}
	latest := backups[0]
	return &latest, backupSignature(latest), nil
}

// ListZipBackups recursively discovers all .zip backups under root.
// Result is sorted newest-first by ModTime, then by path.
func ListZipBackups(ctx context.Context, root string) ([]BackupInfo, error) {
	if root == "" {
		return nil, nil
	}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	collected := make([]BackupInfo, 0, 32)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
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

		collected = append(collected, BackupInfo{
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime().UTC(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(collected, func(i, j int) bool {
		if collected[i].ModTime.Equal(collected[j].ModTime) {
			return collected[i].Path < collected[j].Path
		}
		return collected[i].ModTime.After(collected[j].ModTime)
	})

	return collected, nil
}

func backupSignature(info BackupInfo) string {
	return fmt.Sprintf("%s|%d|%d", filepath.Clean(info.Path), info.Size, info.ModTime.UTC().UnixNano())
}
