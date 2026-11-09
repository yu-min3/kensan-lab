package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

var helperPath, helperHash string

func TestMain(m *testing.M) {
	dir, e := os.MkdirTemp("", "llmops-adapter-test-")
	if e != nil {
		panic(e)
	}
	source := `package main
import("encoding/json";"os";"strings";"time")
func main(){var r struct{Protocol string ` + "`json:\"protocol_version\"`" + `;ID string ` + "`json:\"request_id\"`" + `;Schema string ` + "`json:\"domain_schema_version\"`" + `;Payload map[string]any ` + "`json:\"payload\"`" + `};json.NewDecoder(os.Stdin).Decode(&r);mode,_:=r.Payload["mode"].(string);switch mode{case "timeout":time.Sleep(30*time.Second);case "large":os.Stdout.WriteString(strings.Repeat("x",100000));return;case "stderr":os.Stderr.WriteString(strings.Repeat("x",100000));return;case "truncated":os.Stdout.WriteString("{\"protocol_version\":");return;case "badversion":r.Protocol="2";case "badid":r.ID="other";case "touch":os.WriteFile(r.Payload["marker"].(string),[]byte("touched"),0600)};cwd,_:=os.Getwd();json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol_version":r.Protocol,"request_id":r.ID,"result":map[string]any{"secret":os.Getenv("WORKER_TEST_SECRET"),"home":os.Getenv("HOME"),"cwd":cwd,"schema":r.Schema}})}`
	file := filepath.Join(dir, "main.go")
	if e = os.WriteFile(file, []byte(source), 0600); e != nil {
		panic(e)
	}
	helperPath = filepath.Join(dir, "helper")
	if out, e := exec.Command("go", "build", "-o", helperPath, file).CombinedOutput(); e != nil {
		panic(string(out))
	}
	b, _ := os.ReadFile(helperPath)
	sum := sha256.Sum256(b)
	helperHash = hex.EncodeToString(sum[:])
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
func config(t *testing.T) Config {
	t.Helper()
	root := t.TempDir()
	if e := os.Chmod(root, 0700); e != nil {
		t.Fatal(e)
	}
	return Config{App: "sample", Role: "transform", TempRoot: root, TimeoutMillis: 1000, MaxOutputBytes: 1024, Adapters: map[string]Adapter{"sample": {Binary: helperPath, SHA256: helperHash, DomainSchemaVersion: "sample/1"}}}
}
func request(mode string) Request {
	b, _ := json.Marshal(map[string]any{"mode": mode})
	return Request{ProtocolVersion: "1", RequestID: "request-1", Operation: "prepare", DomainSchemaVersion: "sample/1", Payload: b}
}
func TestCleanEnvironmentAndWorkspace(t *testing.T) {
	t.Setenv("WORKER_TEST_SECRET", "must-not-cross-boundary")
	c := config(t)
	r, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	res, e := r.Call(context.Background(), "sample", request("ok"))
	if e != nil {
		t.Fatal(e)
	}
	var body struct{ Secret, Home, Cwd, Schema string }
	if json.Unmarshal(res.Result, &body) != nil || body.Secret != "" || body.Home != body.Cwd || body.Schema != "sample/1" {
		t.Fatalf("unexpected response: %s", res.Result)
	}
	if _, e = os.Stat(body.Cwd); !os.IsNotExist(e) {
		t.Fatal("workspace retained")
	}
	if r.App() != "sample" || r.Role() != "transform" {
		t.Fatal("identity changed")
	}
}
func TestRequestCannotExecuteSideEffect(t *testing.T) {
	c := config(t)
	r, _ := New(c)
	marker := filepath.Join(t.TempDir(), "marker")
	req := request("touch")
	req.Payload, _ = json.Marshal(map[string]any{"mode": "touch", "marker": marker})
	req.Operation = "exec"
	if _, e := r.Call(context.Background(), "sample", req); !errors.Is(e, ErrRequest) {
		t.Fatal(e)
	}
	if _, e := os.Stat(marker); !os.IsNotExist(e) {
		t.Fatal("invalid request reached child")
	}
	if _, e := r.Call(context.Background(), "/bin/sh", request("ok")); !errors.Is(e, ErrRequest) {
		t.Fatal("unregistered executable accepted")
	}
}
func TestResponseFailuresAndDeadlines(t *testing.T) {
	for _, tc := range []struct {
		mode string
		want error
	}{{"truncated", ErrProtocol}, {"badversion", ErrProtocol}, {"badid", ErrProtocol}, {"large", ErrLimit}, {"stderr", ErrLimit}, {"timeout", ErrTimeout}} {
		t.Run(tc.mode, func(t *testing.T) {
			c := config(t)
			c.TimeoutMillis = 100
			r, _ := New(c)
			start := time.Now()
			if _, e := r.Call(context.Background(), "sample", request(tc.mode)); !errors.Is(e, tc.want) {
				t.Fatalf("got %v want %v", e, tc.want)
			}
			if time.Since(start) > 2*time.Second {
				t.Fatal("child was not terminated promptly")
			}
		})
	}
}
func TestExecutableAndConfigBinding(t *testing.T) {
	c := config(t)
	c.Adapters["sample"] = Adapter{Binary: helperPath, SHA256: hex.EncodeToString(make([]byte, 32)), DomainSchemaVersion: "sample/1"}
	r, _ := New(c)
	if _, e := r.Call(context.Background(), "sample", request("ok")); !errors.Is(e, ErrExecution) {
		t.Fatal("modified binary accepted")
	}
	c = config(t)
	r, _ = New(c)
	c.Adapters["sample"] = Adapter{}
	if _, e := r.Call(context.Background(), "sample", request("ok")); e != nil {
		t.Fatal("caller mutated registry")
	}
	c = config(t)
	os.Chmod(c.TempRoot, 0755)
	if _, e := New(c); !errors.Is(e, ErrConfig) {
		t.Fatal("public temporary directory accepted")
	}
}
func TestCredentialRoleAndPermissions(t *testing.T) {
	c := config(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "transform-sample.json")
	os.WriteFile(file, []byte(`{"access_key":"test-key-123456789","secret_key":"test-secret-1234567890123456789"}`), 0600)
	c.Storage = &Storage{Endpoint: "http://127.0.0.1:19300", Region: "us-east-1", Bucket: "llmops-dev", CredentialFile: file}
	r, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	s, e := r.Objects()
	if e != nil || s.AppID() != "sample" {
		t.Fatal(e)
	}
	c.Role = "eval"
	if _, e := New(c); !errors.Is(e, ErrConfig) {
		t.Fatal("other role credential accepted")
	}
	c.Role = "transform"
	os.Chmod(file, 0644)
	if _, e := New(c); !errors.Is(e, ErrConfig) {
		t.Fatal("public credential accepted")
	}
}

func TestCallerCancellation(t *testing.T) {
	r, e := New(config(t))
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = r.Call(ctx, "sample", request("timeout")); !errors.Is(e, context.Canceled) {
		t.Fatalf("got %v", e)
	}
}

func TestSingleHostWriterLock(t *testing.T) {
	r, e := New(config(t))
	if e != nil {
		t.Fatal(e)
	}
	executed := false
	e = r.WithLock("dataset-publish", func() error {
		inner := r.WithLock("dataset-publish", func() error { executed = true; return nil })
		if inner == nil {
			return errors.New("second writer entered")
		}
		return nil
	})
	if e != nil || executed {
		t.Fatal("writer exclusion failed")
	}
	if e = r.WithLock("dataset-publish", func() error { return nil }); e != nil {
		t.Fatal("lock retained after completion")
	}
}
