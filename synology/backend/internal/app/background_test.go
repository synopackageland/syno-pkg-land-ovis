package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/synopackageland/syno-pkg-land-sdk/api"
	"github.com/synopackageland/syno-pkg-land-sdk/background"
)

type bgEnv struct {
	t       *testing.T
	store   *background.Store
	dataDir string
	srv     map[string]*api.Server
}

func newBgEnv(t *testing.T) *bgEnv {
	lookupSubmitterUID = func(string) string { return "1024" }
	t.Cleanup(func() { lookupSubmitterUID = osLookupUID })
	dir := t.TempDir()
	store := background.NewStore(filepath.Join(dir, "background"))
	e := &bgEnv{t: t, store: store, dataDir: dir, srv: map[string]*api.Server{}}
	e.inputPath("alice")
	return e
}

func (e *bgEnv) call(auth api.Authenticator, method, params string) (string, map[string]any) {
	return e.callContext(context.Background(), auth, method, params)
}

func (e *bgEnv) callContext(ctx context.Context, auth api.Authenticator, method, params string) (string, map[string]any) {
	e.t.Helper()
	// SDK policy provides owner routes immediately.
	routes := newBackgroundRoutes(e.store, e.dataDir)
	srv, err := api.NewServerWithPermissions(auth, []byte(`{"schema":1,"default":"administrators","methods":{}}`), routes...)
	if err != nil {
		e.t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api.cgi", strings.NewReader(`{"method":"`+method+`","params":`+params+`}`))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		e.t.Fatalf("not JSON: %s", rec.Body.String())
	}
	code := ""
	if er, ok := out["error"].(map[string]any); ok {
		code, _ = er["code"].(string)
	}
	return code, out
}

func data(out map[string]any) map[string]any { return out["data"].(map[string]any) }

var bgSample = map[string]string{
	"bg.schedules.list":    `{}`,
	"bg.schedules.create":  `{"key":"schedule","jobId":"missing","intervalSeconds":3600,"consent":true}`,
	"bg.schedules.disable": `{"id":"missing"}`,
	"bg.schedules.delete":  `{"id":"missing"}`,
	"bg.schedules.enable":  `{"id":"missing","consent":true}`,
	"bg.status":            `{}`,
	"bg.jobs.purge":        `{"owner":{"user":"bob","uid":"1025"}}`,
	"bg.jobs.submit":       `{"key":"k1","name":"a.txt","input":"/missing"}`,
	"bg.jobs.list":         `{}`,
	"bg.jobs.get":          `{"id":"` + strings.Repeat("a", 32) + `"}`,
	"bg.jobs.cancel":       `{"id":"` + strings.Repeat("a", 32) + `"}`,
	"bg.jobs.pause":        `{"id":"` + strings.Repeat("a", 32) + `"}`,
	"bg.jobs.resume":       `{"id":"` + strings.Repeat("a", 32) + `"}`,
	"bg.jobs.result":       `{"id":"` + strings.Repeat("a", 32) + `"}`,
}

func TestBackgroundSDKPoliciesAuthenticateEveryRoute(t *testing.T) {
	e := newBgEnv(t)
	routes := newBackgroundRoutes(e.store, e.dataDir)
	if len(routes) != len(bgSample) {
		t.Fatalf("%d routes, %d samples", len(routes), len(bgSample))
	}
	for _, r := range routes {
		if rule(t, r) == "public" {
			t.Errorf("%s must not be public", r.Method())
		}
		params, ok := bgSample[r.Method()]
		if !ok {
			t.Fatalf("no sample for %s", r.Method())
		}
		code, _ := e.call(bob, r.Method(), params)
		if r.Method() == "bg.jobs.purge" || r.Method() == "bg.schedules.enable" {
			if code != "admin_required" {
				t.Errorf("%s as ordinary user: %q", r.Method(), code)
			}
		} else if code == "admin_required" || code == "unsupported" || code == "disabled" {
			t.Errorf("%s unexpectedly blocked: %q", r.Method(), code)
		}
		if code, _ := e.call(nobody, r.Method(), params); code != "session_required" {
			t.Errorf("%s without a session: %q", r.Method(), code)
		}
	}
}

func TestBackgroundStatusReportsLimitsAndWorker(t *testing.T) {
	e := newBgEnv(t)
	code, out := e.call(alice, "bg.status", `{}`)
	d := data(out)
	if code != "" || d["algorithm"] != "sha256-v1" || d["worker"] != nil {
		t.Fatalf("%s %v", code, out)
	}
	limits := d["limits"].(map[string]any)
	for name, want := range map[string]float64{
		"maxInputBytes":     background.MaxInputBytes,
		"maxQueued":         background.MaxQueued,
		"maxSchedules":      background.MaxSchedules,
		"maxTotalBytes":     background.MaxTotalBytes,
		"maxOwnerPending":   background.MaxOwnerPending,
		"maxOwnerBytes":     background.MaxOwnerBytes,
		"jobTimeoutSeconds": background.JobTimeout.Seconds(),
	} {
		if got := limits[name]; got != want {
			t.Errorf("limits[%q] = %v, want %v", name, got, want)
		}
	}
	info := background.WorkerInfo{PID: os.Getpid(), UID: os.Getuid(), Username: "pkg"}
	if err := e.store.Heartbeat(context.Background(), info); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.store.ClearHeartbeat(info) })
	_, out = e.call(alice, "bg.status", `{}`)
	w := data(out)["worker"].(map[string]any)
	if w["alive"] != true || w["uid"].(float64) != float64(info.UID) || w["pid"].(float64) != float64(info.PID) {
		t.Fatalf("worker: %v", w)
	}
}

