package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestPackageIdentitySupportsSSH(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires package identity")
	}
	if _, err := os.Stat("/usr/local/lib/libnss_wrapper.so"); err != nil {
		t.Skip("run in Ovis runtime image")
	}
	for _, key := range []string{"LD_PRELOAD", "NSS_WRAPPER_PASSWD", "NSS_WRAPPER_GROUP"} {
		t.Setenv(key, os.Getenv(key))
	}
	home := t.TempDir()
	if err := prepareIdentity(home); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Dir(os.Getenv("NSS_WRAPPER_PASSWD")))
	row, err := exec.Command("getent", "passwd", strconv.Itoa(os.Geteuid())).Output()
	if err != nil || !strings.Contains(string(row), ":"+home+":/bin/bash") {
		t.Fatalf("account lookup failed: %s %v", row, err)
	}
	// OpenSSH's account/config check must work even if /etc/passwd has no entry
	// for the assigned DSM package UID. No connection is attempted by -G.
	if err := os.Mkdir(filepath.Join(home, ".ssh"), 0700); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(home, ".ssh/config")
	if err := os.WriteFile(cfg, []byte("Host fixture\n HostName localhost\n User fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("ssh", "-G", "-F", cfg, "fixture").CombinedOutput()
	if err != nil || !strings.Contains(string(output), "hostname localhost") {
		t.Fatalf("SSH account lookup failed: %s %v", output, err)
	}
}
