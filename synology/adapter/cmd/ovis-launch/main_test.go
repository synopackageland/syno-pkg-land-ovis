package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestWithTokenReplacesInheritedTokens(t *testing.T) {
	env := withToken([]string{"PATH=/usr/bin:/bin", "OVIS_TOKEN=old", "HERDR_WEB_TOKEN=older", "OVIS_TOKENS=keep"}, "new")
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
