package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUninstallDeletesTheJobSpoolOnlyWhenAsked(t *testing.T) {
	dir := t.TempDir()
	spool := filepath.Join(dir, "background")
	if err := os.MkdirAll(filepath.Join(spool, "inputs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(dir, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(spool); err != nil {
		t.Fatalf("kept state is gone: %v", err)
	}
	if err := Uninstall(dir, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(spool); !os.IsNotExist(err) {
		t.Fatalf("state not deleted: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("package data must stay: %v", err)
	}
	if err := Uninstall(dir, true); err != nil {
		t.Fatalf("nothing left to delete: %v", err)
	}
}
