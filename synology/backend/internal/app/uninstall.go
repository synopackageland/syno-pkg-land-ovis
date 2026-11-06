package app

import (
	"os"
	"path/filepath"
)

// Uninstall is the App's purge hook (dsm-package.yaml uninstall.purge): DSM
// runs `api.cgi uninstall` as the package account before the executor is
// removed, with the admin's "delete state" answer. Release here what the App
// holds outside the package. With deleteState it deletes the state that
// uninstall.state lists, the background job spool; package data stays for
// DSM's own "remove data" option.
func Uninstall(dataDir string, deleteState bool) error {
	if !deleteState {
		return nil
	}
	return os.RemoveAll(filepath.Join(dataDir, "background"))
}
