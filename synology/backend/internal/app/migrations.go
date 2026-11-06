package app

import (
	"os"

	"github.com/synopackageland/syno-pkg-land-sdk/datamigrate"
)

// Migrations returns this app's schema migrations. The template ships one
// empty example; replace it with your own steps. Versions start at 1 and never
// change meaning. Every step must be idempotent: a crash between Apply and the
// recorded version, or two CGI requests racing on the first run after an
// upgrade, can run one step twice. A failed step stops the run with the
// previous version still recorded, and the next run resumes from there; the
// utility itself never deletes or rewrites data.
func Migrations() []datamigrate.Migration {
	return []datamigrate.Migration{
		{
			Version: 1,
			Name:    "example",
			Apply: func(datamigrate.Context) error {
				// Nothing to do yet. Read ctx.DataDir, rewrite files with a
				// temporary file + rename, and keep the step idempotent.
				return nil
			},
		},
	}
}

// Migrate applies Migrations to the package's persistent var directory. The
// CGI calls it once per request; the recorded data-version makes that a cheap
// no-op. SYNOPKG_OLD_PKGVER is set when DSM starts the app from a lifecycle
// script; in a plain CGI request the version of the last successful run is
// used instead.
func Migrate(cfg Config) error {
	_, err := datamigrate.Run(cfg.DataDir, datamigrate.Info{
		OldPkgVer: os.Getenv("SYNOPKG_OLD_PKGVER"),
		NewPkgVer: cfg.Version,
	}, Migrations())
	return err
}
