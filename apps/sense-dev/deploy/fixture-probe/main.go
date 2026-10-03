// Credential-free boundary check. Never prints file contents or env values.
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	checks := map[string]bool{}
	for _, p := range []string{"/agent-auth/auth.json", "/agent-auth/provider-secret.txt", "/workspace/auth-alias"} {
		f, err := os.Open(p)
		checks["deny_read:"+p] = os.IsPermission(err)
		if err == nil {
			f.Close()
		}
	}
	for _, p := range []string{"/var/lib/kensan-dev", "/opt/kensan-dev/auth", "/run/systemd/private", "/var/run/docker.sock"} {
		_, err := os.Stat(p)
		checks["hidden:"+p] = os.IsNotExist(err)
	}
	for _, addr := range []string{"127.0.0.1:8787", "192.168.0.113:8787", "192.168.0.113:11434", "1.1.1.1:443"} {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		checks["no_connect:"+addr] = err != nil
		if conn != nil {
			conn.Close()
		}
	}
	for _, name := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "OPENAI_API_KEY", "KENSAN_ADMIN_TOKEN"} {
		_, exists := os.LookupEnv(name)
		checks["no_env:"+name] = !exists
	}
	entries, err := os.ReadDir("/proc/self/fd")
	checks["fd_inventory"] = err == nil
	checks["no_auth_fd"] = true
	for _, e := range entries {
		target, err := os.Readlink("/proc/self/fd/" + e.Name())
		if err == nil && (strings.Contains(target, "agent-auth") || strings.Contains(target, "provider-secret")) {
			checks["no_auth_fd"] = false
		}
	}
	passed := true
	for _, ok := range checks {
		passed = passed && ok
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"passed": passed, "checks": checks}); err != nil {
		fmt.Fprintln(os.Stderr, "cannot encode checks")
		os.Exit(2)
	}
	if !passed {
		os.Exit(1)
	}
}
