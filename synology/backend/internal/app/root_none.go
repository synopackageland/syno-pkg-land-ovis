package app

import "github.com/synopackageland/syno-pkg-land-sdk/api"

// rootRoutes is empty: this package declares privilege: none and never runs
// anything as root. `synopkgland init -t native-go --privilege root` creates
// root.go instead, which reads the results of administrator-approved root
// operations.
func (Config) rootRoutes() []api.Route { return nil }
