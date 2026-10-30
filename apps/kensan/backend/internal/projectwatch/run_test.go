package projectwatch

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/agentclient"
)

type fake struct {
	payloads   [][]byte
	creates    int
	keys       []string
	failCreate bool
	output     string
	state      string
}

func (f *fake) Create(_ context.Context, r agentclient.Request) (agentclient.Job, error) {
	f.creates++
	f.keys = append(f.keys, r.IdempotencyKey)
	payload, _ := json.Marshal(r)
	f.payloads = append(f.payloads, payload)
	if f.failCreate {
		f.failCreate = false
		return agentclient.Job{}, os.ErrDeadlineExceeded
	}
	return agentclient.Job{ID: "j1", State: "running"}, nil
}
func (f *fake) Get(_ context.Context, id string) (agentclient.Job, error) {
	s := f.state
	if s == "" {
		s = "succeeded"
	}
	return agentclient.Job{ID: id, State: s, Output: json.RawMessage(f.output)}, nil
}
func (f *fake) Action(context.Context, string, string) (agentclient.Job, error) {
	panic("no automatic retry")
}
func put(t *testing.T, root, path, text string) {
	t.Helper()
	p := filepath.Join(root, path)
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(text), 0600); e != nil {
		t.Fatal(e)
	}
}

const project = `---
status: active
---
## 現在地
2026-10-30: 試作を使って確認する
## マイルストーン
- [ ] 実機で確認する
## ログ
- 2026-10-29: 試作を完成した
- 2026-10-30: テストした
## 関連ノート
[[other]]
`

var now = time.Date(2026, 10, 30, 10, 0, 0, 0, time.FixedZone("JST", 9*3600))

const goodOutput = `{"items":[{"project":"sample","change":"テストした","meaning":"実装を確認できた","decision":"","sourceIds":["project:sample"]}]}`

