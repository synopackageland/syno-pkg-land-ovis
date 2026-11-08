package main

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

// migrateHomeToRoot adopts only this App's named-volume home, once. WalkDir
// never follows symlinks, Root confines parent traversal, Lchown changes the
// entry itself, and filesystem boundaries are refused. A partial failure leaves
// no completion marker so the next start resumes the idempotent walk.
func migrateHomeToRoot(home string) error {
	if os.Geteuid() != 0 {
		return nil
	}
	st, err := os.Lstat(home)
	if err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("Ovis HOME must be a real private directory")
	}
	root, err := os.OpenRoot(home)
	if err != nil {
		return err
	}
	defer root.Close()
	const marker = ".ovis-root-home-v1"
	if mark, err := root.Lstat(marker); err == nil {
		owner, ok := mark.Sys().(*syscall.Stat_t)
		if !mark.Mode().IsRegular() || !ok || owner.Uid != 0 || owner.Gid != 0 || mark.Mode().Perm() != 0600 || mark.Size() != 2 {
			return fmt.Errorf("Ovis home migration marker is invalid")
		}
		raw, err := root.ReadFile(marker)
		if err != nil || string(raw) != "1\n" {
			return fmt.Errorf("Ovis home migration marker is invalid")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	device := st.Sys().(*syscall.Stat_t).Dev
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		owner, ok := info.Sys().(*syscall.Stat_t)
		if !ok || owner.Dev != device {
			return fmt.Errorf("Ovis home migration cannot cross a filesystem boundary")
		}
		if owner.Uid == 0 && owner.Gid == 0 {
			return nil
		}
		return root.Lchown(name, 0, 0)
	})
	if err != nil {
		return fmt.Errorf("Ovis home ownership migration failed: %w", err)
	}
	f, err := root.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err = f.WriteString("1\n"); err == nil {
		err = f.Sync()
	}
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil {
		root.Remove(marker)
		return err
	}
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
