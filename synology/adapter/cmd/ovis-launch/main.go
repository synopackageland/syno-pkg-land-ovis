// Command ovis-launch is the package daemon's launcher and supervisor for Ovis.
//
// It makes sure the per-install OVIS_TOKEN exists, turns anonymous usage counts
// off by default on a first start (see telemetryDefaultOff), then runs two
// children as the package account:
//
//   - the bundled herdr (official release binary, `herdr server`), headless,
//     with its config folder chosen so that it listens on HERDR_SOCKET, the
//     socket Ovis reads. The NAS is then a working herdr host of its own.
//   - the packaged Bun on upstream's server entrypoint (server/index.ts, as
//     `bun run server` does), with OVIS_TOKEN and OVIS_HERDR_BIN added.
//
// herdr starts first and Bun after it. A herdr that exits while Ovis runs is
// started again with backoff; when Ovis exits, or the package stops (SIGTERM),
// Bun is stopped first and herdr after it, and the launcher exits with Bun's
// status, so the SDK's own restart policy still applies to Ovis.
// Every other setting is a fixed daemon.env value in dsm-package.yaml.
package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"ovis-dsm-adapter/internal/token"
)

const (
	// how long herdr gets to open its socket before Ovis starts anyway (Ovis retries)
	herdrReadyTimeout = 20 * time.Second
	// how long a child gets after SIGTERM before SIGKILL; the package stop timeout is 30 s
	stopGrace = 10 * time.Second
	// herdr restart backoff
	restartMin = time.Second
	restartMax = 30 * time.Second
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
	herdrBin := filepath.Join(payload, "herdr")
	t, err := token.Ensure(token.Path(os.Getenv))
	if err != nil {
		return fmt.Errorf("gateway token: %w", err)
	}
	if err := telemetryDefaultOff(os.Getenv("OVIS_STATE_DIR")); err != nil {
		return fmt.Errorf("telemetry default: %w", err)
	}
	socket := os.Getenv("HERDR_SOCKET")
	configHome, err := herdrConfigHome(socket)
	if err != nil {
		return err
	}

	signals := make(chan os.Signal, 4)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	h := &herdrChild{bin: herdrBin, env: herdrEnv(os.Environ(), configHome), socket: socket}
	if err := h.start(); err != nil {
		// Ovis still runs: its sidebar then says this PC's herdr is away, and remote PCs work
		fmt.Fprintf(os.Stderr, "ovis-launch: herdr: %v\n", err)
	} else if !waitSocket(socket, herdrReadyTimeout) {
		fmt.Fprintln(os.Stderr, "ovis-launch: herdr did not open its socket in time; starting Ovis anyway")
	}

	ovis := exec.Command(bun, filepath.Join(app, "server", "index.ts"))
	ovis.Dir = app
	ovis.Env = ovisEnv(os.Environ(), t, herdrBin)
	ovis.Stdout, ovis.Stderr = os.Stdout, os.Stderr
	if err := ovis.Start(); err != nil {
		h.stop()
		return fmt.Errorf("start Ovis: %w", err)
	}
	ovisDone := make(chan error, 1)
	go func() { ovisDone <- ovis.Wait() }()

	stopping := false
	for {
		select {
		case sig := <-signals:
			if !stopping {
				stopping = true
				h.hold() // herdr stops after Ovis (below); nothing restarts it meanwhile
				_ = ovis.Process.Signal(sig)
				go func() {
					time.Sleep(stopGrace)
					_ = ovis.Process.Kill()
				}()
			}
		case err := <-ovisDone:
			h.stop()
			h.wait()
			return exitWith(err)
		}
	}
}

// exitWith ends the launcher with Ovis's own exit status.
func exitWith(err error) error {
	if err == nil {
		os.Exit(0)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			os.Exit(128 + int(status.Signal()))
		}
		os.Exit(exit.ExitCode())
	}
	return err
}

// herdrConfigHome returns the XDG config home that makes herdr listen on socket:
// herdr's server socket is $XDG_CONFIG_HOME/herdr/herdr.sock.
func herdrConfigHome(socket string) (string, error) {
	if socket == "" || !filepath.IsAbs(socket) {
		return "", fmt.Errorf("HERDR_SOCKET must be an absolute path, got %q", socket)
	}
	dir := filepath.Dir(socket)
	if filepath.Base(socket) != "herdr.sock" || filepath.Base(dir) != "herdr" {
		return "", fmt.Errorf("HERDR_SOCKET must end in /herdr/herdr.sock, got %q", socket)
	}
	return filepath.Dir(dir), nil
}

// herdrEnv is the runtime environment for herdr: without Ovis's token or any
// herdr pane variables, with its config home, and a login shell for its panes
// (the package account's own shell refuses logins).
func herdrEnv(env []string, configHome string) []string {
	out := make([]string, 0, len(env)+2)
	for _, kv := range env {
		if hasKey(kv, "OVIS_TOKEN") || hasKey(kv, "HERDR_WEB_TOKEN") || hasKey(kv, "XDG_CONFIG_HOME") || hasKey(kv, "SHELL") || strings.HasPrefix(kv, "HERDR_") {
			continue
		}
		out = append(out, kv)
	}
	return append(out, "XDG_CONFIG_HOME="+configHome, "SHELL="+paneShell())
}

