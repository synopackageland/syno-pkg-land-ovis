package main

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"ovis-dsm-adapter/internal/token"
)

func TestReplyInjectsBearerToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway", "ovis-token")
	want, err := token.Ensure(path)
	if err != nil {
		t.Fatal(err)
	}
	out, err := reply(path)
	if err != nil {
		t.Fatal(err)
	}
	var got struct{ Headers map[string][]string }
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Headers) != 1 || len(got.Headers["Authorization"]) != 1 || got.Headers["Authorization"][0] != "Bearer "+want {
		t.Fatalf("headers = %v", got.Headers)
	}
}

func TestReplyRefusesMissingToken(t *testing.T) {
	if _, err := reply(filepath.Join(t.TempDir(), "none")); err == nil {
		t.Fatal("a missing token produced credentials")
	}
}

func TestSecretPathUsesInstalledComposeSeed(t *testing.T) {
	t.Setenv("SYNOPKG_PKGDEST", "/opt/package")
	if got := secretPath(); got != "/opt/package/compose/secrets/runtime-ovis.secret" {
		t.Fatalf("secret path = %s", got)
	}
	t.Setenv("SYNOPKG_PKGDEST", "")
	if got := secretPath(); got != "/var/packages/Ovis/target/compose/secrets/runtime-ovis.secret" {
		t.Fatalf("fallback secret path = %s", got)
	}
}
