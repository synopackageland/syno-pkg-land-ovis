package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// prepareSSH retains existing user keys and creates a package key only on a
// first start as the package UID.
func prepareSSH(home string) error {
	if !filepath.IsAbs(home) {
		return fmt.Errorf("SSH setup requires an absolute HOME")
	}
	dir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(home, 0700); err != nil {
		return err
	}
	if info, err := os.Lstat(home); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("SSH setup requires a real HOME directory")
	}
	if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	st, err := os.Lstat(dir)
	if err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("SSH setup requires a private .ssh directory without symlinks")
	}
	// DSM ACL inheritance can leave mkdir(0700) reporting 0777. Narrow only
	// this package-owned directory, through a no-follow descriptor. Never adopt
	// another account's directory or change the selected NAS folder's ACL.
	private, err := os.OpenFile(dir, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	st, err = private.Stat()
	if err != nil {
		private.Close()
		return err
	}
	owner, ok := st.Sys().(*syscall.Stat_t)
	if !ok || owner.Uid != uint32(os.Geteuid()) {
		private.Close()
		return fmt.Errorf("SSH directory must belong to the package account")
	}
	if err = private.Chmod(0700); err != nil {
		private.Close()
		return fmt.Errorf("SSH private directory permissions: %w", err)
	}
	st, err = private.Stat()
	private.Close()
	if err != nil || st.Mode().Perm() != 0700 {
		return fmt.Errorf("SSH directory must enforce owner-only permissions")
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
	return nil
}
