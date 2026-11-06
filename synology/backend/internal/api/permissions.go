// Package api holds this App's method policy. The SDK's own capability
// methods keep the SDK policy; this file only configures App methods.
package api

import (
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
)

//go:embed permissions.json
var permissionsJSON []byte

// Set by `synopkgland build` through -ldflags -X to bind the policy it
// validated to the compiled CGI. Empty only in direct Go development builds.
var expectedPermissionsDigest string

// Permissions returns permissions.json, refusing a file that differs from the
// one the package build validated.
func Permissions() ([]byte, error) {
	if expectedPermissionsDigest != "" && fmt.Sprintf("%x", sha256.Sum256(permissionsJSON)) != expectedPermissionsDigest {
		return nil, errors.New("API permissions do not match package build policy")
	}
	return permissionsJSON, nil
}

// operationsJSON is generated from the App registry and the SDK capability catalog.
//
//go:embed operations.json
var operationsJSON []byte

func Operations() []byte { return operationsJSON }
