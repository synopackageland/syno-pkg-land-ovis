package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/synopackageland/syno-pkg-land-sdk/sharedfolder"
)

func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// sharedFixture has /volume1/Photos with the mode, chosen for key "media".
func sharedFixture(t *testing.T, mode os.FileMode) Config {
	t.Helper()
	cfg := testConfig(t)
	cfg.VolumeRoot = t.TempDir()
	dir := filepath.Join(cfg.VolumeRoot, "volume1", "Photos")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	writeConfig(t, cfg.DataDir, `{"schema":1,"shared_folders":{"media":"Photos"}}`)
	cfg.SharedFolderRequired = map[string]sharedfolder.Access{"media": sharedfolder.Write}
	return cfg
}

func skipRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory modes")
	}
}

func TestSharedFolderStatusIsEmptyWithoutAFolder(t *testing.T) {
	code, out := call(t, testConfig(t), alice, "sharedfolder.status", `{}`)
	if code != "" || len(out["data"].(map[string]any)["folders"].([]any)) != 0 {
		t.Fatalf("%q %v", code, out)
	}
}

func TestSharedFolderStatusReportsGrantedAndPending(t *testing.T) {
	skipRoot(t)
	for _, c := range []struct {
		mode  os.FileMode
		state string
	}{{0o755, "ready"}, {0o555, "pending"}} {
		cfg := sharedFixture(t, c.mode)
		code, out := call(t, cfg, alice, "sharedfolder.status", `{}`)
		if code != "" {
			t.Fatalf("%q %v", code, out)
		}
		row := out["data"].(map[string]any)["folders"].([]any)[0].(map[string]any)
		if row["state"] != c.state || row["required"] != "write" || row["name"] != "Photos" || row["path"] != filepath.Join(cfg.VolumeRoot, "volume1", "Photos") {
			t.Fatalf("mode %o: %v", c.mode, row)
		}
	}
}

func TestSharedFolderStatusUsesSDKAuthenticatedPolicy(t *testing.T) {
	cfg := sharedFixture(t, 0o755)
	// The SDK owns this read-only method's policy, including in jobs overlays.
	if code, _ := call(t, cfg, bob, "sharedfolder.status", `{}`); code != "" {
		t.Fatalf("a signed-in user could not read the status: %q", code)
	}
	if code, _ := call(t, cfg, nobody, "sharedfolder.status", `{}`); code != "session_required" {
		t.Fatalf("a signed-out caller got %q, want session_required", code)
	}
}

func TestAppStatusNoLongerCarriesSharedFolders(t *testing.T) {
	cfg := sharedFixture(t, 0o755)
	_, out := call(t, cfg, alice, "app.status", `{}`)
	data := out["data"].(map[string]any)
	if _, present := data["shared_folders"]; present {
		t.Fatalf("app.status: %v", data)
	}
}

func TestSharedFolderStatusFailsOnACorruptConfig(t *testing.T) {
	cfg := testConfig(t)
	writeConfig(t, cfg.DataDir, `not json`)
	if code, _ := call(t, cfg, alice, "sharedfolder.status", `{}`); code != "internal" {
		t.Fatalf("code %q", code)
	}
}

func TestStorageAdapterFollowsTheChosenFolderAndAMovedShare(t *testing.T) {
	skipRoot(t)
	cfg := sharedFixture(t, 0o755)
	var applied []string
	cfg.StorageAdapter = func(_ context.Context, key, path string) error {
		applied = append(applied, key+"="+path)
		return nil
	}
	first := filepath.Join(cfg.VolumeRoot, "volume1", "Photos")
	for i := 0; i < 2; i++ {
		if code, _ := call(t, cfg, alice, "sharedfolder.status", `{}`); code != "" {
			t.Fatal(code)
		}
	}
	if len(applied) != 1 || applied[0] != "media="+first {
		t.Fatalf("first reads apply once: %v", applied)
	}
	second := filepath.Join(cfg.VolumeRoot, "volume2", "Photos")
	if err := os.MkdirAll(filepath.Dir(second), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(first, second); err != nil {
		t.Fatal(err)
	}
	if code, _ := call(t, cfg, alice, "sharedfolder.status", `{}`); code != "" {
		t.Fatal(code)
	}
	if len(applied) != 2 || applied[1] != "media="+second {
		t.Fatalf("a moved share applies the new path: %v", applied)
	}
}

func TestStorageAdapterWaitsForThePermission(t *testing.T) {
	skipRoot(t)
	cfg := sharedFixture(t, 0o555)
	called := false
	cfg.StorageAdapter = func(context.Context, string, string) error { called = true; return nil }
	if code, _ := call(t, cfg, alice, "sharedfolder.status", `{}`); code != "" || called {
		t.Fatalf("pending folder: code %q called %v", code, called)
	}
}

func TestSharedFolderPathNamesWhatToGrant(t *testing.T) {
	skipRoot(t)
	_, err := sharedFixture(t, 0o555).SharedFolderPath("media")
	if !errors.Is(err, sharedfolder.ErrPermissionPending) {
		t.Fatalf("got %v", err)
	}
	if path, err := sharedFixture(t, 0o755).SharedFolderPath("media"); err != nil || path == "" {
		t.Fatalf("%q %v", path, err)
	}
}