func paneShell() string {
	for _, shell := range []string{"/bin/bash", "/bin/sh"} {
		if info, err := os.Stat(shell); err == nil && !info.IsDir() {
			return shell
		}
	}
	return "/bin/sh"
}

// ovisEnv is the runtime environment for Ovis: OVIS_TOKEN set to t (any
// inherited copy, and its pre-rename alias, removed) and OVIS_HERDR_BIN naming
// the bundled herdr, which Ovis runs for pane observation and terminals.
func ovisEnv(env []string, t, herdrBin string) []string {
	out := make([]string, 0, len(env)+2)
	for _, kv := range env {
		if hasKey(kv, "OVIS_HERDR_BIN") || hasKey(kv, "HERDR_WEB_HERDR_BIN") {
			continue
		}
		out = append(out, kv)
	}
	return append(withToken(out, t), "OVIS_HERDR_BIN="+herdrBin)
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

// socketAnswers reports whether something accepts connections on socket.
func socketAnswers(socket string) bool {
	conn, err := net.DialTimeout("unix", socket, time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func waitSocket(socket string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if socketAnswers(socket) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// herdrChild runs `herdr server` and starts it again with backoff when it
// exits on its own, until stop.
type herdrChild struct {
	bin, socket string
	env         []string

	mu       sync.Mutex
	cmd      *exec.Cmd
	stopped  bool // no more restarts
	signaled bool
	done     chan struct{} // closed when the supervising goroutine ends
}

func (h *herdrChild) start() error {
	if _, err := os.Stat(h.bin); err != nil {
		return err
	}
	// a herdr left over from a launcher that did not stop cleanly holds the
	// socket: it is this package's own, and the new one replaces it
	if socketAnswers(h.socket) {
		stop := exec.Command(h.bin, "server", "stop")
		stop.Env = h.env
		_ = stop.Run()
		for i := 0; i < 100 && socketAnswers(h.socket); i++ {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if err := os.MkdirAll(filepath.Dir(h.socket), 0o700); err != nil {
		return err
	}
	cmd, err := h.spawn()
	if err != nil {
		return err
	}
	h.cmd = cmd
	h.done = make(chan struct{})
	go h.supervise()
	return nil
}

func (h *herdrChild) spawn() (*exec.Cmd, error) {
	cmd := exec.Command(h.bin, "server")
	cmd.Env = h.env
	cmd.Dir = filepath.Dir(filepath.Dir(h.socket)) // the config home, created with the socket folder
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	// its own process group: a terminal signal to the launcher's group does not reach it first
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd, cmd.Start()
}

func (h *herdrChild) supervise() {
	defer close(h.done)
	delay := restartMin
	for {
		h.mu.Lock()
		cmd := h.cmd
		h.mu.Unlock()
		started := time.Now()
		err := cmd.Wait()
		h.mu.Lock()
		stopped := h.stopped
		h.mu.Unlock()
		if stopped {
			return
		}
		fmt.Fprintf(os.Stderr, "ovis-launch: herdr exited (%v); starting it again in %s\n", err, delay)
		if time.Since(started) > 5*time.Minute {
			delay = restartMin
		}
		time.Sleep(delay)
		delay = min(delay*2, restartMax)
		h.mu.Lock()
		if h.stopped {
			h.mu.Unlock()
			return
		}
		next, err := h.spawn()
		if err != nil {
			h.mu.Unlock()
			fmt.Fprintf(os.Stderr, "ovis-launch: herdr: %v\n", err)
			continue
		}
		h.cmd = next
		h.mu.Unlock()
	}
}

// stop tells herdr to stop (SIGTERM, then SIGKILL after stopGrace) and keeps it
// from being started again. It returns at once; wait waits for the exit.
func (h *herdrChild) stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stopped = true
	if h.signaled || h.cmd == nil {
		return
	}
	h.signaled = true
	cmd, done := h.cmd, h.done
	_ = cmd.Process.Signal(syscall.SIGTERM)
	go func() {
		select {
		case <-done:
		case <-time.After(stopGrace):
			_ = cmd.Process.Kill()
		}
	}()
}

// hold keeps herdr running but stops restarting it.
func (h *herdrChild) hold() {
	h.mu.Lock()
	h.stopped = true
	h.mu.Unlock()
}

func (h *herdrChild) wait() {
	h.mu.Lock()
	done := h.done
	h.mu.Unlock()
	if done != nil {
		<-done
	}
}

// telemetryDefaultOff makes upstream's anonymous usage counts start off while
// leaving its own Settings > About switch in the user's hands. On a first start
// (no telemetry.json and no mark yet) it writes upstream's own off mark,
// telemetry-off, in the state directory. Turning the switch on removes that
// mark and saves telemetry.json, so later starts leave the choice alone.
func telemetryDefaultOff(stateDir string) error {
	if stateDir == "" {
		return nil
	}
	for _, name := range []string{"telemetry.json", "telemetry-off"} {
		if _, err := os.Lstat(filepath.Join(stateDir, name)); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(stateDir, "telemetry-off"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	return f.Close()
}
