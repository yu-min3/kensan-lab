package core

import (
	"context"
	"errors"
	"testing"
)

type unknownImageTransport struct {
	executions int
	accepted   bool
}

func (f *unknownImageTransport) Inspect(_ context.Context, i PublishIntent) (string, bool, error) {
	if f.accepted {
		return i.ImageRelease.DispatchID, true, nil
	}
	return "", false, nil
}
func (f *unknownImageTransport) Execute(context.Context, PublishIntent) (string, error) {
	f.executions++
	return "", errors.New("dispatch response lost")
}
func TestImagePublisherUnknownIsInspectOnly(t *testing.T) {
	s, origin, candidate, request := classifiedReleaseFixture(t, "image_publish")
	if _, err := s.DecideApproval(request.ID, "tap", origin.Operation, origin.HeadSHA, "approved"); err != nil {
		t.Fatal(err)
	}
	_, gate := taskAgent(t, s, Platform, "release_gate")
	if err := s.BindReleaseGateInputs(gate.ID, origin.AuthorAgentID, origin.ArtifactRefs, origin.ScanRef, candidate); err != nil {
		t.Fatal(err)
	}
	d := gateFixtureDecision(t, s, gate.ID, origin.AuthorAgentID, origin.ArtifactRefs[0], origin.ScanRef, "allow", "fresh independent image review")
	d.ApprovalID = request.ID
	d, err := s.RecordReleaseDecision(d)
	if err != nil {
		t.Fatal(err)
	}
	transport := &unknownImageTransport{}
	if _, err := s.RunPublish(context.Background(), d.ID, transport); err == nil {
		t.Fatal("ambiguous result ignored")
	}
	if _, err := s.RunPublish(context.Background(), d.ID, transport); err == nil || transport.executions != 1 {
		t.Fatal("ambiguous image dispatch resent")
	}
	transport.accepted = true
	result, err := s.RunPublish(context.Background(), d.ID, transport)
	if err != nil || result.Status != "sent" || result.ExternalID != candidate.ImageRelease.DispatchID || transport.executions != 1 {
		t.Fatalf("reconciliation %+v %v", result, err)
	}
	if !imageReleasesEqual(result.ImageRelease, candidate.ImageRelease) {
		t.Fatal("reconciliation changed fixed spec")
	}
}
