package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func validTeam(t Team) bool { return t == Platform || t == App }

func (s *Store) CreateTask(mission string, team Team, kind, title, contract string) (Task, error) {
	if mission == "" || !validTeam(team) || kind == "" || strings.TrimSpace(title) == "" || contract == "" {
		return Task{}, errors.New("mission, team, kind, title and contract version are required")
	}
	id, err := newID()
	if err != nil {
		return Task{}, err
	}
	now := time.Now().UTC()
	t := Task{ID: id, MissionID: mission, Team: team, Kind: kind, Title: title, Status: "ready", ContractVersion: contract, CreatedAt: now, UpdatedAt: now}
	err = s.update(func(st *State) error {
		st.Tasks[id] = t
		st.Events = append(st.Events, event("task_created", id, string(team)))
		return nil
	})
	return t, err
}

func (s *Store) AddAgent(taskID, role, provider, model string) (Agent, error) {
	return s.AddAgentWithDeps(taskID, role, provider, model, nil)
}

func (s *Store) AddAgentWithDeps(taskID, role, provider, model string, dependsOn []string) (Agent, error) {
	if taskID == "" || role == "" || provider == "" || model == "" {
		return Agent{}, errors.New("task, role, provider and model are required")
	}
	id, err := newID()
	if err != nil {
		return Agent{}, err
	}
	var a Agent
	err = s.update(func(st *State) error {
		t, ok := st.Tasks[taskID]
		if !ok {
			return errors.New("task not found")
		}
		seen := map[string]bool{}
		for _, depID := range dependsOn {
			dep, ok := st.Agents[depID]
			if !ok || dep.TaskID != taskID || seen[depID] {
				return errors.New("agent dependency must be a distinct agent in the same task")
			}
			seen[depID] = true
		}
		a = Agent{ID: id, TaskID: taskID, Team: t.Team, Role: role, Provider: provider, Model: model, SessionGeneration: 1, MemoVersion: 0, Status: "ready", DependsOn: append([]string(nil), dependsOn...), UpdatedAt: time.Now().UTC()}
		st.Agents[id] = a
		st.Events = append(st.Events, event("agent_created", id, taskID))
		return nil
	})
	return a, err
}

func (s *Store) PutArtifact(agentID, kind string, content []byte) (Artifact, error) {
	if len(content) == 0 || len(content) > 2<<20 || kind == "" {
		return Artifact{}, errors.New("artifact must be 1 byte to 2 MiB with a kind")
	}
	if strings.ContainsAny(kind, "/\\\x00") || strings.Contains(kind, "..") {
		return Artifact{}, errors.New("invalid artifact kind")
	}
	if agentID != "system" {
		if _, ok := s.Snapshot().Agents[agentID]; !ok {
			return Artifact{}, errors.New("agent not found")
		}
	}
	id, err := newID()
	if err != nil {
		return Artifact{}, err
	}
	// Content is immutable and never executes as an instruction.
	path := filepath.Join(s.root, "artifacts", id)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return Artifact{}, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return Artifact{}, err
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		return Artifact{}, err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return Artifact{}, err
	}
	if err := f.Close(); err != nil {
		return Artifact{}, err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return Artifact{}, err
	}
	if err := dir.Sync(); err != nil {
		dir.Close()
		return Artifact{}, err
	}
	if err := dir.Close(); err != nil {
		return Artifact{}, err
	}
	var out Artifact
	err = s.update(func(st *State) error {
		if agentID != "system" {
			if _, ok := st.Agents[agentID]; !ok {
				return errors.New("agent not found")
			}
		}
		version := 1
		for _, old := range st.Artifacts {
			if old.AgentID == agentID && old.Kind == kind && old.Version >= version {
				version = old.Version + 1
			}
		}
		taskID := ""
		if agentID != "system" {
			taskID = st.Agents[agentID].TaskID
		}
		out = Artifact{ID: id, TaskID: taskID, AgentID: agentID, Kind: kind, Version: version, SHA256: digest(content), Path: filepath.Join("artifacts", id), CreatedAt: time.Now().UTC()}
		st.Artifacts[id] = out
		st.Events = append(st.Events, event("artifact_created", id, kind))
		return nil
	})
	return out, err
}

func (s *Store) ReadArtifact(id string) ([]byte, error) {
	st := s.Snapshot()
	a, ok := st.Artifacts[id]
	if !ok {
		return nil, errors.New("artifact not found")
	}
	b, err := os.ReadFile(filepath.Join(s.root, a.Path))
	if err != nil {
		return nil, err
	}
	if digest(b) != a.SHA256 {
		return nil, errors.New("artifact hash mismatch")
	}
	return b, nil
}

