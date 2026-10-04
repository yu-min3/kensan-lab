package core

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// BindReleaseGateInputs fixes the author artifacts, test evidence, scan, and
// proposed operation before an independent gate session starts.
func (s *Store) BindReleaseGateInputs(gateID, authorID string, sourceRefs []ArtifactRef, scanRef ArtifactRef, candidate ReleaseCandidate) error {
	if len(sourceRefs) == 0 || scanRef.ID == "" {
		return errors.New("release gate needs author artifacts and scan")
	}
	for _, ref := range append(append([]ArtifactRef{}, sourceRefs...), scanRef) {
		if err := s.verifyRef(ref); err != nil {
			return err
		}
	}
	proof, err := s.releaseReady(authorID, sourceRefs)
	if err != nil {
		return err
	}
	b, err := s.ReadArtifact(scanRef.ID)
	if err != nil {
		return err
	}
	var scan ReleaseScan
	if err := json.Unmarshal(b, &scan); err != nil {
		return errors.New("invalid release scan")
	}
	if err := validateReleaseCandidate(candidate, scan); err != nil {
		return err
	}
	if err := s.validateDeploymentCandidate(s.Snapshot().Tasks[s.Snapshot().Agents[authorID].TaskID], candidate, scan); err != nil {
		return err
	}
	candidateBody, err := json.Marshal(candidate)
	if err != nil {
		return err
	}
	candidateArtifact, err := s.PutArtifact("system", "release_candidate", candidateBody)
	if err != nil {
		return err
	}
	candidateRef := artifactRef(candidateArtifact)
	classified := candidateClassification(scan, candidate)
	var approvalRef ArtifactRef
	state := s.Snapshot()
	for id := range state.Approvals {
		proposed := ReleaseDecision{ApprovalID: id, AuthorAgentID: authorID, GateAgentID: gateID, Operation: scan.Operation, Repository: scan.Repository, Ref: scan.Ref, HeadSHA: scan.HeadSHA, TargetEnvironment: candidate.TargetEnvironment, ScanRef: scanRef, HumanCategories: classified.HumanCategories, HumanReasons: classified.HumanReasons}
		if approvalMatches(state, proposed, classified, candidateRef.SHA256) {
			body, err := json.Marshal(state.Approvals[id])
			if err != nil {
				return err
			}
			artifact, err := s.PutArtifact("system", "human_approval", body)
			if err != nil {
				return err
			}
			approvalRef = artifactRef(artifact)
			break
		}
	}
	return s.update(func(st *State) error {
		gate, okG := st.Agents[gateID]
		author, okA := st.Agents[authorID]
		if !okG || !okA || gate.ID == author.ID || gate.Role != "release_gate" || gate.Provider != "codex" || gate.Model != "gpt-6-astra" || gate.SessionID != "" || gate.ReviewAuthorID != "" {
			return errors.New("fresh independent release gate required")
		}
		gt, at := st.Tasks[gate.TaskID], st.Tasks[author.TaskID]
		if author.Team == App && (gate.Team != Platform || gt.Team != Platform) {
			return errors.New("App release must use Platform gate")
		}
		if !scanMatchesAuthorTeam(scan, author, at) {
			return errors.New("release scan ownership does not match author team")
		}
		if gt.MissionID != at.MissionID || gt.ContractVersion != at.ContractVersion || !fullSHA(at.BaseSHA) || !fullSHA(at.HeadSHA) || scan.BaseSHA != at.BaseSHA || scan.HeadSHA != at.HeadSHA || scan.PolicyVersion != ReleasePolicyVersion {
			return errors.New("release gate mission, contract or SHA mismatch")
		}
		if gt.BaseSHA != "" && gt.BaseSHA != at.BaseSHA || gt.HeadSHA != "" && gt.HeadSHA != at.HeadSHA {
			return errors.New("gate task was pinned to another revision")
		}
		if a := st.Artifacts[scanRef.ID]; a.AgentID != "system" || a.Kind != "release_scan" {
			return errors.New("controller-owned scan required")
		}
		if !releaseProofStillCurrent(st, author.TaskID, proof) {
			return errors.New("change readiness changed before gate binding")
		}
		for _, ref := range sourceRefs {
			if a := st.Artifacts[ref.ID]; a.AgentID != authorID || a.TaskID != author.TaskID || a.Version != ref.Version || a.SHA256 != ref.SHA256 {
				return errors.New("review input is not an author artifact")
			}
		}
		gt.BaseSHA, gt.HeadSHA, gt.UpdatedAt = at.BaseSHA, at.HeadSHA, time.Now().UTC()
		st.Tasks[gt.ID] = gt
		gate.ReviewAuthorID = authorID
		gate.ReviewInputs = append(append(append([]ArtifactRef{}, sourceRefs...), proof.Verification, proof.QualityReview), scanRef)
		if approvalRef.ID != "" {
			gate.ReviewInputs = append(gate.ReviewInputs, approvalRef)
		}
		gate.ReviewInputs = append(gate.ReviewInputs, candidateRef)
		gate.UpdatedAt = time.Now().UTC()
		st.Agents[gate.ID] = gate
		st.Events = append(st.Events, event("release_gate_inputs_bound", gateID, authorID))
		return nil
	})
}

