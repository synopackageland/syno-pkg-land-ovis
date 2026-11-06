// Package app holds the business handlers. Declare each operation once in
// operations.json; ctl generate writes the policy and SDK runtime registry.
//
//line Ovis/internal/app/routes.go:1
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/synopackageland/syno-pkg-land-sdk/api"
	"github.com/synopackageland/syno-pkg-land-sdk/filestation"
	"github.com/synopackageland/syno-pkg-land-sdk/sharedfolder"
)

// Config is fixed at build time and startup, never taken from a request.
type Config struct {
	DelegateScope                                                      string
	DelegateOperationsJSON, DelegateDestinationsJSON, DelegateLifetime string
	ExecutorDigest, PolicyDigest                                       string

	// SessionFiles is the interactive File Station adapter; nil keeps legacy refusal.
	SessionFiles *filestation.Service
	PackageID    string
	Version      string
	// DataDir is the package's persistent, package-owned directory
	// (/var/packages/<package>/var). It survives upgrades.
	DataDir string
	// VolumeRoot is where /volumeN lives; empty means "/". Tests set it.
	VolumeRoot string
	// SharedFolderRequired is the access each Shared Folder key needs. Keep it
	// equal to install.sharedFolder.permission in dsm-package.yaml: ro is
	// sharedfolder.Read, rw is sharedfolder.Write.
	SharedFolderRequired map[string]sharedfolder.Access
	// StorageAdapter, when set, hands a Shared Folder's path to the App's own
	// storage settings (a Local storage, a library root): the first time the
	// folder is ready, and again whenever its path changes. Make it idempotent.
	StorageAdapter func(ctx context.Context, key, path string) error
	// Portal serves portal.status; PortalExecutor adds the executor's entry
	// below DSM 7.2. PortalConfig overrides DSM's saved AppPortal.json path in
	// tests.
	Portal         bool
	PortalExecutor bool
	PortalConfig   string
}

func Routes(cfg Config) []api.Route {
	routes := cfg.delegateRoutes()
	routes = append(routes, []api.Route{
		api.Handle("app.status", cfg.status),
		api.Handle("sharedfolder.status", cfg.sharedFolderStatus),
	}...)
	routes = append(routes, cfg.SessionFiles.Routes()...)
	routes = append(routes, cfg.backgroundRoutes()...)
	routes = append(routes, cfg.portalRoutes()...)
	return append(routes, cfg.rootRoutes()...)
}

func (cfg Config) status(_ context.Context, caller api.Caller, _ json.RawMessage) (any, error) {
	return map[string]any{
		"package":    cfg.PackageID,
		"version":    cfg.Version,
		"user":       caller.User(),
		"admin":      caller.Admin(),
		"process":    map[string]int{"uid": os.Getuid(), "gid": os.Getgid()},
		"go_version": runtime.Version(),
		"platform":   runtime.GOOS + "/" + runtime.GOARCH,
		"time":       time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// decodeParams decodes params strictly: unknown fields are an error.
func decodeParams(params json.RawMessage, into any) error {
	if len(params) == 0 {
		params = json.RawMessage(`{}`)
	}
	dec := json.NewDecoder(bytes.NewReader(params))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return api.BadRequest("invalid params: " + err.Error())
	}
	return nil
}

// writeFileAtomic replaces path in one rename, so readers never see half a file.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
