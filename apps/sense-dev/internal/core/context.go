package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

func (s *Store) SeedKnowledge() error {
	initial := map[string]string{
		"profile-platform":   "Platform team: own the Golden Path contract, ADRs, templates, operability, and rollback. Submit versioned contracts and evidence. Consumer constraints from App are input, not hidden shared conversation.",
		"knowledge-platform": "Current mission: complete the production Golden Path, then align existing apps. Scope and accepted contract are versioned; do not expand either from a worker memo.",
		"profile-app":        "App team: represent each consumer's UX, identity, data, and acceptance scenarios from design time. Send expected versus observed failures with evidence.",
		"knowledge-app":      "Current mission: test the Golden Path as a consumer. Preserve web-stateless, web-stateful, and batch differences. Feedback returns to Platform as a versioned artifact.",
		"knowledge-common":   "Shared: user-approved mission and scope, accepted contract, repository rules. Private team memories and author chats are never implicit shared inputs.",
	}
	st := s.Snapshot()
	for kind, body := range initial {
		if _, ok := latest(st, "system", kind); ok {
			continue
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
	m := ContextManifest{SchemaVersion: SchemaVersion, MissionID: t.MissionID, TaskID: t.ID, Team: a.Team, Role: a.Role, AgentID: a.ID, Provider: a.Provider, Model: a.Model, Generation: a.SessionGeneration, TeamProfile: artifactRef(profile), TeamKnowledge: artifactRef(knowledge), CommonKnowledge: artifactRef(common), Inbox: []ArtifactRef{}, MessageIDs: []string{}, ContractVersion: t.ContractVersion, BaseSHA: t.BaseSHA, HeadSHA: t.HeadSHA, AllowedScope: append([]string(nil), allowedScope...)}
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
	for _, ref := range append([]ArtifactRef{m.TeamProfile, m.TeamKnowledge, m.CommonKnowledge}, m.Inbox...) {
		if err := s.verifyRef(ref); err != nil {
			return ContextManifest{}, fmt.Errorf("context artifact %s: %w", ref.ID, err)
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