func validateReleaseCandidate(candidate ReleaseCandidate, scan ReleaseScan) error {
	if candidate.SchemaVersion != 1 || candidate.Operation != scan.Operation || candidate.Repository != scan.Repository || candidate.Ref != scan.Ref || candidate.HeadSHA != scan.HeadSHA || candidate.TargetEnvironment != "github" && candidate.TargetEnvironment != "private-sense" && candidate.TargetEnvironment != "private-canary" || strings.TrimSpace(candidate.Impact) == "" || strings.TrimSpace(candidate.Rollback) == "" || len(candidate.Impact) > 4000 || len(candidate.Rollback) > 4000 || len(candidate.PullRequestSummary) > 4000 {
		return errors.New("release candidate does not match the scan or lacks impact and rollback")
	}
	return validateCandidateImage(candidate, scan)
}

func (s *Store) releaseCandidate(gate Agent, scan ReleaseScan) (ReleaseCandidate, ArtifactRef, error) {
	if len(gate.ReviewInputs) < 2 {
		return ReleaseCandidate{}, ArtifactRef{}, errors.New("gate candidate input missing")
	}
	ref := gate.ReviewInputs[len(gate.ReviewInputs)-1]
	artifact := s.Snapshot().Artifacts[ref.ID]
	if artifact.AgentID != "system" || artifact.Kind != "release_candidate" || artifact.Version != ref.Version || artifact.SHA256 != ref.SHA256 {
		return ReleaseCandidate{}, ArtifactRef{}, errors.New("gate candidate is not controller-owned")
	}
	body, err := s.ReadArtifact(ref.ID)
	if err != nil {
		return ReleaseCandidate{}, ArtifactRef{}, err
	}
	var candidate ReleaseCandidate
	if err := json.Unmarshal(body, &candidate); err != nil || validateReleaseCandidate(candidate, scan) != nil {
		return ReleaseCandidate{}, ArtifactRef{}, errors.New("invalid fixed gate candidate")
	}
	if err := s.validateDeploymentCandidate(s.Snapshot().Tasks[s.Snapshot().Agents[gate.ReviewAuthorID].TaskID], candidate, scan); err != nil {
		return ReleaseCandidate{}, ArtifactRef{}, err
	}
	return candidate, ref, nil
}

const ReleasePolicyVersion = "private-v4-image-release"

