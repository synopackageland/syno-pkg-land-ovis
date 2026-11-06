package api

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestPermissionsWithoutABuildDigestAreTheEmbeddedFile(t *testing.T) {
	got, err := Permissions()
	if err != nil || string(got) != string(permissionsJSON) {
		t.Fatalf("Permissions() = %q, %v", got, err)
	}
}

func TestPermissionsMatchingTheBuildDigestAreAccepted(t *testing.T) {
	t.Cleanup(func() { expectedPermissionsDigest = "" })
	expectedPermissionsDigest = fmt.Sprintf("%x", sha256.Sum256(permissionsJSON))
	if _, err := Permissions(); err != nil {
		t.Fatal(err)
	}
}

func TestPermissionsDifferingFromTheBuildDigestAreRefused(t *testing.T) {
	t.Cleanup(func() { expectedPermissionsDigest = "" })
	expectedPermissionsDigest = fmt.Sprintf("%x", sha256.Sum256([]byte("other")))
	if got, err := Permissions(); err == nil || got != nil {
		t.Fatalf("Permissions() = %q, %v; want refusal", got, err)
	}
}
