package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/synopackageland/syno-pkg-land-sdk/api"

	appapi "Ovis/internal/api"
)

type stubAuth struct {
	identity api.Identity
	err      error
}

func (s stubAuth) Authenticate(*http.Request) (api.Identity, error) { return s.identity, s.err }

func testConfig(t *testing.T) Config {
	return Config{PackageID: "HelloGo", Version: "0.1.0-0001", DataDir: t.TempDir()}
}

// rule is the access rule the server applies to r: the App policy for App
// methods, the SDK policy (reported by the route) for SDK methods.
func rule(t *testing.T, r api.Route) string {
	t.Helper()
	raw, err := appapi.Permissions()
	if err != nil {
		t.Fatal(err)
	}
	var policy struct{ Methods map[string]string }
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatal(err)
	}
	switch value, ok := policy.Methods[r.Method()]; {
	case ok:
		return value
	case r.Public():
		return "public"
	case r.AdminOnly():
		return api.AdministratorPermission
	}
	return "authenticated"
}

// call returns the envelope's error code ("" on success) and the envelope.
func call(t *testing.T, cfg Config, auth api.Authenticator, method, params string, permissionJSON ...[]byte) (string, map[string]any) {
	t.Helper()
	previous := lookupSubmitterUID
	lookupSubmitterUID = func(string) string { return "1024" }
	defer func() { lookupSubmitterUID = previous }()
	permissions, err := appapi.Permissions()
	if err != nil {
		t.Fatal(err)
	}
	if len(permissionJSON) != 0 {
		permissions = permissionJSON[0]
	}
	return serve(t, cfg, auth, apiRequest(method, params), permissions)
}

func apiRequest(method, params string) *http.Request {
	body := `{"method":"` + method + `","params":` + params + `}`
	req := httptest.NewRequest(http.MethodPost, "/api.cgi", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// A browser says which page sent a POST; the SDK refuses one that does not.
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	return req
}

// serve runs req through the App's server and returns as call does.
func serve(t *testing.T, cfg Config, auth api.Authenticator, req *http.Request, permissions []byte) (string, map[string]any) {
	t.Helper()
	srv, err := api.NewServerWithPermissions(auth, permissions, Routes(cfg)...)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not JSON: %s", rec.Body.String())
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP %d: DSM replaces non-200 CGI bodies", rec.Code)
	}
	code := ""
	if e, ok := out["error"].(map[string]any); ok {
		code, _ = e["code"].(string)
	}
	return code, out
}

var (
	alice  = stubAuth{identity: api.Identity{User: "alice", Admin: true}}
	bob    = stubAuth{identity: api.Identity{User: "bob"}}
	nobody = stubAuth{err: api.ErrNoSession}
)

// sampleParams holds valid params for every route, so the denial test below
// proves the refusal came from the policy and not from bad input.
// Expected domain errors for routes an overlay adds (fresh store, no DSM cookie).
var sampleOutcome = map[string]string{}

var sampleParams = map[string]string{
	"app.status":          `{}`,
	"sharedfolder.status": `{}`,
}

func TestEveryRouteHasSampleParamsAndNoneIsPublic(t *testing.T) {
	routes := Routes(testConfig(t))
	if len(routes) != len(sampleParams) {
		t.Fatalf("%d routes but %d sample params: add the new route to sampleParams", len(routes), len(sampleParams))
	}
	for _, r := range routes {
		if _, ok := sampleParams[r.Method()]; !ok {
			t.Errorf("route %s has no sample params", r.Method())
		}
		if rule(t, r) == "public" {
			t.Errorf("route %s is public; this template ships none", r.Method())
		}
	}
}

func TestEveryRouteRefusesCallersWithoutASessionAndChangesNothing(t *testing.T) {
	for _, r := range Routes(testConfig(t)) {
		cfg := testConfig(t)
		code, _ := call(t, cfg, nobody, r.Method(), sampleParams[r.Method()])
		if code != "session_required" {
			t.Errorf("%s: %q, want session_required", r.Method(), code)
		}
		assertDataDirEmpty(t, cfg)
	}
}

func TestAdministratorRoutesRefuseOrdinaryUsersAndChangeNothing(t *testing.T) {
	for _, r := range Routes(testConfig(t)) {
		if rule(t, r) != api.AdministratorPermission {
			continue
		}
		cfg := testConfig(t)
		code, _ := call(t, cfg, bob, r.Method(), sampleParams[r.Method()])
		if code != "admin_required" {
			t.Errorf("%s: %q, want admin_required", r.Method(), code)
		}
		assertDataDirEmpty(t, cfg)
	}
}

func TestEveryRouteSucceedsForAnAdmin(t *testing.T) {
	for _, r := range Routes(testConfig(t)) {
		code, out := call(t, testConfig(t), alice, r.Method(), sampleParams[r.Method()])
		want := sampleOutcome[r.Method()]
		if strings.HasPrefix(r.Method(), "jobs.") {
			want = "unavailable"
		}
		if code != want {
			t.Errorf("%s: %q %v", r.Method(), code, out)
		}
	}
}

func TestStatusReportsPackageAndCaller(t *testing.T) {
	code, out := call(t, testConfig(t), alice, "app.status", `{}`)
	data := out["data"].(map[string]any)
	if code != "" || data["package"] != "HelloGo" || data["version"] != "0.1.0-0001" || data["user"] != "alice" || data["admin"] != true {
		t.Fatalf("got %q %v", code, out)
	}
}

func TestSampleOperationsAreAbsent(t *testing.T) {
	for _, method := range []string{"greeting.get", "greeting.set"} {
		if code, _ := call(t, testConfig(t), alice, method, `{}`); code != "unknown_method" {
			t.Fatalf("sample route %s remains: %s", method, code)
		}
	}
}

func assertDataDirEmpty(t *testing.T, cfg Config) {
	t.Helper()
	entries, err := os.ReadDir(cfg.DataDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("data dir changed: %v %v", entries, err)
	}
}
