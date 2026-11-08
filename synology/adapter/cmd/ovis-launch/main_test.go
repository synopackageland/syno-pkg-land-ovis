package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestWithTokenReplacesInheritedTokens(t *testing.T) {
	env := withToken([]string{"PATH=/usr/bin:/bin", "OVIS_TOKEN=old", "OVIS_TOKENS=keep"}, "new")
	want := []string{"PATH=/usr/bin:/bin", "OVIS_TOKENS=keep", "OVIS_TOKEN=new"}
	if !slices.Equal(env, want) {
		t.Fatalf("env = %v, want %v", env, want)
	}
}

func TestTelemetryDefaultOffMarksAFirstStart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ovis")
	if err := telemetryDefaultOff(dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "telemetry-off"))
	if err != nil {
		t.Fatalf("no off mark: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mark mode = %v", info.Mode().Perm())
	}
}

func TestTelemetryDefaultOffKeepsTheUsersChoice(t *testing.T) {
	dir := t.TempDir()
	// the user turned counts on: upstream saved telemetry.json and removed the mark
	if err := os.WriteFile(filepath.Join(dir, "telemetry.json"), []byte(`{"enabled":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := telemetryDefaultOff(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "telemetry-off")); !os.IsNotExist(err) {
		t.Fatalf("mark was added over the user's choice: %v", err)
	}
}

func TestTelemetryDefaultOffWithoutStateDir(t *testing.T) {
	if err := telemetryDefaultOff(""); err != nil {
		t.Fatal(err)
	}
}

func TestHerdrConfigHomeFollowsTheSocket(t *testing.T) {
	home, err := herdrConfigHome("/var/packages/Ovis/var/herdr/herdr.sock")
	if err != nil || home != "/var/packages/Ovis/var" {
		t.Fatalf("home = %q, %v", home, err)
	}
	for _, bad := range []string{"", "relative/herdr/herdr.sock", "/var/x/herdr.sock", "/var/herdr/other.sock"} {
		if _, err := herdrConfigHome(bad); err == nil {
			t.Fatalf("%q was accepted", bad)
		}
	}
}

func TestHerdrEnvDropsTokensAndPaneVariables(t *testing.T) {
	env := herdrEnv([]string{"PATH=/usr/bin:/bin", "HOME=/v", "OVIS_TOKEN=x", "HERDR_ENV=1", "HERDR_SOCKET=/v/herdr/herdr.sock", "XDG_CONFIG_HOME=/elsewhere", "SHELL=/sbin/nologin", "OVIS_STATE_DIR=/v/ovis"}, "/v")
	for _, kv := range env {
		if hasKey(kv, "OVIS_TOKEN") || hasKey(kv, "HERDR_ENV") || hasKey(kv, "HERDR_SOCKET") || kv == "XDG_CONFIG_HOME=/elsewhere" || kv == "SHELL=/sbin/nologin" {
			t.Fatalf("herdr env keeps %q: %v", kv, env)
		}
	}
	if !slices.Contains(env, "XDG_CONFIG_HOME=/v") || !slices.Contains(env, "SHELL="+paneShell()) || !slices.Contains(env, "HOME=/v") {
		t.Fatalf("herdr env = %v", env)
	}
}

func TestOvisEnvNamesTheBundledHerdr(t *testing.T) {
	env := ovisEnv([]string{"PATH=/usr/bin:/bin", "OVIS_HERDR_BIN=/usr/bin/herdr", "HERDR_SOCKET=/v/herdr/herdr.sock"}, "tok", "/pkg/herdr")
	want := []string{"PATH=/usr/bin:/bin", "HERDR_SOCKET=/v/herdr/herdr.sock", "OVIS_TOKEN=tok", "OVIS_HERDR_BIN=/pkg/herdr"}
	if !slices.Equal(env, want) {
		t.Fatalf("env = %v, want %v", env, want)
	}
}

// a fake herdr: `server` counts its starts, then sleeps or exits as told
func fakeHerdr(t *testing.T, body string) (*herdrChild, string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "herdr")
	script := "#!/bin/sh\n[ \"$1\" = server ] || exit 0\necho x >> \"" + dir + "/starts\"\n" + body + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return &herdrChild{bin: bin, env: []string{"PATH=/usr/bin:/bin"}, socket: filepath.Join(dir, "var", "herdr", "herdr.sock")}, filepath.Join(dir, "starts")
}

func starts(t *testing.T, file string) int {
	data, _ := os.ReadFile(file)
	return len(data) / 2
}

func TestHerdrChildStopsOnStop(t *testing.T) {
	h, file := fakeHerdr(t, "exec sleep 60")
	if err := h.start(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100 && starts(t, file) == 0; i++ {
		time.Sleep(50 * time.Millisecond)
	}
	begin := time.Now()
	h.stop()
	h.wait()
	if time.Since(begin) > 5*time.Second || starts(t, file) != 1 {
		t.Fatalf("stop took %s, %d starts", time.Since(begin), starts(t, file))
	}
}

func TestHerdrChildRestartsAfterAnExit(t *testing.T) {
	h, file := fakeHerdr(t, "exit 3")
	if err := h.start(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100 && starts(t, file) < 2; i++ {
		time.Sleep(50 * time.Millisecond)
	}
	h.hold()
	h.stop()
	h.wait()
	if n := starts(t, file); n < 2 {
		t.Fatalf("herdr was started %d times, want a restart", n)
	}
}
