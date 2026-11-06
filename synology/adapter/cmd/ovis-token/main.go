// Command ovis-token is the DSM identity gateway's token adapter for Ovis.
//
// The gateway runs it as the package account for each request a DSM
// administrator was admitted for, and sets the returned header upstream in
// place of the client's own credentials. It prints Ovis's proxy credential,
// {"headers":{"Authorization":["Bearer <OVIS_TOKEN>"]}}, and mints nothing:
// ovis-launch creates the token before Ovis starts.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"ovis-dsm-adapter/internal/token"
)

func main() {
	out, err := reply(secretPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "ovis-token: %v\n", err)
		os.Exit(1)
	}
	os.Stdout.Write(out)
}

func reply(path string) ([]byte, error) {
	t, err := token.Read(path)
	if err != nil {
		return nil, err
	}
	out, err := json.Marshal(map[string]map[string][]string{
		"headers": {"Authorization": {"Bearer " + t}},
	})
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// The SDK materializes the same private runtime seed that it injects into Ovis.
func secretPath() string {
	root := os.Getenv("SYNOPKG_PKGDEST")
	if root == "" {
		root = "/var/packages/Ovis/target"
	}
	return filepath.Join(root, "compose", "secrets", "runtime-ovis.secret")
}
