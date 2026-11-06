package app

import (
	"context"
	"github.com/synopackageland/syno-pkg-land-sdk/api"
	"github.com/synopackageland/syno-pkg-land-sdk/background"
	"strings"
	"testing"
)

func ordinaryEnv(t *testing.T) *bgEnv {
	e := newBgEnv(t)
	lookupSubmitterUID = func(name string) string {
		switch name {
		case "alice":
			return "1024"
		case "bob":
			return "1025"
		case "carol":
			return "1026"
		}
		return ""
	}
	return e
}

var carol = stubAuth{identity: api.Identity{User: "carol"}}

func ownerSubmit(t *testing.T, e *bgEnv, a api.Authenticator, key string) string {
	t.Helper()
	code, out := e.call(a, "bg.jobs.submit", e.submitParams(key, a.(stubAuth).identity.User))
	if code != "" {
		t.Fatalf("submit %s: %s %v", key, code, out)
	}
	return data(out)["job"].(map[string]any)["id"].(string)
}
func TestBackgroundOwnersAreIsolatedIncludingKeysAndCounts(t *testing.T) {
	e := ordinaryEnv(t)
	a := ownerSubmit(t, e, alice, "same")
	b := ownerSubmit(t, e, bob, "same")
	c := ownerSubmit(t, e, carol, "same")
	if a == b || b == c {
		t.Fatal("idempotency key leaked across owners")
	}
	for _, auth := range []api.Authenticator{bob, carol} {
		code, out := e.call(auth, "bg.jobs.list", `{}`)
		if code != "" || len(data(out)["jobs"].([]any)) != 1 {
			t.Fatalf("list: %s %v", code, out)
		}
		_, out = e.call(auth, "bg.status", `{}`)
		if data(out)["queued"] != float64(1) || data(out)["admin"] != false {
			t.Fatalf("status leaks: %v", out)
		}
	}
	for _, method := range []string{"bg.jobs.get", "bg.jobs.cancel", "bg.jobs.pause", "bg.jobs.resume", "bg.jobs.result"} {
		for _, id := range []string{a, c, strings.Repeat("0", 32)} {
			if code, out := e.call(bob, method, `{"id":"`+id+`"}`); code != "not_found" {
				t.Fatalf("%s %s: %s %v", method, id, code, out)
			}
		}
	}
	_, out := e.call(alice, "bg.jobs.list", `{}`)
	if len(data(out)["jobs"].([]any)) != 3 {
		t.Fatal("admin must see all")
	}
	if code, _ := e.call(alice, "bg.jobs.cancel", `{"id":"`+b+`"}`); code != "" {
		t.Fatal(code)
	}
}
func TestBackgroundOwnerResultAndUIDChangeFailClosed(t *testing.T) {
	e := ordinaryEnv(t)
	id := ownerSubmit(t, e, bob, "one")
	ctx := context.Background()
	_, _ = e.store.Claim(ctx)
	if err := e.store.Finish(ctx, id, background.Outcome{State: background.StateSucceeded, Result: &background.Result{SHA256: "digest", Bytes: 5}}); err != nil {
		t.Fatal(err)
	}
	if code, _ := e.call(bob, "bg.jobs.result", `{"id":"`+id+`"}`); code != "" {
		t.Fatal(code)
	}
	if code, _ := e.call(carol, "bg.jobs.result", `{"id":"`+id+`"}`); code != "not_found" {
		t.Fatal(code)
	}
	lookupSubmitterUID = func(string) string { return "2000" }
	_, out := e.call(bob, "bg.jobs.list", `{}`)
	if len(data(out)["jobs"].([]any)) != 0 {
		t.Fatal("same name with new UID saw old jobs")
	}
	if code, _ := e.call(bob, "bg.jobs.get", `{"id":"`+id+`"}`); code != "not_found" {
		t.Fatal(code)
	}
	lookupSubmitterUID = func(string) string { return "" }
	if code, _ := e.call(bob, "bg.jobs.list", `{}`); code != "identity_unavailable" {
		t.Fatal(code)
	}
}
func TestBackgroundPersonalQuotaAndAdminPurge(t *testing.T) {
	e := ordinaryEnv(t)
	for _, method := range []string{"bg.jobs.purge"} {
		if code, _ := e.call(bob, method, `{}`); code != "admin_required" {
			t.Fatalf("%s: %s", method, code)
		}
	}
	ownerSubmit(t, e, bob, "one")
	ownerSubmit(t, e, bob, "two")
	if code, _ := e.call(bob, "bg.jobs.submit", e.submitParams("three", "bob")); code != "owner-queue-full" {
		t.Fatal(code)
	}
	ownerSubmit(t, e, carol, "one")

}
func TestBackgroundPurgeRequiresAdminAndExactOwner(t *testing.T) {
	e := ordinaryEnv(t)
	id := ownerSubmit(t, e, bob, "one")
	ownerSubmit(t, e, carol, "one")
	code, out := e.call(alice, "bg.jobs.purge", `{"owner":{"user":"bob","uid":"1025"}}`)
	if code != "" || data(out)["removed"] != float64(1) {
		t.Fatalf("purge: %s %v", code, out)
	}
	if code, _ := e.call(alice, "bg.jobs.get", `{"id":"`+id+`"}`); code != "not_found" {
		t.Fatal(code)
	}
	_, out = e.call(alice, "bg.jobs.list", `{}`)
	if len(data(out)["jobs"].([]any)) != 1 {
		t.Fatal("purge touched another owner")
	}
}

func TestFreshOrdinaryUserJobsNeedNoAdminSetup(t *testing.T) {
	e := newBgEnv(t)
	lookupSubmitterUID = func(name string) string {
		if name == "bob" {
			return "1025"
		}
		return "1026"
	}
	code, out := e.call(bob, "bg.status", `{}`)
	if code != "" {
		t.Fatalf("fresh status: %s %v", code, out)
	}
	for _, key := range []string{"enabled", "allowOrdinaryUsers"} {
		if _, exists := data(out)[key]; exists {
			t.Fatalf("obsolete status switch %s", key)
		}
	}
	id := ownerSubmit(t, e, bob, "fresh")
	for _, method := range []string{"bg.jobs.get", "bg.jobs.cancel", "bg.jobs.pause", "bg.jobs.resume"} {
		if code, _ := e.call(carol, method, `{"id":"`+id+`"}`); code != "not_found" {
			t.Fatalf("other owner %s: %s", method, code)
		}
	}
	if code, _ := e.call(bob, "bg.jobs.get", `{"id":"`+id+`"}`); code != "" {
		t.Fatal(code)
	}
	if code, _ := e.call(bob, "bg.jobs.cancel", `{"id":"`+id+`"}`); code != "" {
		t.Fatal(code)
	}
	for _, method := range []string{"bg.mode.set", "bg.users.set"} {
		if code, _ := e.call(alice, method, `{}`); code != "unknown_method" {
			t.Fatalf("obsolete route %s: %s", method, code)
		}
	}
}
