package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

func (s *Store) SeedKnowledge() error {
	initial := map[string]string{
		"profile-platform":   "Platform team: own the safe deployment path (Release Gate, publisher, GitOps, and rollback), Golden Path contracts, templates, and operability. Judge versioned App feedback with explicit decisions and reasons. Security and exposure changes require Yu approval. Private conversations are not shared inputs.",
		"knowledge-platform": "Current mission: provide the canary Golden Path and safe deployment path for App development. Improve the platform from App feedback and return versioned decisions. Adopted changes use independent stages and Gate; the originating App rechecks the same operation after deployment. Keep approved scope and contract fixed.",
		"profile-app":        "App team: lead application development within approved owned paths. Run requirements, design review, implementation, credentialless verification, and implementation review in independent sessions. Request deployment through Platform Release Gate and publisher; verify the deployed user path. Send platform feedback with expected versus observed evidence.",
		"knowledge-app":      "Current mission: add a small feature to the canary application, deploy through the Platform path, and verify the user operation. App does not hold publisher credentials. Report deployment or operational needs as versioned platform feedback; continue independent work while Platform decides. Recheck adopted improvements after deployment.",
		"knowledge-common":   "Shared: user-approved mission and scope, accepted contract, repository rules. Private team memories and author chats are never implicit shared inputs.",
	}
	// Upgrade only our original defaults. Operator-maintained knowledge remains
	// authoritative, and existing artifact references remain immutable.
	legacy := map[string]string{
		"profile-platform":   "Platform team: own the Golden Path contract, ADRs, templates, operability, and rollback. Submit versioned contracts and evidence. Consumer constraints from App are input, not hidden shared conversation.",
		"knowledge-platform": "Current mission: complete the production Golden Path, then align existing apps. Scope and accepted contract are versioned; do not expand either from a worker memo.",
		"profile-app":        "App team: represent each consumer's UX, identity, data, and acceptance scenarios from design time. Send expected versus observed failures with evidence.",
		"knowledge-app":      "Current mission: test the Golden Path as a consumer. Preserve web-stateless, web-stateful, and batch differences. Feedback returns to Platform as a versioned artifact.",
	}
	st := s.Snapshot()
	for kind, body := range initial {
		if old, ok := latest(st, "system", kind); ok {
			content, err := s.ReadArtifact(old.ID)
			if err != nil {
				return err
			}
			if legacy[kind] == "" || string(content) != legacy[kind] {
				continue
			}
		}
		if _, err := s.PutArtifact("system", kind, []byte(body)); err != nil {
			return err
		}
	}
	return nil
}

func latest(st State, agentID, kind string) (Artifact, bool) {
	var found Artifact
	ok := false
	for _, a := range st.Artifacts {
		if a.AgentID == agentID && a.Kind == kind && (!ok || a.Version > found.Version) {
			found, ok = a, true
		}
	}
	return found, ok
}

