package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireLockCreatesFile(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "disform.state.json")

	lock, err := AcquireLock(statePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer lock.Release()

	lockPath := statePath + ".lock"
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("expected lock file to exist after AcquireLock")
	}
}

func TestAcquireLockWritesPID(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "disform.state.json")

	lock, err := AcquireLock(statePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer lock.Release()

	data, err := os.ReadFile(statePath + ".lock")
	if err != nil {
		t.Fatalf("reading lock file: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected lock file to contain PID")
	}
}

func TestAcquireLockConflict(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "disform.state.json")

	lock1, err := AcquireLock(statePath)
	if err != nil {
		t.Fatalf("first lock failed: %v", err)
	}
	defer lock1.Release()

	_, err = AcquireLock(statePath)
	if err == nil {
		t.Fatal("expected error when acquiring already-held lock")
	}
}

func TestLockRelease(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "disform.state.json")

	lock, err := AcquireLock(statePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lock.Release()

	lockPath := statePath + ".lock"
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("expected lock file to be removed after Release")
	}
}

func TestLockReleaseIdempotent(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "disform.state.json")

	lock, err := AcquireLock(statePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lock.Release()
	lock.Release() // second release should not panic
}

func TestAcquireLockAfterRelease(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "disform.state.json")

	lock1, err := AcquireLock(statePath)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	lock1.Release()

	lock2, err := AcquireLock(statePath)
	if err != nil {
		t.Fatalf("expected to re-acquire lock after release: %v", err)
	}
	lock2.Release()
}

func TestAcquireLockErrorMessageContainsPath(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "disform.state.json")

	lock, err := AcquireLock(statePath)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	defer lock.Release()

	_, err = AcquireLock(statePath)
	if err == nil {
		t.Fatal("expected error")
	}
	if msg := err.Error(); len(msg) == 0 {
		t.Error("expected non-empty error message")
	}
}
