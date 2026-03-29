package monitor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListZipBackups_RecursiveSortedNewestFirst(t *testing.T) {
	root := t.TempDir()
	subA := filepath.Join(root, "A")
	subB := filepath.Join(root, "B", "Nested")
	if err := os.MkdirAll(subA, 0o755); err != nil {
		t.Fatalf("mkdir subA: %v", err)
	}
	if err := os.MkdirAll(subB, 0o755); err != nil {
		t.Fatalf("mkdir subB: %v", err)
	}

	zip1 := filepath.Join(subA, "older.zip")
	zip2 := filepath.Join(subB, "newer.zip")
	txt := filepath.Join(subB, "ignore.txt")

	if err := os.WriteFile(zip1, []byte("a"), 0o644); err != nil {
		t.Fatalf("write zip1: %v", err)
	}
	if err := os.WriteFile(zip2, []byte("bb"), 0o644); err != nil {
		t.Fatalf("write zip2: %v", err)
	}
	if err := os.WriteFile(txt, []byte("noop"), 0o644); err != nil {
		t.Fatalf("write txt: %v", err)
	}

	old := time.Date(2026, 3, 30, 0, 0, 1, 0, time.UTC)
	newTime := time.Date(2026, 3, 30, 0, 0, 3, 0, time.UTC)
	if err := os.Chtimes(zip1, old, old); err != nil {
		t.Fatalf("chtimes zip1: %v", err)
	}
	if err := os.Chtimes(zip2, newTime, newTime); err != nil {
		t.Fatalf("chtimes zip2: %v", err)
	}

	backups, err := ListZipBackups(context.Background(), root)
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("len(backups) = %d, want 2", len(backups))
	}
	if backups[0].Path != zip2 {
		t.Fatalf("backups[0].Path = %s, want %s", backups[0].Path, zip2)
	}
	if backups[1].Path != zip1 {
		t.Fatalf("backups[1].Path = %s, want %s", backups[1].Path, zip1)
	}
}

func TestBackupMonitor_EmitsOnlyOnSignatureChange(t *testing.T) {
	infoA := &BackupInfo{Path: `D:\A\1.zip`, Size: 100, ModTime: time.Date(2026, 3, 30, 0, 1, 0, 0, time.UTC)}
	infoB := &BackupInfo{Path: `D:\A\2.zip`, Size: 200, ModTime: time.Date(2026, 3, 30, 0, 2, 0, 0, time.UTC)}

	steps := []struct {
		info *BackupInfo
		sig  string
		err  error
	}{
		{info: nil, sig: "", err: nil}, // baseline
		{info: infoA, sig: backupSignature(*infoA), err: nil},
		{info: infoA, sig: backupSignature(*infoA), err: nil}, // duplicate: no emit
		{info: infoB, sig: backupSignature(*infoB), err: nil},
	}
	stepIdx := 0

	emitted := make([]BackupInfo, 0, 2)
	monitor := NewBackupMonitor(`D:\A`, 1*time.Second, BackupHooks{
		OnNewBackup: func(info BackupInfo) {
			emitted = append(emitted, info)
		},
	})
	monitor.scanFn = func(ctx context.Context) (*BackupInfo, string, error) {
		if stepIdx >= len(steps) {
			return nil, "", nil
		}
		s := steps[stepIdx]
		stepIdx++
		return s.info, s.sig, s.err
	}

	for i := 0; i < len(steps); i++ {
		monitor.scanAndDispatch(context.Background())
	}

	if len(emitted) != 2 {
		t.Fatalf("len(emitted) = %d, want 2", len(emitted))
	}
	if emitted[0].Path != infoA.Path {
		t.Fatalf("first emitted path = %s, want %s", emitted[0].Path, infoA.Path)
	}
	if emitted[1].Path != infoB.Path {
		t.Fatalf("second emitted path = %s, want %s", emitted[1].Path, infoB.Path)
	}
}

func TestBackupMonitor_ReportsScanErrors(t *testing.T) {
	expectedErr := errors.New("scan failed")
	called := false

	monitor := NewBackupMonitor(`D:\A`, 1*time.Second, BackupHooks{
		OnError: func(err error) {
			if err == expectedErr {
				called = true
			}
		},
	})
	monitor.scanFn = func(ctx context.Context) (*BackupInfo, string, error) {
		return nil, "", expectedErr
	}

	monitor.scanAndDispatch(context.Background())
	if !called {
		t.Fatal("expected OnError callback to be called")
	}
}

func TestListZipBackups_NonExistentRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing-dir")
	backups, err := ListZipBackups(context.Background(), root)
	if err != nil {
		t.Fatalf("ListZipBackups unexpected error: %v", err)
	}
	if len(backups) != 0 {
		t.Fatalf("len(backups) = %d, want 0", len(backups))
	}
}
