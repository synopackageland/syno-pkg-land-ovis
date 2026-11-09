package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSSHHomeCreatesRetainedKeyWithoutOverwriting(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen unavailable")
	}
	home := t.TempDir()
	if err := prepareSSH(home); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(home, ".ssh", "id_ed25519")
	before, err := os.ReadFile(key)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(key)
	if st.Mode().Perm() != 0600 {
		t.Fatal(st.Mode())
	}
	if err := prepareSSH(home); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(key)
	if string(before) != string(after) {
		t.Fatal("key changed on restart")
	}
	pub, err := os.ReadFile(key + ".pub")
	if err != nil || len(pub) == 0 {
		t.Fatal("public key missing", err)
	}
}

func TestSSHHomeRejectsLinksAndRetainsExistingKeys(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	os.Symlink(outside, filepath.Join(home, ".ssh"))
	if err := prepareSSH(home); err == nil {
		t.Fatal("SSH directory symlink accepted")
	}
	os.Remove(filepath.Join(home, ".ssh"))
	os.Mkdir(filepath.Join(home, ".ssh"), 0700)
	os.WriteFile(filepath.Join(home, ".ssh", "id_ed25519"), []byte("existing user key"), 0600)
	if err := prepareSSH(home); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(home, ".ssh", "id_ed25519"))
	if string(b) != "existing user key" {
		t.Fatal("existing private key changed")
	}
}

func TestSSHDirectoryNarrowsInheritedPermissions(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen unavailable")
	}
	home := t.TempDir()
	dir := filepath.Join(home, ".ssh")
	if err := os.Mkdir(dir, 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatal(err)
	}
	if err := prepareSSH(home); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(dir)
	if err != nil || st.Mode().Perm() != 0700 {
		t.Fatal("SSH directory is not private", err)
	}
	key, err := os.Stat(filepath.Join(dir, "id_ed25519"))
	if err != nil || key.Mode().Perm() != 0600 {
		t.Fatal("SSH key is not private", err)
	}
}