var allowedOperations = map[string]bool{
	"branch_push":   true,
	"pr_create":     true,
	"pr_update":     true,
	"merge":         true,
	"deploy":        true,
	"rollback":      true,
	"image_publish": true,
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
	var scan ReleaseScan
	var gateManifest ContextManifest
	var proof releaseProof
	var candidateRef ArtifactRef
	var gateOutputRef ArtifactRef
	if d.ScanRef.ID != "" {
		loaded, err := s.loadReleaseScan(d.ScanRef, d)
		if err != nil {
			return ReleaseDecision{}, err
		}
		candidate, _, err := s.releaseCandidate(s.Snapshot().Agents[d.GateAgentID], loaded)
		if err != nil {
			return ReleaseDecision{}, err
		}
		if d.ImageRelease != nil && !imageReleasesEqual(d.ImageRelease, candidate.ImageRelease) {
			return ReleaseDecision{}, errors.New("decision image spec differs from fixed gate candidate")
		}
		if d.ImageDeployment != nil && !imageDeploymentsEqual(d.ImageDeployment, candidate.ImageDeployment) {
			return ReleaseDecision{}, errors.New("decision deployment image differs from fixed candidate")
		}
		d.ImageDeployment = cloneImageDeployment(candidate.ImageDeployment)
		d.ImageRelease = cloneImageRelease(candidate.ImageRelease)
		loaded = candidateClassification(loaded, candidate)
		d.HumanCategories, d.HumanReasons = loaded.HumanCategories, loaded.HumanReasons
		if loaded.Status == "deny" {
			d.Verdict = "deny"
		}
		if d.Verdict == "allow" && len(d.HumanCategories) > 0 && d.ApprovalID == "" {
			return ReleaseDecision{}, errors.New("classified release needs human approval and fresh gate")
		}
		if d.Verdict != "deny" && len(d.HumanCategories) > 0 && d.ApprovalID == "" {
			d.Verdict = "needs_human"
		}
	} else {
		if d.Operation == "image_publish" || d.ImageRelease != nil || d.ImageDeployment != nil {
			return ReleaseDecision{}, errors.New("image publish requires fixed controller scan and candidate")
		}
		d.HumanCategories, d.HumanReasons = nil, nil
	}
	if d.Verdict == "allow" {
		if d.ScanRef.ID == "" {
			return ReleaseDecision{}, errors.New("allow verdict requires controller release scan")
		}
		var err error
		scan, err = s.checkReleaseScan(d.ScanRef, d)
		if err != nil {
			return ReleaseDecision{}, err
		}
		proof, err = s.releaseReady(d.AuthorAgentID, d.ArtifactRefs)
		if err != nil {
			return ReleaseDecision{}, err
		}
		candidate, ref, err := s.releaseCandidate(s.Snapshot().Agents[d.GateAgentID], scan)
		if err != nil || candidate.Operation != d.Operation || candidate.Repository != d.Repository || candidate.Ref != d.Ref || candidate.HeadSHA != d.HeadSHA || candidate.TargetEnvironment != d.TargetEnvironment {
			return ReleaseDecision{}, errors.New("gate candidate differs from requested decision")
		}
		candidateRef = ref
		if err := s.boundApprovalMatches(d, scan, candidateRef); err != nil {
			return ReleaseDecision{}, err
		}
		gateManifest, err = s.BuildManifest(d.GateAgentID, []string{"isolated-model-worker"})
		if err != nil || gateManifest.ReviewAuthorID != d.AuthorAgentID || len(gateManifest.ReviewInputs) != releaseInputCount(d) || !refsEqual(gateManifest.ReviewInputs[:len(d.ArtifactRefs)], d.ArtifactRefs) || gateManifest.ReviewInputs[len(d.ArtifactRefs)] != proof.Verification || gateManifest.ReviewInputs[len(d.ArtifactRefs)+1] != proof.QualityReview || gateManifest.ReviewInputs[len(d.ArtifactRefs)+2] != d.ScanRef || gateManifest.ReviewInputs[len(gateManifest.ReviewInputs)-1] != candidateRef {
			return ReleaseDecision{}, errors.New("release gate manifest lacks exact author and scan inputs")
		}
		gateOutputRef, err = s.matchingGateOutput(d, gateManifest, proof, candidateRef)
		if err != nil {
			return ReleaseDecision{}, err
		}
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
		if !okA || !okG || author.ID == gate.ID || gate.Role != "release_gate" || gate.Provider != "codex" || gate.Model != "gpt-6-astra" || gate.SessionID == "" || author.SessionID == gate.SessionID {
			return errors.New("independent release gate session required")
		}
		if author.Team == App && (gate.Team != Platform || st.Tasks[gate.TaskID].Team != Platform) {
			return errors.New("App release must use Platform gate")
		}
		if d.Verdict == "allow" && d.ApprovalID != "" && !approvalMatches(*st, d, scan, candidateRef.SHA256) {
			return errors.New("human approval changed before gate decision")
		}
		if d.Verdict == "allow" {
			for _, stage := range st.Agents {
				if stage.TaskID == author.TaskID && stage.SessionID != "" && stage.SessionID == gate.SessionID {
					return errors.New("release gate reused a stage session")
				}
			}
		}
		if st.Tasks[author.TaskID].MissionID != st.Tasks[gate.TaskID].MissionID || st.Tasks[author.TaskID].ContractVersion != st.Tasks[gate.TaskID].ContractVersion {
			return errors.New("release gate mission or contract mismatch")
		}
		if d.Verdict == "allow" && (strings.HasPrefix(author.SessionID, "mock-") || strings.HasPrefix(gate.SessionID, "mock-")) {
			return errors.New("simulation sessions cannot authorize release")
		}
		if d.Verdict == "allow" && (author.SessionID == "" || gate.InputHash != gateManifest.InputSHA256 || gate.ReviewAuthorID != author.ID || st.Tasks[gate.TaskID].BaseSHA != scan.BaseSHA || st.Tasks[gate.TaskID].HeadSHA != scan.HeadSHA) {
			return errors.New("release gate did not review pinned inputs in an independent session")
		}
		if d.Verdict == "allow" && !scanMatchesAuthorTeam(scan, author, st.Tasks[author.TaskID]) {
			return errors.New("release scan ownership changed before gate decision")
		}
		if d.Verdict == "allow" && !releaseProofStillCurrent(st, author.TaskID, proof) {
			return errors.New("verification or quality review changed before gate decision")
		}
		if d.Verdict == "allow" && !gateOutputStillCurrent(st, gate.ID, gateOutputRef, gateManifest.InputSHA256, d.HeadSHA) {
			return errors.New("gate result changed before decision")
		}
		if st.Tasks[author.TaskID].HeadSHA != d.HeadSHA {
			return errors.New("decision head differs from current task")
		}
		if d.Verdict == "allow" && (st.Tasks[author.TaskID].BaseSHA == "" || st.Tasks[author.TaskID].BaseSHA != scan.BaseSHA) {
			return errors.New("decision scan does not cover task base")
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
	d.ImageRelease = cloneImageRelease(d.ImageRelease)
	d.ImageDeployment = cloneImageDeployment(d.ImageDeployment)
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
	var proof releaseProof
	var gateManifest ContextManifest
	var gateOutputRef ArtifactRef
	var pullRequestSummary string
	var scan ReleaseScan
	if d.Verdict == "allow" {
		var err error
		scan, err = s.checkReleaseScan(d.ScanRef, d)
		if err != nil || st.Tasks[st.Agents[d.AuthorAgentID].TaskID].BaseSHA != scan.BaseSHA {
			return PublishIntent{}, errors.New("release scan no longer matches decision")
		}
		candidate, candidateRef, err := s.releaseCandidate(st.Agents[d.GateAgentID], scan)
		if err != nil || candidate.Operation != d.Operation || candidate.Repository != d.Repository || candidate.Ref != d.Ref || candidate.HeadSHA != d.HeadSHA || candidate.TargetEnvironment != d.TargetEnvironment || !imageReleasesEqual(d.ImageRelease, candidate.ImageRelease) || !imageDeploymentsEqual(d.ImageDeployment, candidate.ImageDeployment) {
			return PublishIntent{}, errors.New("release candidate no longer matches decision")
		}
		pullRequestSummary = candidate.PullRequestSummary
		if err := s.boundApprovalMatches(d, scan, candidateRef); err != nil {
			return PublishIntent{}, err
		}
		proof, err = s.releaseReady(d.AuthorAgentID, d.ArtifactRefs)
		if err != nil {
			return PublishIntent{}, err
		}
		gateManifest, err = s.BuildManifest(d.GateAgentID, []string{"isolated-model-worker"})
		if err != nil || gateManifest.ReviewAuthorID != d.AuthorAgentID || len(gateManifest.ReviewInputs) != releaseInputCount(d) || gateManifest.ReviewInputs[len(gateManifest.ReviewInputs)-1] != candidateRef {
			return PublishIntent{}, errors.New("gate input manifest changed after decision")
		}
		gateOutputRef, err = s.matchingGateOutput(d, gateManifest, proof, candidateRef)
		if err != nil {
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
		author := st.Agents[d.AuthorAgentID]
		gate := st.Agents[d.GateAgentID]
		if author.Team == App && (gate.Team != Platform || st.Tasks[gate.TaskID].Team != Platform) {
			return errors.New("App release must use Platform gate")
		}
		if d.ApprovalID != "" && !approvalMatches(*st, d, scan, "") {
			return errors.New("human approval expired or changed before publish intent")
		}
		if !scanMatchesAuthorTeam(scan, author, st.Tasks[author.TaskID]) {
			return errors.New("release scan ownership changed before publish intent")
		}
		if !releaseProofStillCurrent(st, st.Agents[d.AuthorAgentID].TaskID, proof) {
			return errors.New("release readiness changed before publish intent")
		}
		if !gateOutputStillCurrent(st, d.GateAgentID, gateOutputRef, gateManifest.InputSHA256, d.HeadSHA) {
			return errors.New("gate result changed before publish intent")
		}
		for _, old := range st.Intents {
			if old.DecisionID == decisionID {
				if !imageReleasesEqual(old.ImageRelease, d.ImageRelease) || !imageDeploymentsEqual(old.ImageDeployment, d.ImageDeployment) {
					return errors.New("image intent differs from fixed release decision")
				}
				intent = old
				return nil
			}
		}
		deadline := d.ExpiresAt
		if d.ApprovalID != "" && st.Approvals[d.ApprovalID].ExpiresAt.Before(deadline) {
			deadline = st.Approvals[d.ApprovalID].ExpiresAt
		}
		intent = PublishIntent{ID: id, DecisionID: decisionID, Operation: operation, Repository: repository, Ref: ref, HeadSHA: sha, BaseSHA: scan.BaseSHA, TargetEnvironment: d.TargetEnvironment, PolicyVersion: d.PolicyVersion, ExpiresAt: deadline, ImageRelease: cloneImageRelease(d.ImageRelease), ImageDeployment: cloneImageDeployment(d.ImageDeployment), PullRequestSummary: pullRequestSummary, Status: "pending_reconcile", CreatedAt: time.Now().UTC()}
		st.Intents[id] = intent
		st.Events = append(st.Events, event("publish_intent", id, operation))
		return nil
	})
	intent.ImageRelease = cloneImageRelease(intent.ImageRelease)
	intent.ImageDeployment = cloneImageDeployment(intent.ImageDeployment)
	return intent, err
}
