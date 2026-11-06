package app

import "testing"

func TestIdentityApplicationPath(t *testing.T) {
	for _, tc := range []struct {
		script string
		valid  bool
	}{{"/webman/3rdparty/test/cgi/api.cgi", true}, {"/cgi/api.cgi", true}, {"", false}, {"cgi/api.cgi", false}, {"//evil/cgi/api.cgi", false}} {
		_, err := StandaloneIdentity(Config{DataDir: t.TempDir()}, tc.script)
		if (err == nil) != tc.valid {
			t.Errorf("script %q: %v", tc.script, err)
		}
	}
}
