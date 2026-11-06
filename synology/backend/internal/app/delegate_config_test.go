package app

import "testing"

func TestDelegateBuildConfigStrictlySeparatesLegacyAndPermanent(t *testing.T) {
	valid := Config{DelegateScope: "home", DelegateOperationsJSON: `["files.moveFromStaging"]`, DelegateDestinationsJSON: `["save"]`, DelegateLifetime: "until-revoked"}
	settings, err := ParseDelegateConfig(valid)
	if err != nil || settings.Lifetime != "until-revoked" || len(settings.Destinations) != 1 {
		t.Fatal(settings, err)
	}
	for _, cfg := range []Config{{}, {DelegateScope: "home"}, {DelegateScope: "home", DelegateOperationsJSON: `["files.write"]`}} {
		settings, err = ParseDelegateConfig(cfg)
		if err != nil || settings.Lifetime != "" || len(settings.Destinations) != 0 {
			t.Fatal("legacy changed", settings, err)
		}
	}
	for _, mutate := range []func(*Config){
		func(c *Config) { c.DelegateLifetime = "" }, func(c *Config) { c.DelegateScope = "" }, func(c *Config) { c.DelegateOperationsJSON = `["files.write"]` },
		func(c *Config) { c.DelegateDestinationsJSON = `["save","save"]` }, func(c *Config) { c.DelegateDestinationsJSON = `["Save"]` }, func(c *Config) { c.DelegateDestinationsJSON = `[]` }, func(c *Config) { c.DelegateDestinationsJSON = `null` },
		func(c *Config) { c.DelegateDestinationsJSON = `["save"] {}` },
	} {
		cfg := valid
		mutate(&cfg)
		if _, err = ParseDelegateConfig(cfg); err == nil {
			t.Fatal("invalid trusted config accepted", cfg)
		}
	}
}

func TestDelegateBuildConfigAcceptsEveryTimeLimitedFileOperation(t *testing.T) {
	ops := `["files.checkPermission","files.copy","files.createFolder","files.delete","files.info","files.list","files.move","files.read","files.rename","files.write"]`
	settings, err := ParseDelegateConfig(Config{DelegateScope: "shared:media", DelegateOperationsJSON: ops, DelegateDestinationsJSON: `[]`})
	if err != nil || len(settings.Operations) != 10 || settings.Lifetime != "" {
		t.Fatal(settings, err)
	}
}