func artifactRef(a Artifact) ArtifactRef {
	return ArtifactRef{ID: a.ID, Version: a.Version, SHA256: a.SHA256}
}

func (s *Store) SendMessage(m Message) (Message, error) {
	if m.ID == "" {
		id, err := newID()
		if err != nil {
			return Message{}, err
		}
		m.ID = id
	}
	if m.CorrelationID == "" || m.FromAgent == "" || m.ToAgent == "" || m.Kind == "" || len(m.ArtifactRefs) == 0 {
		return Message{}, errors.New("message id, correlation, sender, recipient, kind and artifacts are required")
	}
	if m.Kind == "acceptance_failed" && (m.ScenarioID == "" || m.Expected == "" || m.Observed == "") {
		return Message{}, errors.New("acceptance failure needs scenario, expected and observed")
	}
	for _, ref := range m.ArtifactRefs {
		if err := s.verifyRef(ref); err != nil {
			return Message{}, fmt.Errorf("invalid outgoing artifact: %w", err)
		}
	}
	err := s.update(func(st *State) error {
		if old, ok := st.Messages[m.ID]; ok {
			if sameMessage(old, m) {
				m = old
				return nil
			}
			return errors.New("message ID reused with different content")
		}
		from, okFrom := st.Agents[m.FromAgent]
		to, okTo := st.Agents[m.ToAgent]
		if !okFrom || !okTo || from.ID == to.ID || from.Team == to.Team {
			return errors.New("cross-team sender and recipient agents required")
		}
		if m.SourceTask != from.TaskID || m.TargetTask != to.TaskID {
			return errors.New("message task ownership mismatch")
		}
		fromTask, toTask := st.Tasks[from.TaskID], st.Tasks[to.TaskID]
		if fromTask.MissionID != toTask.MissionID || fromTask.ContractVersion != m.ContractVersion || toTask.ContractVersion != m.ContractVersion {
			return errors.New("mission or contract version mismatch")
		}
		if toTask.SourceTaskID != "" && (m.Kind != "change_ready" || m.SourceTask != toTask.SourceTaskID || !fullSHA(m.HeadSHA) || !reviewedPlatformChange(fromTask, from)) {
			return errors.New("linked acceptance requires reviewed Platform change_ready at a full head SHA")
		}
		if m.HeadSHA != "" && fromTask.HeadSHA != m.HeadSHA {
			return errors.New("stale head SHA")
		}
		if m.ReplyTo != "" {
			prior, ok := st.Messages[m.ReplyTo]
			if !ok || prior.ToAgent != from.ID || prior.FromAgent != to.ID || prior.CorrelationID != m.CorrelationID {
				return errors.New("reply relationship mismatch")
			}
		}
		for _, ref := range m.ArtifactRefs {
			a, ok := st.Artifacts[ref.ID]
			if !ok || a.AgentID != from.ID || a.Version != ref.Version || a.SHA256 != ref.SHA256 {
				return errors.New("artifact reference mismatch")
			}
		}
		m.Status = "pending"
		m.CreatedAt = time.Now().UTC()
		st.Messages[m.ID] = m
		st.Events = append(st.Events, event("message_sent", m.ID, m.Kind))
		return nil
	})
	return m, err
}

func sameMessage(old, candidate Message) bool {
	return old.CorrelationID == candidate.CorrelationID && old.FromAgent == candidate.FromAgent && old.ToAgent == candidate.ToAgent && old.Kind == candidate.Kind && old.SourceTask == candidate.SourceTask && old.TargetTask == candidate.TargetTask && old.ContractVersion == candidate.ContractVersion && old.HeadSHA == candidate.HeadSHA && old.ReplyTo == candidate.ReplyTo && old.ScenarioID == candidate.ScenarioID && old.Expected == candidate.Expected && old.Observed == candidate.Observed && refsEqual(old.ArtifactRefs, candidate.ArtifactRefs)
}

