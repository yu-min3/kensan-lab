package core

import (
	"encoding/json"
	"errors"
)

// GateOutput is the structured result of the independent Astra turn. A caller
// may propose a ReleaseDecision, but cannot turn a different model result into
// an allow merely by supplying booleans to the API.
type GateOutput struct {
	SchemaVersion        int    `json:"schema_version"`
	Verdict              string `json:"verdict"`
	Reason               string `json:"reason"`
	Operation            string `json:"operation"`
	Repository           string `json:"repository"`
	Ref                  string `json:"ref"`
	HeadSHA              string `json:"head_sha"`
	TargetEnvironment    string `json:"target_environment"`
	PolicyVersion        string `json:"policy_version"`
	ImplementationSHA256 string `json:"implementation_sha256"`
	VerificationSHA256   string `json:"verification_sha256"`
	QualityReviewSHA256  string `json:"quality_review_sha256"`
	ScanSHA256           string `json:"scan_sha256"`
	CandidateSHA256      string `json:"candidate_sha256"`
	SecretFree           bool   `json:"secret_free"`
	PrivateTarget        bool   `json:"private_target"`
	Reversible           bool   `json:"reversible"`
	CIComplete           bool   `json:"ci_complete"`
}

func (s *Store) matchingGateOutput(d ReleaseDecision, manifest ContextManifest, proof releaseProof, candidateRef ArtifactRef) (ArtifactRef, error) {
	st := s.Snapshot()
	gate, ok := st.Agents[d.GateAgentID]
	if !ok || gate.Status != "completed" || gate.Role != "release_gate" || gate.Provider != "codex" || gate.Model != "gpt-6-astra" || gate.InputHash != manifest.InputSHA256 || gate.SessionID == "" {
		return ArtifactRef{}, errors.New("independent release gate turn has not completed")
	}
	var result Attempt
	for _, attempt := range st.Attempts {
		if attempt.AgentID == gate.ID && attempt.Status == "completed" && attempt.OutputRef != nil && attempt.InputHash == manifest.InputSHA256 && attempt.SessionID == gate.SessionID && attempt.HeadSHA == d.HeadSHA {
			if result.ID != "" {
				return ArtifactRef{}, errors.New("multiple gate results for one decision")
			}
			result = attempt
		}
	}
	if result.ID == "" {
		return ArtifactRef{}, errors.New("gate has no completed result for candidate")
	}
	ref := *result.OutputRef
	artifact := st.Artifacts[ref.ID]
	if artifact.AgentID != gate.ID || artifact.TaskID != gate.TaskID || artifact.Kind != "result-release_gate" || artifact.Version != ref.Version || artifact.SHA256 != ref.SHA256 {
		return ArtifactRef{}, errors.New("gate result artifact identity mismatch")
	}
	listed := false
	for _, evidence := range d.EvidenceRefs {
		if evidence == ref {
			listed = true
		}
	}
	if !listed {
		return ArtifactRef{}, errors.New("decision omits the actual gate result")
	}
	body, err := s.ReadArtifact(ref.ID)
	if err != nil {
		return ArtifactRef{}, err
	}
	var output GateOutput
	if json.Unmarshal(body, &output) != nil || output.SchemaVersion != 1 || output.Verdict != d.Verdict || output.Reason != d.Reason || output.Operation != d.Operation || output.Repository != d.Repository || output.Ref != d.Ref || output.HeadSHA != d.HeadSHA || output.TargetEnvironment != d.TargetEnvironment || output.PolicyVersion != d.PolicyVersion || output.ImplementationSHA256 != proof.Implementation.SHA256 || output.VerificationSHA256 != proof.Verification.SHA256 || output.QualityReviewSHA256 != proof.QualityReview.SHA256 || output.ScanSHA256 != d.ScanRef.SHA256 || output.CandidateSHA256 != candidateRef.SHA256 || output.SecretFree != d.SecretFree || output.PrivateTarget != d.PrivateTarget || output.Reversible != d.Reversible || output.CIComplete != d.CIComplete {
		return ArtifactRef{}, errors.New("release decision differs from independent gate output")
	}
	return ref, nil
}

func gateOutputStillCurrent(st *State, gateID string, ref ArtifactRef, inputHash, headSHA string) bool {
	gate, ok := st.Agents[gateID]
	if !ok || gate.Status != "completed" || gate.InputHash != inputHash || gate.SessionID == "" || st.Tasks[gate.TaskID].HeadSHA != headSHA {
		return false
	}
	for _, attempt := range st.Attempts {
		if attempt.AgentID == gateID && attempt.Status == "completed" && attempt.InputHash == inputHash && attempt.SessionID == gate.SessionID && attempt.HeadSHA == headSHA && attempt.OutputRef != nil && *attempt.OutputRef == ref {
			return true
		}
	}
	return false
}