func (e *bgEnv) inputPath(user string) string {
	inbox, err := BackgroundInbox(e.dataDir, background.Submitter{User: user, UID: lookupSubmitterUID(user)})
	if err != nil {
		e.t.Fatal(err)
	}
	if err = os.MkdirAll(inbox, 0700); err != nil {
		e.t.Fatal(err)
	}
	input := filepath.Join(inbox, "input.txt")
	if _, err = os.Stat(input); os.IsNotExist(err) {
		if err = os.WriteFile(input, []byte("hello"), 0600); err != nil {
			e.t.Fatal(err)
		}
	}
	return input
}
func (e *bgEnv) submitParams(key string, users ...string) string {
	user := "alice"
	if len(users) > 0 {
		user = users[0]
	}
	p, err := json.Marshal(map[string]string{"key": key, "name": "a.txt", "input": e.inputPath(user)})
	if err != nil {
		e.t.Fatal(err)
	}
	return string(p)
}

func TestBackgroundSubmitImportsAppInputAndIsIdempotent(t *testing.T) {
	e := newBgEnv(t)
	params := e.submitParams("k1")
	code, out := e.call(alice, "bg.jobs.submit", params)
	if code != "" {
		t.Fatalf("%s %v", code, out)
	}
	job := data(out)["job"].(map[string]any)
	if job["state"] != "queued" || job["name"] != "a.txt" || data(out)["duplicate"] != false {
		t.Fatal(out)
	}
	submitter := job["submitter"].(map[string]any)
	if submitter["user"] != "alice" || submitter["uid"] != "1024" {
		t.Fatal(submitter)
	}
	_, again := e.call(alice, "bg.jobs.submit", params)
	if data(again)["duplicate"] != true || data(again)["job"].(map[string]any)["id"] != job["id"] {
		t.Fatal(again)
	}
	if err := os.WriteFile(e.inputPath("alice"), []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	if code, _ := e.call(alice, "bg.jobs.submit", params); code != "conflict" {
		t.Fatalf("different bytes: %q", code)
	}
}

func TestBackgroundSubmitRejectsExternalAndOversizedInput(t *testing.T) {
	e := newBgEnv(t)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(e.dataDir, "escape")); err != nil {
		t.Fatal(err)
	}
	if code, _ := e.call(alice, "bg.jobs.submit", e.submitParams("first")); code != "" {
		t.Fatal(code)
	}
	spooled, err := filepath.Glob(filepath.Join(e.dataDir, "background", "*"))
	if err != nil || len(spooled) == 0 {
		t.Fatalf("no spool files: %v", err)
	}
	if err := os.WriteFile(filepath.Join(e.dataDir, "background", "copy"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	inputs := []string{outside, "input.txt", filepath.Join(e.dataDir, "escape"), e.dataDir, filepath.Join(e.dataDir, "background", "copy")}
	for _, input := range append(inputs, spooled...) {
		p, _ := json.Marshal(map[string]string{"key": "bad", "name": "a.txt", "input": input})
		if code, _ := e.call(alice, "bg.jobs.submit", string(p)); code != "bad_request" {
			t.Fatalf("%s: %s", input, code)
		}
	}
	if err := os.WriteFile(e.inputPath("alice"), make([]byte, background.MaxInputBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	if code, _ := e.call(alice, "bg.jobs.submit", e.submitParams("big")); code != "limit-exceeded" {
		t.Fatal(code)
	}
	jobs, err := e.store.List(context.Background())
	if err != nil || len(jobs) != 1 {
		t.Fatalf("invalid input queued: %v %v", jobs, err)
	}
}

func TestBackgroundSubmitValidatesParams(t *testing.T) {
	e := newBgEnv(t)
	for _, p := range []string{
		`{}`, `{"key":"k","name":"a","input":"/a","extra":1}`, e.submitParams("bad key"),
		`{"key":"k","name":"a","input":""}`,
	} {
		if code, _ := e.call(alice, "bg.jobs.submit", p); code != "bad_request" {
			t.Errorf("%s: %q", p, code)
		}
	}
}

func TestBackgroundQueueFullAndQuotaErrorsHaveStableCodes(t *testing.T) {
	e := newBgEnv(t)
	for i := 0; i < background.MaxQueued; i++ {
		lookupSubmitterUID = func(string) string { return fmt.Sprint(1024 + i/2) }
		key := "k" + string(rune('a'+i))
		if code, _ := e.call(alice, "bg.jobs.submit", e.submitParams(key)); code != "" {
			t.Fatalf("%d: %q", i, code)
		}
	}
	lookupSubmitterUID = func(string) string { return "9999" }
	if code, _ := e.call(alice, "bg.jobs.submit", e.submitParams("more")); code != "queue-full" {
		t.Fatalf("%q", code)
	}
}

func TestBackgroundListGetCancelAndResult(t *testing.T) {
	e := newBgEnv(t)
	_, out := e.call(alice, "bg.jobs.submit", e.submitParams("k1"))
	id := data(out)["job"].(map[string]any)["id"].(string)
	idp := `{"id":"` + id + `"}`

	if _, out = e.call(alice, "bg.jobs.list", `{}`); len(data(out)["jobs"].([]any)) != 1 {
		t.Fatalf("%v", out)
	}
	if code, out := e.call(alice, "bg.jobs.get", idp); code != "" || data(out)["job"].(map[string]any)["id"] != id {
		t.Fatalf("%s %v", code, out)
	}
	if code, _ := e.call(alice, "bg.jobs.result", idp); code != "not_ready" {
		t.Fatalf("result of a queued job: %q", code)
	}
	ctx := context.Background()
	if _, err := e.store.Claim(ctx); err != nil {
		t.Fatal(err)
	}
	sum := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if err := e.store.Finish(ctx, id, background.Outcome{State: background.StateSucceeded, Result: &background.Result{SHA256: sum, Bytes: 5}}); err != nil {
		t.Fatal(err)
	}
	code, out := e.call(alice, "bg.jobs.result", idp)
	res := data(out)["result"].(map[string]any)
	if code != "" || res["sha256"] != sum || res["bytes"].(float64) != 5 || res["worker"] != "sha256-v1" || res["jobId"] != id {
		t.Fatalf("%s %v", code, out)
	}
	if code, _ := e.call(alice, "bg.jobs.cancel", idp); code != "conflict" {
		t.Fatalf("cancel after completion: %q", code)
	}
	for _, m := range []string{"bg.jobs.get", "bg.jobs.cancel", "bg.jobs.result"} {
		if code, _ := e.call(alice, m, `{"id":"`+strings.Repeat("0", 32)+`"}`); code != "not_found" {
			t.Fatalf("%s unknown id: %q", m, code)
		}
	}
}

func TestBackgroundCancelQueuedJob(t *testing.T) {
	e := newBgEnv(t)
	_, out := e.call(alice, "bg.jobs.submit", e.submitParams("k1"))
	id := data(out)["job"].(map[string]any)["id"].(string)
	code, out := e.call(alice, "bg.jobs.cancel", `{"id":"`+id+`"}`)
	if code != "" || data(out)["job"].(map[string]any)["state"] != "canceled" {
		t.Fatalf("%s %v", code, out)
	}
}

func TestBackgroundPauseAndResumeQueuedJob(t *testing.T) {
	e := newBgEnv(t)
	_, out := e.call(alice, "bg.jobs.submit", e.submitParams("k1"))
	idp := `{"id":"` + data(out)["job"].(map[string]any)["id"].(string) + `"}`
	for method, state := range map[string]string{"bg.jobs.pause": "paused", "bg.jobs.resume": "queued"} {
		if code, out := e.call(alice, method, idp); code != "" || data(out)["job"].(map[string]any)["state"] != state {
			t.Fatalf("%s: %s %v", method, code, out)
		}
	}
	if code, _ := e.call(alice, "bg.jobs.cancel", idp); code != "" {
		t.Fatal(code)
	}
	for _, m := range []string{"bg.jobs.pause", "bg.jobs.resume"} {
		if code, _ := e.call(alice, m, idp); code != "conflict" {
			t.Fatalf("%s after cancel: %q", m, code)
		}
		if code, _ := e.call(alice, m, `{"id":"`+strings.Repeat("0", 32)+`"}`); code != "not_found" {
			t.Fatalf("%s unknown id: %q", m, code)
		}
	}
}

func TestBackgroundWorkerAliveExpires(t *testing.T) {
	if workerAlive(background.WorkerInfo{At: time.Now().Add(-time.Minute)}, time.Now()) || !workerAlive(background.WorkerInfo{At: time.Now()}, time.Now()) {
		t.Fatal("a stale heartbeat is not alive")
	}
}

func TestBackgroundListWithoutJobsIsAnEmptyArray(t *testing.T) {
	e := newBgEnv(t)
	_, out := e.call(alice, "bg.jobs.list", `{}`)
	jobs, ok := data(out)["jobs"].([]any)
	if !ok || len(jobs) != 0 {
		t.Fatalf("jobs must be [] not null: %v", out)
	}
}
