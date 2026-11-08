package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestRootHomeMigrationRetainsDataAndDoesNotFollowLinks(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("root ownership fixture")
	}
	home, outside := t.TempDir(), t.TempDir()
	inside := filepath.Join(home, "repository", ".git")
	os.MkdirAll(inside, 0700)
	data := filepath.Join(inside, "config")
	os.WriteFile(data, []byte("retained repository"), 0600)
	foreign := filepath.Join(outside, "retained")
	os.WriteFile(foreign, []byte("outside"), 0600)
	for _, p := range []string{home, inside, data, foreign} {
		if err := os.Chown(p, 12345, 12345); err != nil {
			t.Skip("fixture needs chown")
		}
	}
	os.Symlink(outside, filepath.Join(home, "outside"))
	if err := migrateHomeToRoot(home); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{home, inside, data} {
		st, _ := os.Stat(p)
		if st.Sys().(*syscall.Stat_t).Uid != 0 {
			t.Fatal("legacy owner retained", p)
		}
	}
	st, _ := os.Stat(foreign)
	if st.Sys().(*syscall.Stat_t).Uid != 12345 {
		t.Fatal("followed a symlink out of HOME")
	}
	b, _ := os.ReadFile(data)
	if string(b) != "retained repository" {
		t.Fatal("data changed")
	}
	before, _ := os.Stat(filepath.Join(home, ".ovis-root-home-v1"))
	if err := migrateHomeToRoot(home); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(filepath.Join(home, ".ovis-root-home-v1"))
	if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("migration repeated on restart")
	}
}
