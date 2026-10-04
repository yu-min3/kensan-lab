package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type unknownMergeTransport struct {
	executions int
	merged     bool
}

func (f *unknownMergeTransport) Inspect(context.Context, PublishIntent) (string, bool, error) {
	if f.merged {
		return strings.Repeat("d", 40), true, nil
	}
	return "", false, nil
}

func (f *unknownMergeTransport) Execute(context.Context, PublishIntent) (string, error) {
	f.executions++
	return "", errors.New("response lost")
}

func TestGitOpsPublisherUnknownIsInspectOnly(t *testing.T) {
	for _, operation := range []string{"merge", "deploy"} {
		t.Run(operation, func(t *testing.T) {
			s, origin, candidate, request := classifiedReleaseFixture(t, operation)
			if _, err := s.DecideApproval(request.ID, "tap", operation, origin.HeadSHA, "approved"); err != nil {
				t.Fatal(err)
			}
			_, gate := taskAgent(t, s, Platform, "release_gate")
			if err := s.BindReleaseGateInputs(gate.ID, origin.AuthorAgentID, origin.ArtifactRefs, origin.ScanRef, candidate); err != nil {
				t.Fatal(err)
			}
			d := gateFixtureDecision(t, s, gate.ID, origin.AuthorAgentID, origin.ArtifactRefs[0], origin.ScanRef, "allow", "approved and independently verified")
			d.ApprovalID = request.ID
			d, err := s.RecordReleaseDecision(d)
			if err != nil {
				t.Fatal(err)
			}
			transport := &unknownMergeTransport{}
			if _, err := s.RunPublish(context.Background(), d.ID, transport); err == nil {
				t.Fatal("unknown response ignored")
			}
			if _, err := s.RunPublish(context.Background(), d.ID, transport); err == nil || transport.executions != 1 {
				t.Fatal("unknown merge automatically retried")
			}
			transport.merged = true
			result, err := s.RunPublish(context.Background(), d.ID, transport)
			if err != nil || result.Status != "sent" || result.ExternalID != strings.Repeat("d", 40) || result.HeadSHA != origin.HeadSHA || transport.executions != 1 {
				t.Fatalf("merge reconciliation: %+v %v", result, err)
			}
			if result.BaseSHA != strings.Repeat("b", 40) || result.TargetEnvironment != "private-canary" || result.PolicyVersion != ReleasePolicyVersion || result.ExpiresAt.IsZero() {
				t.Fatal("publisher operation lacks fixed policy inputs")
			}
		})
	}
}
