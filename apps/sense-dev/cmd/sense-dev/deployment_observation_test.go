package main

import (
	"context"
	"encoding/json"
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type observationFixture struct {
	calls    int
	mismatch bool
}

func (o *observationFixture) Observe(_ context.Context, i core.PublishIntent, task string) (core.DeploymentReceipt, error) {
	o.calls++
	r := core.DeploymentReceipt{ObservedRelease: "v2", TaskID: task, DecisionID: i.DecisionID, IntentID: i.ID, HeadSHA: i.HeadSHA, Revision: i.ExternalID, ImageSourceSHA: i.ExternalID, ImageDigest: "sha256:" + strings.Repeat("d", 64), Environment: "private-canary", Status: "healthy", UserPath: "/api/release"}
	if o.mismatch {
		r.HeadSHA = strings.Repeat("c", 40)
	}
	return r, nil
}
func TestHostObservationRecordsOnceAndRejectsChangedIdentity(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		t.Run(map[bool]string{false: "valid", true: "wrong head"}[mismatch], func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Chmod(dir, 0700); err != nil {
				t.Fatal(err)
			}
			st := core.NewState()
			head := strings.Repeat("a", 40)
			revision := strings.Repeat("b", 40)
			st.Tasks["task"] = core.Task{ID: "task", Kind: "change", Team: core.App, MissionID: "mission", HeadSHA: head, Status: "publish_wait"}
			st.Agents["author"] = core.Agent{ID: "author", TaskID: "task", Team: core.App}
			st.Decisions["decision"] = core.ReleaseDecision{ID: "decision", AuthorAgentID: "author", Repository: "yu-min3/kensan-lab", Ref: "refs/heads/sense-dev/fixture", Verdict: "allow", Operation: "merge", HeadSHA: head, TargetEnvironment: "private-canary", PolicyVersion: core.ReleasePolicyVersion, ExpiresAt: time.Now().Add(time.Hour)}
			st.Intents["intent"] = core.PublishIntent{ID: "intent", DecisionID: "decision", Operation: "merge", HeadSHA: head, Status: "sent", ExternalID: revision, AuthorizedAt: time.Now().Add(-time.Second), ExpiresAt: time.Now().Add(time.Minute), Repository: "yu-min3/kensan-lab", Ref: "refs/heads/sense-dev/fixture", TargetEnvironment: "private-canary", PolicyVersion: core.ReleasePolicyVersion}
			b, _ := json.Marshal(st)
			if err := os.WriteFile(filepath.Join(dir, "state.json"), b, 0600); err != nil {
				t.Fatal(err)
			}
			s, err := core.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			o := &observationFixture{mismatch: mismatch}
			// Another mission cannot trigger host probes or receipt adoption.
			if err := observeDeployments(context.Background(), s, o, "other"); err != nil || o.calls != 0 {
				t.Fatal("cross mission observation")
			}
			err = observeDeployments(context.Background(), s, o, "mission")
			if mismatch {
				if err == nil || len(s.Snapshot().Deployments) != 0 {
					t.Fatal("changed receipt adopted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := observeDeployments(context.Background(), s, o, "mission"); err != nil || o.calls != 1 {
				t.Fatal("duplicate receipt repeated probe")
			}
			if r := s.Snapshot().Deployments["task"]; r.Revision != revision || r.HeadSHA != head || r.EvidenceRef.ID == "" {
				t.Fatal("host observation not stored")
			}
		})
	}
}
