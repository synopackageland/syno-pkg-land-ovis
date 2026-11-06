package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/synopackageland/syno-pkg-land-sdk/api"
	"github.com/synopackageland/syno-pkg-land-sdk/portal"

	appapi "Ovis/internal/api"
)

func portalConfig(t *testing.T, saved string) Config {
	cfg := testConfig(t)
	cfg.Portal = true
	cfg.PortalConfig = filepath.Join(t.TempDir(), "AppPortal.json")
	if saved != "" {
		if err := os.WriteFile(cfg.PortalConfig, []byte(saved), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return cfg
}

func TestPortalStatusTellsEverySignedInUserTheAlias(t *testing.T) {
	cfg := portalConfig(t, `{"alias":"hello-go","id":"SYNO.SDS.HelloGo.Instance","version":3}`)
	code, out := call(t, cfg, bob, "portal.status", `{}`)
	if code != "" {
		t.Fatalf("%q %v", code, out)
	}
	data := out["data"].(map[string]any)
	if data["state"] != "active" || data["alias"] != "hello-go" {
		t.Fatalf("%v", data)
	}
	if code, _ := call(t, cfg, nobody, "portal.status", `{}`); code != "session_required" {
		t.Fatalf("anonymous: %q", code)
	}
	if code, _ := call(t, cfg, bob, "portal.status", `{"alias":"x"}`); code != "bad_request" {
		t.Fatalf("params: %q", code)
	}
}

func TestPortalStatusFailsClosedOnAnotherAppsSetting(t *testing.T) {
	cfg := portalConfig(t, `{"alias":"hello-go","id":"SYNO.SDS.Other.Instance"}`)
	_, out := call(t, cfg, bob, "portal.status", `{}`)
	if data := out["data"].(map[string]any); data["state"] != "unavailable" || data["alias"] != nil {
		t.Fatalf("%v", data)
	}
	_, out = call(t, portalConfig(t, ""), bob, "portal.status", `{}`)
	if data := out["data"].(map[string]any); data["state"] != "inactive" {
		t.Fatalf("%v", data)
	}
}

func TestPortalStatusExistsOnlyForAPortalBuild(t *testing.T) {
	if code, _ := call(t, testConfig(t), alice, "portal.status", `{}`); code != "unknown_method" {
		t.Fatalf("%q", code)
	}
}

// fakeEntry is the executor as the portal routes see it.
type fakeEntry struct {
	entry string
	err   error
	calls []string
	auth  map[string]string
}

func (f *fakeEntry) Call(_ context.Context, op string, auth map[string]string, _, result any) error {
	f.calls = append(f.calls, op)
	if f.err != nil {
		return f.err
	}
	switch op {
	case "portal.register":
		f.entry, f.auth = portal.EntryRegistered, auth
	case "portal.unregister":
		f.entry, f.auth = portal.EntryUnregistered, auth
	}
	key := "entry"
	if op == "status" {
		key = "portal"
	}
	b, _ := json.Marshal(map[string]string{key: f.entry})
	return json.Unmarshal(b, result)
}

func executorPortalConfig(t *testing.T, saved string, executor *fakeEntry) Config {
	cfg := portalConfig(t, saved)
	cfg.PortalExecutor = true
	previous := portalExecutor
	portalExecutor = func(Config) portal.Executor { return executor }
	t.Cleanup(func() { portalExecutor = previous })
	return cfg
}

func sessionCall(t *testing.T, cfg Config, method string) (string, map[string]any) {
	t.Helper()
	permissions, err := appapi.Permissions()
	if err != nil {
		t.Fatal(err)
	}
	req := apiRequest(method, `{}`)
	req.Header.Set("Cookie", "id=session")
	req.Header.Set("X-SYNO-TOKEN", "token")
	return serve(t, cfg, alice, req, permissions)
}

func TestExecutorPortalStatusReportsTheEntry(t *testing.T) {
	saved := `{"alias":"hello-go","id":"SYNO.SDS.HelloGo.Instance"}`
	for _, c := range []struct {
		entry string
		err   error
		state any
		alias any
	}{
		{portal.EntryRegistered, nil, "active", "hello-go"},
		{portal.EntryCleanupRequired, nil, "active", "hello-go"},
		{portal.EntryUnregistered, nil, "inactive", nil},
		{"", errors.New("executor down"), "unavailable", nil},
	} {
		cfg := executorPortalConfig(t, saved, &fakeEntry{entry: c.entry, err: c.err})
		code, out := call(t, cfg, bob, "portal.status", `{}`)
		data := out["data"].(map[string]any)
		if code != "" || data["state"] != c.state || data["alias"] != c.alias {
			t.Fatalf("%s: %q %v", c.entry, code, data)
		}
		if c.err == nil && data["entry"] != c.entry {
			t.Fatalf("%s: entry %v", c.entry, data)
		}
	}
}

func TestExecutorPortalRegisterAndUnregisterReturnTheFreshStatus(t *testing.T) {
	executor := &fakeEntry{entry: portal.EntryUnregistered}
	cfg := executorPortalConfig(t, `{"alias":"hello-go","id":"SYNO.SDS.HelloGo.Instance"}`, executor)
	code, out := sessionCall(t, cfg, "portal.register")
	if data, _ := out["data"].(map[string]any); code != "" || data["entry"] != portal.EntryRegistered || data["state"] != "active" {
		t.Fatalf("register: %q %v", code, out)
	}
	if executor.auth["dsm_cookie"] != "session" {
		t.Fatalf("register did not forward the DSM session: %v", executor.auth)
	}
	code, out = sessionCall(t, cfg, "portal.unregister")
	if data, _ := out["data"].(map[string]any); code != "" || data["entry"] != portal.EntryUnregistered || data["state"] != "inactive" {
		t.Fatalf("unregister: %q %v", code, out)
	}
}

func TestExecutorPortalChangesNeedAnAdministratorsDSMSession(t *testing.T) {
	executor := &fakeEntry{entry: portal.EntryUnregistered}
	cfg := executorPortalConfig(t, "", executor)
	for _, method := range []string{"portal.register", "portal.unregister"} {
		if code, _ := call(t, cfg, bob, method, `{}`); code == "" {
			t.Fatalf("%s: an ordinary user changed the entry", method)
		}
		if code, _ := call(t, cfg, alice, method, `{}`); code != "session_required" {
			t.Fatalf("%s without a DSM session: %q", method, code)
		}
	}
	if len(executor.calls) != 0 {
		t.Fatalf("the executor was asked: %v", executor.calls)
	}
}

func TestExecutorPortalRefusalKeepsTheExecutorsCode(t *testing.T) {
	cfg := executorPortalConfig(t, "", &fakeEntry{err: &api.Error{Code: "PORTAL_FAILED", Message: "x"}})
	if code, _ := sessionCall(t, cfg, "portal.register"); code != "PORTAL_FAILED" {
		t.Fatalf("%q", code)
	}
}

func TestPortalChangesExistOnlyInAnExecutorPortalBuild(t *testing.T) {
	for _, method := range []string{"portal.register", "portal.unregister"} {
		if code, _ := call(t, portalConfig(t, ""), alice, method, `{}`); code != "unknown_method" {
			t.Fatalf("%s: %q", method, code)
		}
	}
}
