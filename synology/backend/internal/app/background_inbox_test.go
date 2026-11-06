package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/synopackageland/syno-pkg-land-sdk/background"
)

func TestBackgroundSubmitRefusesOtherOwnerInboxAndAppFiles(t *testing.T) {
	e := ordinaryEnv(t)
	private := filepath.Join(e.dataDir, "private-settings")
	if err := os.WriteFile(private, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{private, e.inputPath("alice")} {
		p, _ := json.Marshal(map[string]string{"key": "stolen", "name": "copy", "input": input})
		if code, _ := e.call(bob, "bg.jobs.submit", string(p)); code != "bad_request" {
			t.Fatal("cross-owner import accepted", code)
		}
	}
	own := e.inputPath("bob")
	if err := os.Symlink(private, filepath.Join(filepath.Dir(own), "escape")); err != nil {
		t.Fatal(err)
	}
	p, _ := json.Marshal(map[string]string{"key": "link", "name": "copy", "input": filepath.Join(filepath.Dir(own), "escape")})
	if code, _ := e.call(bob, "bg.jobs.submit", string(p)); code != "bad_request" {
		t.Fatal("inbox symlink escaped", code)
	}
	old, _ := BackgroundInbox(e.dataDir, background.Submitter{User: "bob", UID: "1025"})
	current, _ := BackgroundInbox(e.dataDir, background.Submitter{User: "bob", UID: "2048"})
	if old == current {
		t.Fatal("recreated user retained its inbox")
	}
}
