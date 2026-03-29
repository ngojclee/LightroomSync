package monitor

import (
	"os"
	"path/filepath"
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
