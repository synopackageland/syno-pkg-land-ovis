package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// OpenSSH and Rust use libc account lookup independently of HOME. nss_wrapper
// supplies the arbitrary package UID without editing the read-only /etc/passwd
// or running a user-management program as root.
func prepareIdentity(home string) error {
	if os.Geteuid() == 0 {
		return fmt.Errorf("Ovis must run as the package UID")
	}
	if !filepath.IsAbs(home) || strings.ContainsAny(home, ":\n\r") {
		return fmt.Errorf("invalid package HOME")
	}
	dir, err := os.MkdirTemp("/tmp", "ovis-identity-")
	if err != nil {
		return err
	}
	uid, gid := strconv.Itoa(os.Geteuid()), strconv.Itoa(os.Getegid())
	passwd, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return err
	}
	var rows []string
	for _, row := range strings.Split(strings.TrimSpace(string(passwd)), "\n") {
		fields := strings.Split(row, ":")
		if len(fields) == 7 && (fields[2] == uid || fields[0] == "ovis") {
			continue
		}
		rows = append(rows, row)
	}
	rows = append(rows, "ovis:x:"+uid+":"+gid+":Ovis:"+home+":/bin/bash")
	if err = os.WriteFile(filepath.Join(dir, "passwd"), []byte(strings.Join(rows, "\n")+"\n"), 0600); err != nil {
		return err
	}
	groups, err := os.ReadFile("/etc/group")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(dir, "group"), append(groups, []byte("\novis:x:"+gid+":\n")...), 0600); err != nil {
		return err
	}
	for key, value := range map[string]string{"NSS_WRAPPER_PASSWD": filepath.Join(dir, "passwd"), "NSS_WRAPPER_GROUP": filepath.Join(dir, "group"), "LD_PRELOAD": "/usr/local/lib/libnss_wrapper.so"} {
		if err = os.Setenv(key, value); err != nil {
			return err
		}
	}
	return nil
}
