package core

import (
	"encoding/json"
	"errors"
)

// releaseProof binds a release to the actual implementation, credentialless
// checks, and independent quality review of one committed task head.
type releaseProof struct {
	Implementation ArtifactRef
	Verification   ArtifactRef
	QualityReview  ArtifactRef
}

func (s *Store) releaseReady(authorID string, sourceRefs []ArtifactRef) (releaseProof, error) {
	st := s.Snapshot()
	author, ok := st.Agents[authorID]
	if !ok || author.Role != "implementation" || author.Provider != "codex" || author.Model != "gpt-6-sol" || author.SessionID == "" || len(author.SessionID) >= 5 && author.SessionID[:5] == "mock-" {
		return releaseProof{}, errors.New("release author is not an actual Sol implementation")
	}
	task, ok := st.Tasks[author.TaskID]
	if !ok || task.Kind != "change" || task.Status != "publish_wait" || !fullSHA(task.BaseSHA) || !fullSHA(task.HeadSHA) || task.BaseSHA == task.HeadSHA {
		return releaseProof{}, errors.New("change task has not completed its release-ready stages")
	}
	want := map[string]struct{ provider, model string }{
		"requirements":          {"claude", "fable"},
		"design_review":         {"codex", "gpt-6-astra"},
		"implementation":        {"codex", "gpt-6-sol"},
		"verification":          {"system", "local"},
		"implementation_review": {"claude", "opus"},
	}
	roles := map[string]Agent{}
	sessions := map[string]bool{}
	for _, agent := range st.Agents {
		if agent.TaskID != task.ID {
			continue
		}
		expected, allowed := want[agent.Role]
		_, duplicate := roles[agent.Role]
		if !allowed || duplicate || agent.Provider != expected.provider || agent.Model != expected.model || agent.Status != "completed" {
			return releaseProof{}, errors.New("release task stages are incomplete or unexpected")
		}
		if agent.Role != "verification" {
			if agent.SessionID == "" || len(agent.SessionID) >= 5 && agent.SessionID[:5] == "mock-" || sessions[agent.SessionID] {
				return releaseProof{}, errors.New("release task lacks independent real sessions")
			}
			sessions[agent.SessionID] = true
		} else if agent.SessionID != "" {
			return releaseProof{}, errors.New("credentialless verifier acquired a model session")
		}
		roles[agent.Role] = agent
	}
	if len(roles) != len(want) || roles["implementation"].ID != authorID {
		return releaseProof{}, errors.New("release task is missing a required stage")
	}
	result := map[string]ArtifactRef{}
	resultAttempt := map[string]Attempt{}
	for role, agent := range roles {
		found := false
		for _, attempt := range st.Attempts {
			if attempt.AgentID != agent.ID || attempt.Status != "completed" || attempt.OutputRef == nil || attempt.BaseSHA != task.BaseSHA || attempt.ContractVersion != task.ContractVersion {
				continue
			}
			if role != "requirements" && role != "design_review" && attempt.HeadSHA != task.HeadSHA {
				continue
			}
			if found {
				return releaseProof{}, errors.New("multiple completed results for one release stage")
			}
			artifact := st.Artifacts[attempt.OutputRef.ID]
			if artifact.TaskID != task.ID || artifact.AgentID != agent.ID || artifact.Kind != "result-"+role || artifact.Version != attempt.OutputRef.Version || artifact.SHA256 != attempt.OutputRef.SHA256 {
				return releaseProof{}, errors.New("release stage artifact identity mismatch")
			}
			if err := s.verifyRef(*attempt.OutputRef); err != nil {
				return releaseProof{}, err
			}
			result[role] = *attempt.OutputRef
			resultAttempt[role] = attempt
			found = true
		}
		if !found {
			return releaseProof{}, errors.New("completed release stage lacks a pinned result")
		}
	}
	reviewManifest, err := s.BuildManifest(roles["implementation_review"].ID, []string{"isolated-model-worker"})
	if err != nil || roles["implementation_review"].InputHash != reviewManifest.InputSHA256 || resultAttempt["implementation_review"].InputHash != reviewManifest.InputSHA256 {
		return releaseProof{}, errors.New("Opus did not review the current fixed input manifest")
	}
	implementation := result["implementation"]
	listed := false
	for _, ref := range sourceRefs {
		if ref == implementation {
			listed = true
		}
	}
	if !listed {
		return releaseProof{}, errors.New("gate inputs omit the implementation artifact")
	}
	implBody, err := s.ReadArtifact(implementation.ID)
	if err != nil {
		return releaseProof{}, err
	}
	var impl struct {
		SchemaVersion int `json:"schema_version"`
		Change        struct {
			BaseSHA    string `json:"base_sha"`
			HeadSHA    string `json:"head_sha"`
			Clean      bool   `json:"clean"`
			DiffSHA256 string `json:"diff_sha256"`
			Patch      string `json:"patch"`
		} `json:"change"`
	}
	if json.Unmarshal(implBody, &impl) != nil || impl.SchemaVersion != 1 || impl.Change.BaseSHA != task.BaseSHA || impl.Change.HeadSHA != task.HeadSHA || !impl.Change.Clean || impl.Change.Patch == "" || impl.Change.DiffSHA256 != digest([]byte(impl.Change.Patch)) {
		return releaseProof{}, errors.New("implementation artifact does not match committed diff")
	}
	verification := result["verification"]
	verifyBody, err := s.ReadArtifact(verification.ID)
	if err != nil {
		return releaseProof{}, err
	}
	var checks struct {
		SchemaVersion int    `json:"schema_version"`
		BaseSHA       string `json:"base_sha"`
		HeadSHA       string `json:"head_sha"`
		DiffSHA256    string `json:"diff_sha256"`
		PlanSHA256    string `json:"plan_sha256"`
		Status        string `json:"status"`
		Checks        []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if json.Unmarshal(verifyBody, &checks) != nil || checks.SchemaVersion != 1 || checks.BaseSHA != task.BaseSHA || checks.HeadSHA != task.HeadSHA || checks.DiffSHA256 != impl.Change.DiffSHA256 || !fullDigest(checks.PlanSHA256) || checks.Status != "passed" || len(checks.Checks) < 2 || checks.Checks[0].Name != "git-diff-check" {
		return releaseProof{}, errors.New("credentialless verification did not pass for this change")
	}
	for _, check := range checks.Checks {
		if check.Name == "" || check.Status != "passed" {
			return releaseProof{}, errors.New("verification contains a failed or unnamed check")
		}
	}
	quality := result["implementation_review"]
	reviewBody, err := s.ReadArtifact(quality.ID)
	if err != nil {
		return releaseProof{}, err
	}
	var review struct {
		SchemaVersion        int    `json:"schema_version"`
		Verdict              string `json:"verdict"`
		HeadSHA              string `json:"head_sha"`
		ImplementationSHA256 string `json:"implementation_sha256"`
		VerificationSHA256   string `json:"verification_sha256"`
		Reason               string `json:"reason"`
	}
	if json.Unmarshal(reviewBody, &review) != nil || review.SchemaVersion != 1 || review.Verdict != "pass" || review.HeadSHA != task.HeadSHA || review.ImplementationSHA256 != implementation.SHA256 || review.VerificationSHA256 != verification.SHA256 || review.Reason == "" {
		return releaseProof{}, errors.New("independent Opus review did not pass the pinned evidence")
	}
	return releaseProof{Implementation: implementation, Verification: verification, QualityReview: quality}, nil
}

func fullDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if c < '0' || c > '9' && c < 'a' || c > 'f' {
			return false
		}
	}
	return true
}

// This cheap locked-state recheck closes the gap between artifact validation
// and the atomic update that records gate inputs, decisions, or intents.
func releaseProofStillCurrent(st *State, taskID string, proof releaseProof) bool {
	if st.Tasks[taskID].Status != "publish_wait" || proof.Implementation.ID == "" || proof.Verification.ID == "" || proof.QualityReview.ID == "" {
		return false
	}
	expected := map[string]ArtifactRef{
		"implementation":        proof.Implementation,
		"verification":          proof.Verification,
		"implementation_review": proof.QualityReview,
	}
	for role, ref := range expected {
		found := false
		for _, agent := range st.Agents {
			if agent.TaskID != taskID || agent.Role != role || agent.Status != "completed" {
				continue
			}
			for _, attempt := range st.Attempts {
				if attempt.AgentID == agent.ID && attempt.Status == "completed" && attempt.HeadSHA == st.Tasks[taskID].HeadSHA && attempt.OutputRef != nil && *attempt.OutputRef == ref {
					found = true
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}
