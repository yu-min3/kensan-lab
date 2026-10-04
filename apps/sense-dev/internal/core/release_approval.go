package core

import (
	"encoding/json"
	"errors"
	"reflect"
	"time"
)

func approvalMatches(st State, d ReleaseDecision, scan ReleaseScan, candidateHash string) bool {
	return approvalMatchesAt(st, d, scan, candidateHash, time.Now())
}

func approvalMatchesAt(st State, d ReleaseDecision, scan ReleaseScan, candidateHash string, at time.Time) bool {
	r, ok := st.Approvals[d.ApprovalID]
	origin, exists := st.Decisions[r.DecisionID]
	if !ok || !exists || r.Status != "approved" || r.DecidedAt == nil || r.ActionID == "" || !at.Before(r.ExpiresAt) || !at.Before(origin.ExpiresAt) || r.DecidedAt.After(at) || r.PolicyVersion != ReleasePolicyVersion || origin.Verdict != "needs_human" || scan.Status == "deny" || d.GateAgentID == origin.GateAgentID {
		return false
	}
	if origin.PolicyVersion != r.PolicyVersion || origin.Operation != r.Operation || origin.Repository != r.Repository || origin.Ref != r.Ref || origin.HeadSHA != r.HeadSHA || origin.TargetEnvironment != r.Environment || r.ExpiresAt.After(origin.ExpiresAt) || !reflect.DeepEqual(origin.HumanCategories, r.HumanCategories) || !reflect.DeepEqual(origin.HumanReasons, r.HumanReasons) {
		return false
	}
	if r.AuthorAgentID != d.AuthorAgentID || origin.AuthorAgentID != d.AuthorAgentID || r.Operation != d.Operation || r.Repository != d.Repository || r.Ref != d.Ref || r.HeadSHA != d.HeadSHA || r.Environment != d.TargetEnvironment || r.ScanSHA256 == "" || r.ScanSHA256 != d.ScanRef.SHA256 || origin.ScanRef != d.ScanRef || r.CandidateSHA256 == "" || candidateHash != "" && r.CandidateSHA256 != candidateHash || !reflect.DeepEqual(r.HumanCategories, d.HumanCategories) || !reflect.DeepEqual(r.HumanReasons, d.HumanReasons) {
		return false
	}
	gate, oldGate := st.Agents[d.GateAgentID], st.Agents[origin.GateAgentID]
	return gate.ID != "" && gate.ID != d.AuthorAgentID && (gate.SessionID == "" || gate.SessionID != oldGate.SessionID) && st.Tasks[st.Agents[d.AuthorAgentID].TaskID].HeadSHA == d.HeadSHA
}

func (s *Store) boundApprovalMatches(d ReleaseDecision, scan ReleaseScan, candidate ArtifactRef) error {
	if len(d.HumanCategories) == 0 {
		if d.ApprovalID != "" {
			return errors.New("approval supplied for unclassified scan")
		}
		return nil
	}
	st := s.Snapshot()
	if !approvalMatches(st, d, scan, candidate.SHA256) {
		return errors.New("live approval does not match exact release candidate")
	}
	r := st.Approvals[d.ApprovalID]
	inputs := st.Agents[d.GateAgentID].ReviewInputs
	if len(inputs) != len(d.ArtifactRefs)+5 {
		return errors.New("fresh gate did not receive human approval")
	}
	ref := inputs[len(inputs)-2]
	a := st.Artifacts[ref.ID]
	if a.AgentID != "system" || a.Kind != "human_approval" || s.verifyRef(ref) != nil {
		return errors.New("controller approval evidence missing")
	}
	body, err := s.ReadArtifact(ref.ID)
	var bound ApprovalRequest
	if err != nil || json.Unmarshal(body, &bound) != nil || !reflect.DeepEqual(bound, r) {
		return errors.New("gate approval evidence differs from live record")
	}
	for _, attempt := range st.Attempts {
		if attempt.AgentID == d.GateAgentID && attempt.Status == "completed" && attempt.OutputRef != nil && attempt.InputHash == st.Agents[d.GateAgentID].InputHash && attempt.SessionID == st.Agents[d.GateAgentID].SessionID && attempt.HeadSHA == d.HeadSHA && !attempt.StartedAt.Before(*r.DecidedAt) {
			return nil
		}
	}
	return errors.New("fresh independent gate must run after approval")
}

func releaseInputCount(d ReleaseDecision) int {
	if d.ApprovalID != "" {
		return len(d.ArtifactRefs) + 5
	}
	return len(d.ArtifactRefs) + 4
}
