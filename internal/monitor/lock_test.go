package monitor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"strings"
	"testing"
	"time"
)

// TestParseLock_PythonFixture_Online verifies Go can parse Python-generated ONLINE lock.
func TestParseLock_PythonFixture_Online(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "lightroom_lock_online.txt"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	info, err := ParseLock(string(data))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if info.Status != LockOnline {
		t.Errorf("status = %q, want %q", info.Status, LockOnline)
	}
	if info.Machine != "DESKTOP-ABC123" {
		t.Errorf("machine = %q, want %q", info.Machine, "DESKTOP-ABC123")
	}
	expected := time.Date(2026, 3, 29, 15, 42, 30, 123456000, time.UTC)
	if !info.Timestamp.Equal(expected) {
		t.Errorf("timestamp = %v, want %v", info.Timestamp, expected)
	}
}

// TestParseLock_PythonFixture_Offline verifies Go can parse Python-generated OFFLINE lock.
func TestParseLock_PythonFixture_Offline(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "lightroom_lock_offline.txt"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	info, err := ParseLock(string(data))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if info.Status != LockOffline {
		t.Errorf("status = %q, want %q", info.Status, LockOffline)
	}
	if info.Machine != "LAPTOP-XYZ789" {
		t.Errorf("machine = %q, want %q", info.Machine, "LAPTOP-XYZ789")
	}
}

// TestLockInfo_Roundtrip verifies Go-written lock can be re-parsed.
func TestLockInfo_Roundtrip(t *testing.T) {
	original := LockInfo{
		Status:    LockOnline,
		Machine:   "MY-PC",
		Timestamp: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	serialized := original.String()
	parsed, err := ParseLock(serialized)
	if err != nil {
		t.Fatalf("roundtrip parse: %v", err)
	}

	if parsed.Status != original.Status {
		t.Errorf("status roundtrip: got %q, want %q", parsed.Status, original.Status)
	}
	if parsed.Machine != original.Machine {
		t.Errorf("machine roundtrip: got %q, want %q", parsed.Machine, original.Machine)
	}
	if !parsed.Timestamp.Equal(original.Timestamp) {
		t.Errorf("timestamp roundtrip: got %v, want %v", parsed.Timestamp, original.Timestamp)
	}
}

// TestLockInfo_String_MatchesPythonFormat verifies output matches Python's format.
func TestLockInfo_String_MatchesPythonFormat(t *testing.T) {
	info := LockInfo{
		Status:    LockOnline,
		Machine:   "DESKTOP-ABC123",
		Timestamp: time.Date(2026, 3, 29, 15, 42, 30, 123456000, time.UTC),
	}

	got := info.String()
	want := "ONLINE|DESKTOP-ABC123|2026-03-29T15:42:30.123456"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestLockManager_InternalSessionEpoch_DoesNotChangeWireFormat(t *testing.T) {
	catalogDir := t.TempDir()
	mgr := NewLockManager(catalogDir)

	if mgr.SessionID() == "" {
		t.Fatal("session_id should be initialized")
	}
	if mgr.Epoch() != 0 {
		t.Fatalf("initial epoch = %d, want 0", mgr.Epoch())
	}

	ctx := context.Background()
	info1 := LockInfo{
		Status:    LockOnline,
		Machine:   "PC-1",
		Timestamp: time.Date(2026, 3, 30, 1, 2, 3, 0, time.UTC),
	}
	if err := mgr.WriteLock(ctx, info1); err != nil {
		t.Fatalf("WriteLock #1 failed: %v", err)
	}
	if got := mgr.Epoch(); got != 1 {
		t.Fatalf("epoch after write #1 = %d, want 1", got)
	}

	raw, err := os.ReadFile(filepath.Join(catalogDir, lockFileName))
	if err != nil {
		t.Fatalf("read lock file: %v", err)
	}
	line := strings.TrimSpace(string(raw))
	if strings.Count(line, "|") != 2 {
		t.Fatalf("wire format should stay 3-field legacy format, got %q", line)
	}
	if strings.Contains(strings.ToLower(line), "session") || strings.Contains(strings.ToLower(line), "epoch") {
		t.Fatalf("wire format leaked internal metadata: %q", line)
	}

	info2 := LockInfo{
		Status:    LockOffline,
		Machine:   "PC-1",
		Timestamp: time.Date(2026, 3, 30, 1, 3, 3, 0, time.UTC),
	}
	if err := mgr.WriteLock(ctx, info2); err != nil {
		t.Fatalf("WriteLock #2 failed: %v", err)
	}
	if got := mgr.Epoch(); got != 2 {
		t.Fatalf("epoch after write #2 = %d, want 2", got)
	}
}

func TestLockManager_TwoMachineContention_NoCorruption(t *testing.T) {
	catalogDir := t.TempDir()
	mgrA := NewLockManager(catalogDir)
	mgrB := NewLockManager(catalogDir)

	ctx := context.Background()
	writeErrors := make(chan error, 8)

	writeBurst := func(mgr *LockManager, machine string, offset int) {
		for i := 0; i < 50; i++ {
			info := LockInfo{
				Status:    LockOnline,
				Machine:   machine,
				Timestamp: time.Date(2026, 3, 30, 2, 0, 0, 0, time.UTC).Add(time.Duration(offset+i) * time.Millisecond),
			}
			if err := mgr.WriteLock(ctx, info); err != nil {
				writeErrors <- err
				return
			}

			raw, err := os.ReadFile(filepath.Join(catalogDir, lockFileName))
			if err != nil {
				if isTransientLockContentionErr(err) {
					continue
				}
				writeErrors <- err
				return
			}
			parsed, err := ParseLock(string(raw))
			if err != nil {
				writeErrors <- err
				return
			}
			if parsed.Machine != "PC-A" && parsed.Machine != "PC-B" {
				writeErrors <- fmt.Errorf("unexpected machine in lock file: %q", parsed.Machine)
				return
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		writeBurst(mgrA, "PC-A", 0)
	}()
	go func() {
		defer wg.Done()
		writeBurst(mgrB, "PC-B", 1000)
	}()
	wg.Wait()
	close(writeErrors)

	for err := range writeErrors {
		if err != nil {
			t.Fatalf("contention write/read failed: %v", err)
		}
	}

	finalInfo, err := mgrA.ReadLock(ctx)
	if err != nil {
		t.Fatalf("read final lock failed: %v", err)
	}
	if finalInfo == nil {
		t.Fatal("expected final lock file after contention")
	}
	if finalInfo.Status != LockOnline {
		t.Fatalf("final status = %q, want %q", finalInfo.Status, LockOnline)
	}
	if finalInfo.Machine != "PC-A" && finalInfo.Machine != "PC-B" {
		t.Fatalf("final machine = %q, want PC-A or PC-B", finalInfo.Machine)
	}
}

func isTransientLockContentionErr(err error) bool {
	if err == nil {
		return false
	}
	if os.IsNotExist(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "being used by another process") ||
		strings.Contains(msg, "access is denied")
}