func refsEqual(a, b []ArtifactRef) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (s *Store) ReceiveMessage(agentID, messageID string) error {
	st := s.Snapshot()
	m, ok := st.Messages[messageID]
	if !ok || m.ToAgent != agentID {
		return errors.New("message not found for agent")
	}
	for _, ref := range m.ArtifactRefs {
		if err := s.verifyRef(ref); err != nil {
			return err
		}
	}
	return s.update(func(st *State) error {
		m, ok := st.Messages[messageID]
		if !ok || m.ToAgent != agentID {
			return errors.New("message not found for agent")
		}
		if m.Status == "received" {
			return nil
		}
		if m.HeadSHA != "" && st.Tasks[m.SourceTask].HeadSHA != m.HeadSHA {
			return errors.New("message source head is stale")
		}
		target := st.Tasks[m.TargetTask]
		if target.SourceTaskID != "" {
			if m.Kind != "change_ready" || m.SourceTask != target.SourceTaskID || !fullSHA(m.HeadSHA) || target.HeadSHA != "" && target.HeadSHA != m.HeadSHA {
				return errors.New("linked acceptance handoff is stale or from another task")
			}
		}
		for _, ref := range m.ArtifactRefs {
			a, ok := st.Artifacts[ref.ID]
			if !ok || a.SHA256 != ref.SHA256 || a.Version != ref.Version {
				return errors.New("artifact changed before receive")
			}
		}
		now := time.Now().UTC()
		m.Status, m.ReceivedAt = "received", &now
		st.Messages[messageID] = m
		if target.SourceTaskID != "" {
			target.HeadSHA, target.UpdatedAt = m.HeadSHA, now
			st.Tasks[target.ID] = target
		}
		st.Events = append(st.Events, event("message_received", messageID, agentID))
		return nil
	})
}

func (s *Store) SetHeadSHA(taskID, sha string) error {
	if !fullSHA(sha) {
		return errors.New("full lowercase Git SHA required")
	}
	return s.update(func(st *State) error {
		t, ok := st.Tasks[taskID]
		if !ok {
			return errors.New("task not found")
		}
		t.HeadSHA = sha
		t.UpdatedAt = time.Now().UTC()
		st.Tasks[taskID] = t
		st.Events = append(st.Events, event("head_updated", taskID, sha))
		return nil
	})
}

func (s *Store) SetBaseSHA(taskID, sha string) error {
	if !fullSHA(sha) {
		return errors.New("full lowercase Git SHA required")
	}
	return s.update(func(st *State) error {
		t, ok := st.Tasks[taskID]
		if !ok {
			return errors.New("task not found")
		}
		if t.BaseSHA != "" && t.BaseSHA != sha {
			return errors.New("task base SHA is immutable")
		}
		t.BaseSHA = sha
		t.UpdatedAt = time.Now().UTC()
		st.Tasks[taskID] = t
		st.Events = append(st.Events, event("base_pinned", taskID, sha))
		return nil
	})
}

func (s *Store) SetAgentSession(agentID, provider, model, sessionID, inputHash string, generation int) error {
	if sessionID == "" || len(inputHash) != 64 {
		return errors.New("session and input manifest hash required")
	}
	return s.update(func(st *State) error {
		a, ok := st.Agents[agentID]
		if !ok || a.Provider != provider || a.Model != model || a.SessionGeneration != generation {
			return errors.New("session owner, provider, model or generation mismatch")
		}
		for otherID, other := range st.Agents {
			if otherID != agentID && other.Provider == provider && other.SessionID == sessionID {
				return errors.New("provider session already belongs to another agent")
			}
		}
		for _, attempt := range st.Attempts {
			if attempt.AgentID != agentID && attempt.Provider == provider && attempt.SessionID == sessionID {
				return errors.New("historical provider session belongs to another agent")
			}
		}
		if a.SessionID != "" && (a.SessionID != sessionID || a.InputHash != inputHash) {
			return errors.New("existing session bound to different input")
		}
		a.SessionID, a.InputHash, a.UpdatedAt = sessionID, inputHash, time.Now().UTC()
		st.Agents[agentID] = a
		st.Events = append(st.Events, event("session_bound", agentID, fmt.Sprintf("generation %d", generation)))
		return nil
	})
}

func (s *Store) NewSessionGeneration(agentID string) error {
	return s.update(func(st *State) error {
		a, ok := st.Agents[agentID]
		if !ok {
			return errors.New("agent not found")
		}
		a.SessionGeneration++
		a.SessionID, a.InputHash = "", ""
		a.UpdatedAt = time.Now().UTC()
		st.Agents[agentID] = a
		st.Events = append(st.Events, event("session_rebuilt", agentID, fmt.Sprintf("generation %d", a.SessionGeneration)))
		return nil
	})
}

func (s *Store) SetStopped(stopped bool) error {
	return s.update(func(st *State) error {
		st.Stopped = stopped
		st.Events = append(st.Events, event("controller_stopped", "global", fmt.Sprint(stopped)))
		return nil
	})
}

func (s *Store) SetMacPriority(until time.Time) error {
	return s.update(func(st *State) error {
		if until.IsZero() {
			st.PausedUntil = nil
		} else {
			t := until.UTC()
			st.PausedUntil = &t
		}
		st.Events = append(st.Events, event("mac_priority", "global", until.UTC().Format(time.RFC3339)))
		return nil
	})
}
