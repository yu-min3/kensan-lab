package core

import (
	"errors"
	"strings"
	"time"
)

const ReleasePolicyVersion = "private-v1"

var allowedOperations = map[string]bool{
	"branch_push": true,
	"pr_create":   true,
	"pr_update":   true,
	"merge":       true,
	"deploy":      true,
	"rollback":    true,
}

func (s *Store) RecordReleaseDecision(d ReleaseDecision) (ReleaseDecision, error) {
	if d.ID == "" {
		id, err := newID()
		if err != nil {
			return ReleaseDecision{}, err
		}
		d.ID = id
	}
	if d.Verdict != "allow" && d.Verdict != "deny" && d.Verdict != "needs_human" {
		return ReleaseDecision{}, errors.New("invalid gate verdict")
	}
	if d.Reason == "" || d.Operation == "" || d.Repository == "" || d.Ref == "" || d.HeadSHA == "" || d.TargetEnvironment == "" || d.PolicyVersion != ReleasePolicyVersion || len(d.ArtifactRefs) == 0 || len(d.EvidenceRefs) == 0 {
		return ReleaseDecision{}, errors.New("incomplete release decision")
	}
	if !allowedOperations[d.Operation] || d.Repository != "yu-min3/kensan-lab" || !strings.HasPrefix(d.Ref, "refs/heads/") || strings.HasPrefix(d.Ref, "refs/heads/main") && d.Operation == "branch_push" {
		return ReleaseDecision{}, errors.New("operation, repository or ref outside delegated scope")
	}
	if d.TargetEnvironment != "github" && d.TargetEnvironment != "private-sense" && d.TargetEnvironment != "private-canary" {
		return ReleaseDecision{}, errors.New("unknown target environment")
	}
	if len(d.HeadSHA) != 40 && len(d.HeadSHA) != 64 {
		return ReleaseDecision{}, errors.New("full head SHA required")
	}
	for _, r := range d.HeadSHA {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return ReleaseDecision{}, errors.New("invalid head SHA")
		}
	}
	if d.Verdict == "allow" && (!d.SecretFree || !d.PrivateTarget || !d.Reversible || (d.Operation == "merge" || d.Operation == "deploy") && !d.CIComplete) {
		return ReleaseDecision{}, errors.New("allow verdict lacks required checks")
	}
	if d.ExpiresAt.IsZero() || d.ExpiresAt.After(time.Now().Add(24*time.Hour)) || !d.ExpiresAt.After(time.Now()) {
		return ReleaseDecision{}, errors.New("decision must expire within 24 hours")
	}
	for _, ref := range append(append([]ArtifactRef{}, d.ArtifactRefs...), d.EvidenceRefs...) {
		if err := s.verifyRef(ref); err != nil {
			return ReleaseDecision{}, err
		}
	}
	err := s.update(func(st *State) error {
		if _, exists := st.Decisions[d.ID]; exists {
			return errors.New("release decision ID already exists")
		}
		author, okA := st.Agents[d.AuthorAgentID]
		gate, okG := st.Agents[d.GateAgentID]
		if !okA || !okG || author.ID == gate.ID || gate.Role != "release_gate" || gate.SessionID == "" || author.SessionID == gate.SessionID {
			return errors.New("independent release gate session required")
		}
		if st.Tasks[author.TaskID].HeadSHA != d.HeadSHA {
			return errors.New("decision head differs from current task")
		}
		for _, ref := range d.ArtifactRefs {
			a, ok := st.Artifacts[ref.ID]
			if !ok || a.AgentID != author.ID || a.Version != ref.Version || a.SHA256 != ref.SHA256 {
				return errors.New("source artifact mismatch")
			}
		}
		for _, ref := range d.EvidenceRefs {
			a, ok := st.Artifacts[ref.ID]
			if !ok || a.AgentID != gate.ID || a.Version != ref.Version || a.SHA256 != ref.SHA256 {
				return errors.New("independent evidence artifact required")
			}
		}
		d.CreatedAt = time.Now().UTC()
		st.Decisions[d.ID] = d
		st.Events = append(st.Events, event("release_decided", d.ID, d.Verdict))
		return nil
	})
	return d, err
}

// PreparePublish is the only authorization path to the future publisher. It
// persists an intent before an external side effect. A resumed service must
// reconcile pending intent with GitHub/Argo before executing anything.
func (s *Store) PreparePublish(decisionID, operation, repository, ref, sha string) (PublishIntent, error) {
	st := s.Snapshot()
	d, ok := st.Decisions[decisionID]
	if !ok {
		return PublishIntent{}, errors.New("decision not found")
	}
	for _, artifact := range append(append([]ArtifactRef{}, d.ArtifactRefs...), d.EvidenceRefs...) {
		if err := s.verifyRef(artifact); err != nil {
			return PublishIntent{}, err
		}
	}
	id, err := newID()
	if err != nil {
		return PublishIntent{}, err
	}
	var intent PublishIntent
	err = s.update(func(st *State) error {
		d, ok := st.Decisions[decisionID]
		if !ok || d.Verdict != "allow" || !time.Now().Before(d.ExpiresAt) {
			return errors.New("no live allow decision")
		}
		if d.Operation != operation || d.Repository != repository || d.Ref != ref || d.HeadSHA != sha || d.PolicyVersion != ReleasePolicyVersion {
			return errors.New("decision does not match operation")
		}
		if st.Tasks[st.Agents[d.AuthorAgentID].TaskID].HeadSHA != sha {
			return errors.New("task head changed after decision")
		}
		for _, old := range st.Intents {
			if old.DecisionID == decisionID {
				intent = old
				return nil
			}
		}
		intent = PublishIntent{ID: id, DecisionID: decisionID, Operation: operation, Repository: repository, Ref: ref, HeadSHA: sha, Status: "pending_reconcile", CreatedAt: time.Now().UTC()}
		st.Intents[id] = intent
		st.Events = append(st.Events, event("publish_intent", id, operation))
		return nil
	})
	return intent, err
}
