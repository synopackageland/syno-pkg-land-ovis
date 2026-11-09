package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// fixPermissions is the SDK's in-container command that removes group/other
// access from the package UID's credential paths below HOME (the same table
// its init applies on every start). Tests replace it.
var fixPermissions = []string{"/synopkgland/bin/synopkgland", "container", "fix-permissions"}

// prepareSSH retains existing user keys and creates a package key only on a
// first start as the package UID.
func prepareSSH(home string) error {
	if !filepath.IsAbs(home) {
		return fmt.Errorf("SSH setup requires an absolute HOME")
	}
	dir := filepath.Join(home, ".ssh")
	if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	if st, err := os.Lstat(dir); err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("SSH setup requires a private .ssh directory without symlinks")
	}
	key := filepath.Join(dir, "id_ed25519")
	if st, err := os.Lstat(key); err == nil {
		if !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
			return fmt.Errorf("SSH private key must be a private regular file")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	// Generate in a private directory so ssh-keygen never sees an existing key
	// and never prompts to overwrite it. Hard links publish without replacement.
	stage, err := os.MkdirTemp(dir, ".key-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	tmp := filepath.Join(stage, "id_ed25519")
	cmd := exec.Command("/usr/bin/ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "Ovis remote PCs", "-f", tmp)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("SSH key creation failed: %w", err)
	}
	if err := os.Link(tmp, key); err != nil {
		return err
	}
	if err := os.Link(tmp+".pub", key+".pub"); err != nil {
		return fmt.Errorf("SSH public key could not be saved; recover it with ssh-keygen -y: %w", err)
	}
	// A folder ACL can widen the modes mkdir and ssh-keygen asked for; the init
	// ran before .ssh existed, so apply its table once more now.
	fix := exec.Command(fixPermissions[0], fixPermissions[1:]...)
	fix.Stdout, fix.Stderr = os.Stderr, os.Stderr
	if err := fix.Run(); err != nil {
		return fmt.Errorf("SSH key permissions: %w", err)
	}
	return nil
}
