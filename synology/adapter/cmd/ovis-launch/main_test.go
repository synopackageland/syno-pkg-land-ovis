package main

import (
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
