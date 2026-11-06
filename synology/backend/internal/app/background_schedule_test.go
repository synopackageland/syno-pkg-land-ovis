package app

import (
	"context"
	"fmt"
	"github.com/synopackageland/syno-pkg-land-sdk/background"
	"testing"
)

func TestBackgroundScheduleRoutesConsentOwnerAndAdmin(t *testing.T) {
	e := ordinaryEnv(t)
	id := ownerSubmit(t, e, bob, "copy")
	ctx := context.Background()
	_, _ = e.store.Claim(ctx)
	_ = e.store.Finish(ctx, id, background.Outcome{State: background.StateSucceeded, Result: &background.Result{Bytes: 5}})
	params := fmt.Sprintf(`{"key":"s","jobId":%q,"intervalSeconds":3600,"consent":true}`, id)
	if code, _ := e.call(carol, "bg.schedules.create", params); code != "not_found" {
		t.Fatal(code)
	}
	for _, p := range []string{fmt.Sprintf(`{"key":"s","jobId":%q,"intervalSeconds":3600}`, id), fmt.Sprintf(`{"key":"s","jobId":%q,"intervalSeconds":3600,"consent":true,"owner":"carol"}`, id)} {
		if code, _ := e.call(bob, "bg.schedules.create", p); code != "bad_request" {
			t.Fatal(code)
		}
	}
	code, out := e.call(bob, "bg.schedules.create", params)
	if code != "" {
		t.Fatalf("create %s %v", code, out)
	}
	sid := data(out)["schedule"].(map[string]any)["id"].(string)
	_, out = e.call(carol, "bg.schedules.list", `{}`)
	if len(data(out)["schedules"].([]any)) != 0 {
		t.Fatal("list leak")
	}
	_, out = e.call(alice, "bg.schedules.list", `{}`)
	if len(data(out)["schedules"].([]any)) != 1 {
		t.Fatal("admin list")
	}
	for _, m := range []string{"bg.schedules.disable", "bg.schedules.delete"} {
		if code, _ := e.call(carol, m, fmt.Sprintf(`{"id":%q}`, sid)); code != "not_found" {
			t.Fatal(code)
		}
	}
	if code, _ := e.call(bob, "bg.schedules.enable", fmt.Sprintf(`{"id":%q,"consent":true}`, sid)); code != "admin_required" {
		t.Fatal(code)
	}
	if code, _ := e.call(alice, "bg.schedules.disable", fmt.Sprintf(`{"id":%q}`, sid)); code != "" {
		t.Fatal(code)
	}
	if code, _ := e.call(alice, "bg.schedules.enable", fmt.Sprintf(`{"id":%q}`, sid)); code != "bad_request" {
		t.Fatal(code)
	}
	if code, _ := e.call(alice, "bg.schedules.enable", fmt.Sprintf(`{"id":%q,"consent":true}`, sid)); code != "" {
		t.Fatal(code)
	}
	if code, _ := e.call(bob, "bg.schedules.delete", fmt.Sprintf(`{"id":%q}`, sid)); code != "" {
		t.Fatal(code)
	}
}
