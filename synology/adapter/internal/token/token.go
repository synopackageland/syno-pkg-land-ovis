// Package token keeps the per-install OVIS_TOKEN that the DSM identity gateway
// hands to Ovis on behalf of an admitted DSM administrator.
//
// Ovis supports a sign-in proxy in front of it: the server is started with
// OVIS_TOKEN, and the proxy sends "Authorization: Bearer <token>" on every
// request it lets through, the /ws upgrade included (upstream docs/guide.md,
// "If the proxy already signs people in"). The token is random per install,
// lives only in a package-private 0600 file and never reaches the browser.
package token

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Package is the DSM package ID; its var folder is the fallback state root.
const Package = "Ovis"

// file is the token's name below the package's private var folder.
const file = "gateway/ovis-token"

// Path is the token file: below SYNOPKG_PKGVAR when the runtime sets it,
// otherwise below DSM's fixed var folder for this package.
func Path(env func(string) string) string {
	root := env("SYNOPKG_PKGVAR")
	if root == "" || !filepath.IsAbs(root) {
		root = "/var/packages/" + Package + "/var"
	}
	return filepath.Join(root, file)
}

// valid reports whether t has the shape Ensure writes: 64 lowercase hex digits.
func valid(t string) bool {
	if len(t) != 64 {
		return false
	}
	for _, c := range t {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// Read returns the stored token, refusing a missing, malformed or
// group/world-readable file rather than sending a credential Ovis would refuse.
func Read(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("%s must be a regular file with mode 0600", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	t := strings.TrimSpace(string(b))
	if !valid(t) {
		return "", fmt.Errorf("%s does not hold a valid token", path)
	}
	return t, nil
}

// Ensure returns the stored token, creating it on first start: 32 random
// bytes, hex, written to a private temporary file and renamed into place.
func Ensure(path string) (string, error) {
	if t, err := Read(path); err == nil {
		return t, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	t := hex.EncodeToString(raw)
	tmp, err := os.CreateTemp(dir, ".ovis-token-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", err
	}
	if _, err := tmp.WriteString(t + "\n"); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", err
	}
	return t, nil
}
