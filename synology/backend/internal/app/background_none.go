package app

import "github.com/synopackageland/syno-pkg-land-sdk/api"

// backgroundRoutes is empty: this package did not opt in to background jobs.
// `synopkgland init --background` replaces this file with background_on.go,
// which serves BackgroundRoutes for the package-user worker unit.
func (Config) backgroundRoutes() []api.Route { return nil }
