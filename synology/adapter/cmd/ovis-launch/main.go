// Command ovis-launch is the package daemon's env launcher for Ovis.
//
// It makes sure the per-install OVIS_TOKEN exists, adds it to the runtime
// environment and replaces itself with the packaged Bun running the upstream
// server entrypoint (server/index.ts, as `bun run server` does). That
// entrypoint has no self-updater: updates come only from Package Center.
// Every other setting is a fixed daemon.env value in dsm-package.yaml.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"ovis-dsm-adapter/internal/token"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ovis-launch: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return err
	}
	payload := filepath.Dir(self)
	app := filepath.Join(payload, "app")
	bun := filepath.Join(payload, "bun")
	t, err := token.Ensure(token.Path(os.Getenv))
	if err != nil {
		return fmt.Errorf("gateway token: %w", err)
	}
	if err := os.Chdir(app); err != nil {
		return err
	}
	env := withToken(os.Environ(), t)
	return syscall.Exec(bun, []string{bun, filepath.Join(app, "server", "index.ts")}, env)
}

// withToken returns env with OVIS_TOKEN (and its pre-rename alias) set to t
// and any inherited copy removed, so only the package's own token applies.
func withToken(env []string, t string) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if hasKey(kv, "OVIS_TOKEN") || hasKey(kv, "HERDR_WEB_TOKEN") {
			continue
		}
		out = append(out, kv)
	}
	return append(out, "OVIS_TOKEN="+t)
}

func hasKey(kv, key string) bool {
	return len(kv) > len(key) && kv[:len(key)] == key && kv[len(key)] == '='
}