func (s *Store) BuildManifest(agentID string, allowedScope []string) (ContextManifest, error) {
	st := s.Snapshot()
	a, ok := st.Agents[agentID]
	if !ok {
		return ContextManifest{}, errors.New("agent not found")
	}
	t := st.Tasks[a.TaskID]
	profile, hasProfile := latest(st, "system", "profile-"+string(a.Team))
	knowledge, hasKnowledge := latest(st, "system", "knowledge-"+string(a.Team))
	common, hasCommon := latest(st, "system", "knowledge-common")
	if !hasProfile || !hasKnowledge || !hasCommon {
		return ContextManifest{}, errors.New("team knowledge is not seeded")
	}
	m := ContextManifest{SchemaVersion: SchemaVersion, MissionID: t.MissionID, TaskID: t.ID, SourceTaskID: t.SourceTaskID, CheckoutSourceTaskID: t.CheckoutSourceTaskID, Team: a.Team, Role: a.Role, AgentID: a.ID, Provider: a.Provider, Model: a.Model, Generation: a.SessionGeneration, TeamProfile: artifactRef(profile), TeamKnowledge: artifactRef(knowledge), CommonKnowledge: artifactRef(common), Inbox: []ArtifactRef{}, StageInputs: []StageInput{}, ReviewInputs: append([]ArtifactRef{}, a.ReviewInputs...), ReviewAuthorID: a.ReviewAuthorID, MessageIDs: []string{}, ContractVersion: t.ContractVersion, BaseSHA: t.BaseSHA, HeadSHA: t.HeadSHA, AllowedScope: append([]string(nil), allowedScope...)}
	if t.ImageSourceTaskID != "" {
		r, ok := st.ImageReleases[t.ImageSourceTaskID]
		source := st.Tasks[t.ImageSourceTaskID]
		if !ok || t.Team != App || t.Kind != "change" || t.SourceTaskID != "" || t.CheckoutSourceTaskID != "" || r.DeploymentTaskID != t.ID || source.HeadSHA != r.Image.SourceSHA || source.BaseSHA != t.BaseSHA || source.MissionID != t.MissionID || source.ContractVersion != t.ContractVersion || ValidateImageDeploymentSpec(r.Image) != nil {
			return ContextManifest{}, errors.New("image deployment lacks current host image evidence")
		}
		if err := s.verifyRef(r.EvidenceRef); err != nil {
			return ContextManifest{}, err
		}
		artifact := st.Artifacts[r.EvidenceRef.ID]
		if artifact.AgentID != "system" || artifact.Kind != "image_release_observation" {
			return ContextManifest{}, errors.New("image evidence is not host owned")
		}
		ref := r.EvidenceRef
		m.ImageEvidence = &ref
		m.ImageSourceTaskID = source.ID
		m.ImageSourceSHA = r.Image.SourceSHA
	}
	if t.CheckoutSourceTaskID != "" {
		source, ok := st.Tasks[t.CheckoutSourceTaskID]
		if !ok || t.Team != Platform || t.Kind != "change" || t.SourceTaskID != "" || source.Team != App || source.Kind != "change" || !appDeploymentReady(st, source) || t.BaseSHA != st.Deployments[source.ID].Revision || t.MissionID != source.MissionID || t.ContractVersion != source.ContractVersion {
			return ContextManifest{}, errors.New("Platform improvement lacks a deployed App checkout")
		}
	}
	if t.Team == App && t.Kind == "acceptance" && t.SourceTaskID != "" {
		source := st.Tasks[t.SourceTaskID]
		if source.Team == App || source.FeedbackMessageID != "" {
			if !acceptanceSourceDeployed(st, source) || t.HeadSHA != source.HeadSHA {
				return ContextManifest{}, errors.New("App acceptance lacks current deployment evidence")
			}
			ref := st.Deployments[source.ID].EvidenceRef
			m.DeploymentEvidence = &ref
		}
	}
	if memo, ok := latest(st, a.ID, "memo"); ok {
		ref := artifactRef(memo)
		m.AgentMemo = &ref
	}
	// Sort inbox to make a retry produce the same input hash.
	ids := make([]string, 0)
	for id, msg := range st.Messages {
		if msg.ToAgent == a.ID && msg.Status == "received" && msg.ContractVersion == t.ContractVersion {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		msg := st.Messages[id]
		m.MessageIDs = append(m.MessageIDs, id)
		m.Inbox = append(m.Inbox, msg.ArtifactRefs...)
	}
	if t.Team == App && t.Kind == "acceptance" && t.SourceTaskID != "" && a.SessionGeneration > 1 {
		var previous Attempt
		for _, attempt := range st.Attempts {
			if attempt.AgentID == a.ID && attempt.Generation == a.SessionGeneration-1 && attempt.Status == "completed" && attempt.OutputRef != nil && (previous.ID == "" || attempt.StartedAt.After(previous.StartedAt)) {
				previous = attempt
			}
		}
		if previous.ID == "" || previous.HeadSHA == t.HeadSHA {
			return ContextManifest{}, errors.New("retest lacks a distinct previous acceptance result")
		}
		ref := *previous.OutputRef
		m.PreviousAcceptance = &ref
	}
	for _, depID := range a.DependsOn {
		dep, ok := st.Agents[depID]
		if !ok || dep.TaskID != t.ID || dep.Status != "completed" {
			return ContextManifest{}, errors.New("stage dependency not completed in same task")
		}
		var result Attempt
		for _, attempt := range st.Attempts {
			if attempt.AgentID == depID && attempt.Status == "completed" && attempt.OutputRef != nil && (result.ID == "" || attempt.StartedAt.After(result.StartedAt)) {
				result = attempt
			}
		}
		// Requirements and design review are fixed to the contract and base,
		// before implementation advances the task head. Code-dependent stages
		// must match the current head exactly.
		preImplementation := dep.Role == "requirements" || dep.Role == "design_review"
		if result.ID == "" || result.ContractVersion != t.ContractVersion || result.BaseSHA != t.BaseSHA || !preImplementation && result.HeadSHA != t.HeadSHA {
			return ContextManifest{}, errors.New("stage dependency result missing or stale")
		}
		m.StageInputs = append(m.StageInputs, StageInput{AgentID: dep.ID, Role: dep.Role, Artifact: *result.OutputRef})
	}
	for _, ref := range append([]ArtifactRef{m.TeamProfile, m.TeamKnowledge, m.CommonKnowledge}, m.Inbox...) {
		if err := s.verifyRef(ref); err != nil {
			return ContextManifest{}, fmt.Errorf("context artifact %s: %w", ref.ID, err)
		}
	}
	for _, stage := range m.StageInputs {
		if err := s.verifyRef(stage.Artifact); err != nil {
			return ContextManifest{}, fmt.Errorf("stage result %s: %w", stage.AgentID, err)
		}
	}
	for _, ref := range m.ReviewInputs {
		if err := s.verifyRef(ref); err != nil {
			return ContextManifest{}, fmt.Errorf("review input %s: %w", ref.ID, err)
		}
	}
	if m.DeploymentEvidence != nil {
		if err := s.verifyRef(*m.DeploymentEvidence); err != nil {
			return ContextManifest{}, fmt.Errorf("deployment evidence: %w", err)
		}
		artifact := st.Artifacts[m.DeploymentEvidence.ID]
		if artifact.AgentID != "system" || artifact.Kind != "deployment_observation" {
			return ContextManifest{}, errors.New("deployment evidence is not a host observation")
		}
	}
	if m.PreviousAcceptance != nil {
		if err := s.verifyRef(*m.PreviousAcceptance); err != nil {
			return ContextManifest{}, fmt.Errorf("previous acceptance result: %w", err)
		}
	}
	if m.AgentMemo != nil {
		if err := s.verifyRef(*m.AgentMemo); err != nil {
			return ContextManifest{}, err
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ContextManifest{}, err
	}
	m.InputSHA256 = digest(b)
	return m, nil
}

func (s *Store) verifyRef(ref ArtifactRef) error {
	st := s.Snapshot()
	a, ok := st.Artifacts[ref.ID]
	if !ok || a.Version != ref.Version || a.SHA256 != ref.SHA256 {
		return errors.New("artifact reference mismatch")
	}
	_, err := s.ReadArtifact(ref.ID)
	return err
}