func TestResumeAndNoChange(t *testing.T) {
	root := t.TempDir()
	put(t, root, "projects/sample/README.md", project)
	put(t, root, "daily/2026/10/30.md", "PRIVATE DIARY")
	f := &fake{failCreate: true, output: goodOutput}
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		s, e := Step(ctx, root, f, now)
		if e != nil {
			t.Fatal(e)
		}
		if i == 1 && s.Status != "connection_error" {
			t.Fatalf("%+v", s)
		}
	}
	if f.creates != 2 || f.keys[0] != f.keys[1] || string(f.payloads[0]) != string(f.payloads[1]) {
		t.Fatalf("lost response must reuse request: %+v", f)
	}
	s, e := Step(ctx, root, f, now)
	if e != nil || s.Status != "unchanged" {
		t.Fatalf("%+v %v", s, e)
	}
	v, e := Read(root)
	if e != nil || v.Result == nil || len(v.Result.Items) != 1 {
		t.Fatalf("%+v %v", v, e)
	}
	if len(v.Snapshot.Projects) != 1 || v.Snapshot.Projects[0].Date != "2026-10-30" {
		t.Fatalf("log date sort: %+v", v.Snapshot.Projects)
	}
	b, _ := json.Marshal(v)
	if strings.Contains(string(b), "PRIVATE DIARY") || strings.Contains(string(b), "pending") {
		t.Fatal("private or pending input exposed")
	}
	actual, _ := os.ReadFile(filepath.Join(root, "projects/sample/README.md"))
	if string(actual) != project {
		t.Fatal("human record modified")
	}
}
func TestRejectInventedEvidencePreservePrevious(t *testing.T) {
	root := t.TempDir()
	put(t, root, "projects/sample/README.md", project)
	put(t, root, ResultPath, `{"version":1,"hash":"old","generatedAt":"2026-10-29T10:00:00+09:00","items":[],"sources":[]}`)
	f := &fake{output: `{"items":[{"project":"sample","change":"完了","meaning":"","decision":"","sourceIds":["invented"]}]}`}
	for i := 0; i < 3; i++ {
		if _, e := Step(context.Background(), root, f, now); e != nil {
			t.Fatal(e)
		}
	}
	v, e := Read(root)
	if e != nil || v.State.Status != "failed" || v.Result.Hash != "old" {
		t.Fatalf("%+v %v", v, e)
	}
}
func TestLimitAndInputBounds(t *testing.T) {
	root := t.TempDir()
	put(t, root, "projects/sample/README.md", project)
	put(t, root, StatePath, `{"version":1,"status":"succeeded","day":"2026-10-30","attempts":8}`)
	s, e := Step(context.Background(), root, &fake{}, now)
	if e != nil || s.Status != "daily_limit" {
		t.Fatalf("%+v %v", s, e)
	}
	put(t, root, "projects/sample/README.md", project+"\n## 現在地\n")
	put(t, root, "feeds/2026/10/30.md", strings.Repeat("x", 32001))
	if _, e = Collect(root); e == nil {
		t.Fatal("oversized input silently accepted")
	}
}
func TestPendingSnapshotStaysFixed(t *testing.T) {
	root := t.TempDir()
	put(t, root, "projects/sample/README.md", project)
	f := &fake{output: goodOutput}
	if _, e := Step(context.Background(), root, f, now); e != nil {
		t.Fatal(e)
	}
	put(t, root, "projects/sample/README.md", strings.Replace(project, "テストした", "後から別の記録に変えた", 1))
	for i := 0; i < 2; i++ {
		if _, e := Step(context.Background(), root, f, now); e != nil {
			t.Fatal(e)
		}
	}
	v, e := Read(root)
	if e != nil || !v.Stale || !strings.Contains(v.Result.Sources[0].Text, "テストした") {
		t.Fatalf("fixed snapshot lost: %+v %v", v, e)
	}
}
func TestLockAndSymlinkBoundary(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	put(t, outside, "sample/README.md", project)
	if e := os.Symlink(outside, filepath.Join(root, "projects")); e != nil {
		t.Fatal(e)
	}
	if _, e := Collect(root); e == nil {
		t.Fatal("outside root followed")
	}
}
func TestFailedDoesNotAutoRetry(t *testing.T) {
	root := t.TempDir()
	put(t, root, "projects/sample/README.md", project)
	f := &fake{state: "failed"}
	for i := 0; i < 5; i++ {
		if _, e := Step(context.Background(), root, f, now); e != nil {
			t.Fatal(e)
		}
	}
	if f.creates != 1 {
		t.Fatal("terminal failure retried")
	}
	if e := ResetFailed(context.Background(), root, f); e != nil {
		t.Fatal(e)
	}
	if _, e := Step(context.Background(), root, f, now); e != nil {
		t.Fatal(e)
	}
	if _, e := Step(context.Background(), root, f, now); e != nil {
		t.Fatal(e)
	}
	if f.creates != 2 || f.keys[0] == f.keys[1] {
		t.Fatal("explicit retry must have new key")
	}
}
func TestProcessLockPreventsDuplicateSubmission(t *testing.T) {
	root := t.TempDir()
	put(t, root, "projects/sample/README.md", project)
	put(t, root, "feeds/state/project-watch.lock", "")
	f, e := os.OpenFile(filepath.Join(root, "feeds/state/project-watch.lock"), os.O_RDWR, 0600)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		t.Fatal(e)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	g := &fake{}
	if _, e = Step(context.Background(), root, g, now); e == nil || g.creates != 0 {
		t.Fatal("concurrent submission accepted")
	}
}
func TestResetCannotForgetRunningJob(t *testing.T) {
	root := t.TempDir()
	put(t, root, "projects/sample/README.md", project)
	f := &fake{state: "failed"}
	for i := 0; i < 3; i++ {
		if _, e := Step(context.Background(), root, f, now); e != nil {
			t.Fatal(e)
		}
	}
	f.state = "running"
	if e := ResetFailed(context.Background(), root, f); e == nil {
		t.Fatal("running job discarded")
	}
}
