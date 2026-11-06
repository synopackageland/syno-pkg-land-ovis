package app

import (
	"context"
	"encoding/json"

	"github.com/synopackageland/syno-pkg-land-sdk/api"
	"github.com/synopackageland/syno-pkg-land-sdk/sharedfolder"
)

// The install wizard stores the chosen Shared Folder's *name* in
// <DataDir>/config.json (see dsm-package.yaml install.sharedFolder). DSM does
// not give the package account access to a folder chosen at install time: an
// administrator grants it in Control Panel (or with one click in the settings
// page). The SDK's sharedfolder package measures what the account may do, so
// this route, the settings page and any daemon read one state.

// sharedFolders is the SDK's view of this package's Shared Folders.
func (cfg Config) sharedFolders() sharedfolder.Config {
	return sharedfolder.Config{DataDir: cfg.DataDir, Root: cfg.VolumeRoot, Required: cfg.SharedFolderRequired}
}

// sharedFolderStatus answers the settings page: each folder, what the package
// account has and what the App declared it needs, and whether an
// administrator still has to grant. The SDK's authenticated policy lets any
// signed-in DSM user read this status; granting package access stays an
// administrator operation.
func (cfg Config) sharedFolderStatus(ctx context.Context, _ api.Caller, _ json.RawMessage) (any, error) {
	status, err := cfg.sharedFolders().Status()
	if err != nil {
		return nil, err
	}
	// The App's own storage follows a changed path (another folder chosen,
	// the share moved to another volume) here, the first time anything reads it.
	if cfg.StorageAdapter != nil {
		for _, folder := range status.Folders {
			if folder.State == sharedfolder.Ready {
				if _, err := cfg.sharedFolders().Sync(folder.Key, func(path string) error {
					return cfg.StorageAdapter(ctx, folder.Key, path)
				}); err != nil {
					return nil, sharedfolder.APIError(err)
				}
			}
		}
	}
	return status, nil
}

// SharedFolderPath is what a daemon calls before it touches the folder: the
// path once the account has the declared access, otherwise an error naming
// what to grant, never a failed file read.
func (cfg Config) SharedFolderPath(key string) (string, error) {
	return cfg.sharedFolders().Require(key)
}
