package app

import (
	"errors"
	"path"
	"strings"

	"github.com/synopackageland/syno-pkg-land-sdk/identity"
)

// StandaloneIdentity registers identity.status/session/login/logout through
// Service.Wrap. Business routes keep their permissions.json policies. Call only
// for capabilities.identity=standalone; the path comes from the CGI environment.
func StandaloneIdentity(cfg Config, scriptName string) (*identity.Service, error) {
	if !strings.HasPrefix(scriptName, "/") || strings.HasPrefix(scriptName, "//") || path.Clean(scriptName) != scriptName || !strings.HasSuffix(scriptName, "/cgi/api.cgi") {
		return nil, errors.New("identity requires an absolute CGI script path")
	}
	cookiePath := strings.TrimSuffix(path.Dir(path.Dir(scriptName)), "/") + "/"
	return identity.New(identity.Config{DataDir: cfg.DataDir, CookiePath: cookiePath})
}
