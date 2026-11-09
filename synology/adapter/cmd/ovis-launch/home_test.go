package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackageHomeAndWorkspace(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("run as nonroot runtime identity")
	}
	base := t.TempDir()
	if err := prepareHome(base); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"home", "workspace"} {
		st, err := os.Stat(filepath.Join(base, name))
		if err != nil || st.Mode().Perm() != 0700 {
			t.Fatal(name, st, err)
		}
	}
	key := filepath.Join(base, "home", "retained-file")
	os.WriteFile(key, []byte("existing selection contents"), 0600)
	if err := prepareHome(base); err != nil {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(key); string(raw) != "existing selection contents" {
		t.Fatal("existing file changed")
	}
}
func TestPackageHomeRejectsLinks(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("run as nonroot runtime identity")
	}
	base, outside := t.TempDir(), t.TempDir()
	os.Symlink(outside, filepath.Join(base, "home"))
	if err := prepareHome(base); err == nil {
		t.Fatal("HOME link accepted")
	}
}
