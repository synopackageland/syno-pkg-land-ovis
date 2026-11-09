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
	fakeFixer(t, filepath.Join(t.TempDir(), "fixed"))
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

func TestSSHRunsFixPermissionsOnlyAfterCreatingAKey(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen unavailable")
	}
	home := t.TempDir()
	marker := filepath.Join(t.TempDir(), "fixed")
	fakeFixer(t, marker)
	if err := prepareSSH(home); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("fix-permissions did not run after key creation")
	}
	os.Remove(marker)
	if err := prepareSSH(home); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("fix-permissions ran again for an existing key; the init covers later starts")
	}
}

func TestSSHReportsAFailedFixPermissions(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen unavailable")
	}
	saved := fixPermissions
	fixPermissions = []string{"/bin/false"}
	t.Cleanup(func() { fixPermissions = saved })
	if err := prepareSSH(t.TempDir()); err == nil {
		t.Fatal("a failed fix-permissions was ignored")
	}
}

// fakeFixer stands in for the SDK's in-container command, which tests do not have.
func fakeFixer(t *testing.T, marker string) {
	t.Helper()
	saved := fixPermissions
	fixPermissions = []string{"/bin/sh", "-c", "touch " + marker}
	t.Cleanup(func() { fixPermissions = saved })
}
