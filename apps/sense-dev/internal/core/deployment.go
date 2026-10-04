package core

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// RecordDeploymentReceipt is a trusted host/operator entry point. Worker outputs
// cannot create this system-owned observation of the actual deployment.
func (s *Store) RecordDeploymentReceipt(r DeploymentReceipt) error {
	if !validReleaseMarker(r.ObservedRelease) || !fullSHA(r.HeadSHA) || !fullSHA(r.Revision) || r.ImageSourceSHA != r.Revision || r.Environment != "private-canary" || r.Status != "healthy" || strings.TrimSpace(r.UserPath) == "" || !strings.HasPrefix(r.ImageDigest, "sha256:") || len(r.ImageDigest) != 71 {
		return errors.New("deployment needs pinned revision, digest, private environment and healthy user path")
	}
	for _, ch := range r.ImageDigest[7:] {
		if !strings.ContainsRune("0123456789abcdef", ch) {
			return errors.New("invalid image digest")
		}
	}
	if err := s.verifyRef(r.EvidenceRef); err != nil {
		return err
	}
	artifact := s.Snapshot().Artifacts[r.EvidenceRef.ID]
	if artifact.AgentID != "system" || artifact.Kind != "deployment_observation" {
		return errors.New("host-owned deployment observation required")
	}
	body, err := s.ReadArtifact(r.EvidenceRef.ID)
	if err != nil {
		return err
	}
	var observation DeploymentReceipt
	if json.Unmarshal(body, &observation) != nil || observation.TaskID != r.TaskID || observation.DecisionID != r.DecisionID || observation.IntentID != r.IntentID || observation.HeadSHA != r.HeadSHA || observation.Revision != r.Revision || observation.ObservedRelease != r.ObservedRelease || observation.ImageSourceSHA != r.ImageSourceSHA || observation.ImageDigest != r.ImageDigest || observation.Environment != r.Environment || observation.Status != r.Status || observation.UserPath != r.UserPath {
		return errors.New("deployment observation does not match receipt")
	}
	return s.update(func(st *State) error {
		task, ok := st.Tasks[r.TaskID]
		d, okD := st.Decisions[r.DecisionID]
		intent, okI := st.Intents[r.IntentID]
		if !ok || task.Kind != "change" || task.HeadSHA != r.HeadSHA || task.Status != "publish_wait" || !okD || !okI || d.Verdict != "allow" || d.TargetEnvironment != r.Environment || d.HeadSHA != r.HeadSHA || d.Operation != "deploy" && d.Operation != "merge" || !d.ExpiresAt.After(time.Now()) || d.PolicyVersion != ReleasePolicyVersion || st.Agents[d.AuthorAgentID].TaskID != task.ID || intent.DecisionID != d.ID || intent.Status != "sent" || intent.HeadSHA != r.HeadSHA || intent.Operation != d.Operation || intent.ExternalID != r.Revision {
			return errors.New("deployment is not a sent, approved current task operation")
		}
		if old, exists := st.Deployments[r.TaskID]; exists && old.HeadSHA == r.HeadSHA {
			if old.DecisionID == r.DecisionID && old.ImageDigest == r.ImageDigest && old.EvidenceRef == r.EvidenceRef {
				return nil
			}
			return errors.New("conflicting deployment receipt")
		}
		r.RecordedAt = time.Now().UTC()
		st.Deployments[r.TaskID] = r
		st.Events = append(st.Events, event("deployment_verified", r.TaskID, r.HeadSHA))
		return nil
	})
}

func appDeploymentReady(st State, task Task) bool {
	r, ok := st.Deployments[task.ID]
	d, okD := st.Decisions[r.DecisionID]
	intent, okI := st.Intents[r.IntentID]
	return ok && okD && okI && validReleaseMarker(r.ObservedRelease) && r.HeadSHA == task.HeadSHA && fullSHA(r.Revision) && r.ImageSourceSHA == r.Revision && intent.ExternalID == r.Revision && r.Status == "healthy" && r.Environment == "private-canary" && r.RecordedAt.Before(d.ExpiresAt) && d.Verdict == "allow" && d.HeadSHA == task.HeadSHA && d.PolicyVersion == ReleasePolicyVersion && d.TargetEnvironment == r.Environment && (d.Operation == "deploy" || d.Operation == "merge") && st.Agents[d.AuthorAgentID].TaskID == task.ID && intent.DecisionID == d.ID && intent.Status == "sent" && intent.HeadSHA == task.HeadSHA && intent.Operation == d.Operation
}

func validReleaseMarker(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}
