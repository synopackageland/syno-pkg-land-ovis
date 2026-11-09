package main

import (
	"crypto/rand"
	"fmt"
	"os"
)

// The selected folder already exists and DSM grants access. Create only this
// App's two children as the package UID, without changing the selected base or
// adopting other users' directories.
func prepareHome(base string) error {
	if os.Geteuid() == 0 {
		return fmt.Errorf("Ovis requires the nonroot package UID")
	}
	st, err := os.Lstat(base)
	if err != nil {
		return err
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("selected data folder must be a real directory")
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, name := range []string{"home", "workspace"} {
		if err = root.Mkdir(name, 0700); err != nil && !os.IsExist(err) {
			return fmt.Errorf("create %s: %w", name, err)
		}
		info, err := root.Lstat(name)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s must be a real directory", name)
		}
		// Exclusive creation tests actual package-account write access, including ACLs.
		dir, err := root.OpenRoot(name)
		if err != nil {
			return err
		}
		probeName := ".ovis-write-" + rand.Text()
		probe, err := dir.OpenFile(probeName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			dir.Close()
			return fmt.Errorf("%s must be writable by the package account: %w", name, err)
		}
		_, err = probe.WriteString("writable\n")
		if err == nil {
			err = probe.Sync()
		}
		closeErr := probe.Close()
		if err == nil {
			err = closeErr
		}
		removeErr := dir.Remove(probeName)
		if err == nil {
			err = removeErr
		}
		dir.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
