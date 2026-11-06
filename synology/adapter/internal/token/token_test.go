package token

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCreatesPrivateTokenOnceAndKeepsIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "var", file)
	first, err := Ensure(path)
	if err != nil {
		t.Fatal(err)
	}
	if !valid(first) {
		t.Fatalf("token %q is not 64 hex digits", first)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("token mode %v, want 0600", info.Mode().Perm())
	}
	dir, _ := os.Stat(filepath.Dir(path))
	if dir.Mode().Perm() != 0o700 {
		t.Fatalf("token folder mode %v, want 0700", dir.Mode().Perm())
	}
	again, err := Ensure(path)
	if err != nil || again != first {
		t.Fatalf("second start changed the token: %q %v", again, err)
	}
}

func TestReadRefusesLooseOrMalformedFiles(t *testing.T) {
	dir := t.TempDir()
	loose := filepath.Join(dir, "loose")
	os.WriteFile(loose, []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n"), 0o644)
	if _, err := Read(loose); err == nil {
		t.Fatal("a group-readable token was accepted")
	}
	bad := filepath.Join(dir, "bad")
	os.WriteFile(bad, []byte("short\n"), 0o600)
	if _, err := Read(bad); err == nil {
		t.Fatal("a malformed token was accepted")
	}
	if _, err := Ensure(bad); err == nil {
		t.Fatal("Ensure replaced an existing malformed token instead of refusing")
	}
}

func TestPathFollowsPackageVar(t *testing.T) {
	env := map[string]string{"SYNOPKG_PKGVAR": "/volume1/@appdata/Ovis"}
	if got := Path(func(k string) string { return env[k] }); got != "/volume1/@appdata/Ovis/gateway/ovis-token" {
		t.Fatalf("Path = %s", got)
	}
	if got := Path(func(string) string { return "" }); got != "/var/packages/Ovis/var/gateway/ovis-token" {
		t.Fatalf("fallback Path = %s", got)
	}
}
