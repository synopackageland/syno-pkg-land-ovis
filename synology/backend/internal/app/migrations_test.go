package app

import (
	"testing"

	"github.com/synopackageland/syno-pkg-land-sdk/datamigrate"
)

func TestMigrationsAreOrderedUniqueAndValid(t *testing.T) {
	migrations := Migrations()
	if len(migrations) == 0 {
		t.Fatal("the template must ship one example migration")
	}
	for i, migration := range migrations {
		if migration.Version < 1 || migration.Apply == nil || migration.Name == "" {
			t.Fatalf("migration %d is invalid: %+v", i, migration)
		}
		if i > 0 && migrations[i-1].Version >= migration.Version {
			t.Fatalf("migrations are not strictly version-ascending: %+v", migrations)
		}
	}
}

func TestMigrateRecordsTheSchemaAndPackageVersion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SYNOPKG_OLD_PKGVER", "0.1.0-0001")
	cfg := Config{DataDir: dir, Version: "0.2.0-0002"}
	if err := Migrate(cfg); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	state, err := datamigrate.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if state.Schema != 1 || state.PackageVersion != "0.2.0-0002" {
		t.Fatalf("recorded state = %+v", state)
	}
	if err := Migrate(cfg); err != nil {
		t.Fatalf("second Migrate must be a no-op: %v", err)
	}
}
