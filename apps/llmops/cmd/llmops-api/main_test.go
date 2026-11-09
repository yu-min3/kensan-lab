package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPublicHTTPRefusedBeforeServing(t *testing.T) {
	if _, _, e := server(context.Background(), config{Listen: "0.0.0.0:8080"}); e == nil {
		t.Fatal("public plaintext API accepted")
	}
}
func TestPrivateCredentialAndStrictConfiguration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.json")
	os.WriteFile(path, []byte(`{"access_key":"fixture","secret_key":"fixture-secret"}`), 0644)
	if _, e := credentialsFile(path).Retrieve(context.Background()); e == nil {
		t.Fatal("world-readable credential accepted")
	}
	os.Chmod(path, 0600)
	if c, e := credentialsFile(path).Retrieve(context.Background()); e != nil || c.AccessKeyID != "fixture" || !c.CanExpire {
		t.Fatal("private credentials rejected or never refreshed", e)
	}
	cfg := filepath.Join(dir, "config.json")
	os.WriteFile(cfg, []byte(`{"listen":"127.0.0.1:19400","admin_token":"forbidden"}`), 0600)
	if _, e := readConfig(cfg); e == nil {
		t.Fatal("unknown configuration field accepted")
	}
}
