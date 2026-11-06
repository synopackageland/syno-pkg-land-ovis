package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/synopackageland/syno-pkg-land-sdk/api"
	"github.com/synopackageland/syno-pkg-land-sdk/delegate"
	"github.com/synopackageland/syno-pkg-land-sdk/portal"
)

// portalRoutes serves portal.status when dsm-package.yaml declares `portal`.
// An administrator sets the alias with DSM's own AppPortal API from the UI;
// the package only reads DSM's saved setting. Below DSM 7.2 the executor
// registers the App's entry (PortalExecutor): portal.status then reports it,
// and portal.register and portal.unregister change it.
func (cfg Config) portalRoutes() []api.Route {
	if !cfg.Portal {
		return nil
	}
	routes := []api.Route{api.Handle("portal.status", cfg.portalStatus)}
	if cfg.PortalExecutor {
		routes = append(routes,
			api.HandleRequest("portal.register", cfg.portalChange(portal.Register)),
			api.HandleRequest("portal.unregister", cfg.portalChange(portal.Unregister)))
	}
	return routes
}

// portalExecutor is a variable so tests can replace the executor.
var portalExecutor = func(cfg Config) portal.Executor { return delegate.Local(cfg.PackageID) }

func (cfg Config) portalStatus(ctx context.Context, _ api.Caller, params json.RawMessage) (any, error) {
	if err := decodeParams(params, &struct{}{}); err != nil {
		return nil, err
	}
	return cfg.readPortal(ctx), nil
}

func (cfg Config) readPortal(ctx context.Context) portal.Status {
	path := cfg.PortalConfig
	if path == "" {
		path = portal.ConfigPath(cfg.PackageID)
	}
	s := portal.Read(path, "SYNO.SDS."+cfg.PackageID+".Instance")
	if !cfg.PortalExecutor {
		return s
	}
	// An unreadable entry is not a known one: WithEntry reports unavailable.
	entry, _ := portal.ExecutorEntry(ctx, portalExecutor(cfg))
	return portal.WithEntry(s, entry)
}

// portalChange forwards the administrator's DSM session; the executor checks
// it again and keeps its refusal codes, such as PORTAL_FAILED.
func (cfg Config) portalChange(change func(context.Context, portal.Executor, map[string]string) (string, error)) api.RequestHandler {
	return func(ctx context.Context, caller api.Caller, req *http.Request, params json.RawMessage) (any, error) {
		if !caller.Admin() {
			return nil, errors.New("admin_required")
		}
		if err := decodeParams(params, &struct{}{}); err != nil {
			return nil, err
		}
		auth, err := api.DSMSession(req)
		if err != nil {
			return nil, err
		}
		if _, err := change(ctx, portalExecutor(cfg), auth); err != nil {
			return nil, err
		}
		return cfg.readPortal(ctx), nil
	}
}
