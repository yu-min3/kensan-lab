package main

import (
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestPublisherCLIHelper(t *testing.T) {
	raw := os.Getenv("SENSE_PUBLISHER_CLI_TEST_ARGS")
	if raw == "" {
		return
	}
	var args []string
	if json.Unmarshal([]byte(raw), &args) != nil {
		t.Fatal("invalid helper args")
	}
	os.Args = append([]string{os.Args[0]}, args...)
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	main()
}
func TestPublisherCLIRequiresSeparateCredentialFlags(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		message string
		success bool
	}{
		{name: "daemon missing package", args: []string{"-serve-socket", "/unavailable/socket", "-controller-auth-file", "/unavailable/auth", "-token-file", "/unavailable/repo"}, message: "separate repository/package tokens"},
		{name: "observer missing package", args: []string{"-serve-socket", "/unavailable/socket", "-controller-auth-file", "/unavailable/auth", "-token-file", "/unavailable/repo", "-package-token-file", "/unavailable/package", "-observer-plan", "/unavailable/plan", "-observer-kubeconfig", "/unavailable/kube", "-observer-token-file", "/unavailable/observer"}, message: "repository and package observation tokens required together"},
		{name: "observer package without daemon", args: []string{"-observer-package-token-file", "/unavailable/package"}, message: "bridge options require serve-socket"},
		{name: "one-shot missing package", args: []string{"-data", "/unavailable/data", "-decision", "decision", "-token-file", "/unavailable/repo"}, message: "separate repository/package token files are required"},
		{name: "help advertises package flags", args: []string{"-h"}, message: "-observer-package-token-file", success: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(tc.args)
			cmd := exec.Command(os.Args[0], "-test.run=^TestPublisherCLIHelper$")
			cmd.Env = append(os.Environ(), "SENSE_PUBLISHER_CLI_TEST_ARGS="+string(raw))
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.success || !strings.Contains(string(out), tc.message) {
				t.Fatalf("unexpected CLI boundary outcome: %s", out)
			}
		})
	}
}
