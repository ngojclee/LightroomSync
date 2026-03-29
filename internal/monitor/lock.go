package monitor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const lockFileName = "lightroom_lock.txt"

// LockStatus represents the status field in the lock file.
type LockStatus string

const (
	LockOnline  LockStatus = "ONLINE"
	LockOffline LockStatus = "OFFLINE"
)

// LockInfo represents a parsed lock file entry.
// Wire format: STATUS|MACHINE|TIMESTAMP (pipe-separated, UTF-8)
type LockInfo struct {
	Status    LockStatus
	Machine   string
	Timestamp time.Time
}

// String serializes to the Python-compatible wire format.
func (l LockInfo) String() string {
	// Python uses datetime.isoformat() which includes microseconds
	return fmt.Sprintf("%s|%s|%s", l.Status, l.Machine, l.Timestamp.Format("2006-01-02T15:04:05.999999"))
}

// ParseLock parses a lock file line in the format STATUS|MACHINE|TIMESTAMP.
func ParseLock(data string) (LockInfo, error) {
	line := strings.TrimSpace(data)
	parts := strings.SplitN(line, "|", 3)
	if len(parts) < 3 {
		return LockInfo{}, fmt.Errorf("invalid lock format: expected 3 pipe-separated fields, got %d", len(parts))
	}

	ts, err := time.Parse("2006-01-02T15:04:05.999999", parts[2])
	if err != nil {
		// fallback: try without microseconds
		ts, err = time.Parse("2006-01-02T15:04:05", parts[2])
		if err != nil {
			return LockInfo{}, fmt.Errorf("invalid lock timestamp %q: %w", parts[2], err)
		}
	}

	return LockInfo{
		Status:    LockStatus(parts[0]),
		Machine:   parts[1],
		Timestamp: ts,
	}, nil
}

// LockManager handles reading/writing the network lock file.
type LockManager struct {
	catalogDir string // e.g. \\server\share\Catalog
}

// NewLockManager creates a lock manager for the given catalog directory on the network.
func NewLockManager(catalogDir string) *LockManager {
	return &LockManager{catalogDir: catalogDir}
}

// LockPath returns the full path to the lock file.
func (m *LockManager) LockPath() string {
	return filepath.Join(m.catalogDir, lockFileName)
}

// ReadLock reads and parses the lock file. Returns nil info if file doesn't exist.
func (m *LockManager) ReadLock(ctx context.Context) (*LockInfo, error) {
	data, err := readFileWithContext(ctx, m.LockPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read lock: %w", err)
	}
	info, err := ParseLock(string(data))
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// WriteLock writes the lock file atomically (write tmp + rename).
func (m *LockManager) WriteLock(ctx context.Context, info LockInfo) error {
	lockPath := m.LockPath()
	dir := filepath.Dir(lockPath)

	tmpFile := filepath.Join(dir, ".lightroom_lock.tmp")
	if err := writeFileWithContext(ctx, tmpFile, []byte(info.String())); err != nil {
		return fmt.Errorf("write lock tmp: %w", err)
	}
	if err := os.Rename(tmpFile, lockPath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("rename lock: %w", err)
	}
	return nil
}

// IsStale returns true if the lock timestamp is older than timeout.
func (m *LockManager) IsStale(info LockInfo, timeout time.Duration) bool {
	return time.Since(info.Timestamp) > timeout
}

// readFileWithContext wraps os.ReadFile with context cancellation check.
func readFileWithContext(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Note: os.ReadFile is not natively cancellable on Windows SMB.
	// The context check here prevents starting the operation after cancellation.
	// The operation watchdog (Phase 3.4) handles truly stuck I/O.
	return os.ReadFile(path)
}

// writeFileWithContext wraps os.WriteFile with context cancellation check.
func writeFileWithContext(ctx context.Context, path string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
