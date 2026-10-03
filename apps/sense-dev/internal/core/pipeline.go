package core

import (
	"errors"
	"strings"
	"time"
)

type stageSpec struct {
	role, provider, model string
}

func defaultStages(kind string) ([]stageSpec, error) {
	switch kind {
	case "change":
		return []stageSpec{
			{"requirements", "claude", "claude-opus-5-5"},
			{"design_review", "codex", "gpt-6-astra"},
			{"implementation", "codex", "gpt-6-sol"},
			{"verification", "system", "local"},
			{"implementation_review", "claude", "opus"},
		}, nil
	case "acceptance":
		return []stageSpec{{"app_acceptance", "claude", "opus"}}, nil
	case "analysis":
		return []stageSpec{{"feedback", "claude", "claude-opus-5-5"}}, nil
	default:
		return nil, errors.New("unsupported task kind")
	}
}

// CreatePlannedTask atomically records the task and its independent agents.
// The stages are dependencies, not a shared chat. No provider is started here.
func (s *Store) CreatePlannedTask(mission string, team Team, kind, title, contract string) (Task, []Agent, error) {
	if strings.TrimSpace(mission) == "" || !validTeam(team) || strings.TrimSpace(title) == "" || strings.TrimSpace(contract) == "" {
		return Task{}, nil, errors.New("mission, team, title and contract version are required")
	}
	stages, err := defaultStages(kind)
	if err != nil {
		return Task{}, nil, err
	}
	if team == Platform && kind == "acceptance" {
		return Task{}, nil, errors.New("acceptance task belongs to App team")
	}
	id, err := newID()
	if err != nil {
		return Task{}, nil, err
	}
	agentIDs := make([]string, len(stages))
	for i := range stages {
		agentIDs[i], err = newID()
		if err != nil {
			return Task{}, nil, err
		}
	}
	now := time.Now().UTC()
	task := Task{ID: id, MissionID: mission, Team: team, Kind: kind, Title: title, Status: "ready", ContractVersion: contract, CreatedAt: now, UpdatedAt: now}
	agents := make([]Agent, 0, len(stages))
	for i, stage := range stages {
		a := Agent{ID: agentIDs[i], TaskID: id, Team: team, Role: stage.role, Provider: stage.provider, Model: stage.model, SessionGeneration: 1, Status: "ready", UpdatedAt: now.Add(time.Duration(i) * time.Nanosecond)}
		if i > 0 {
			a.DependsOn = append([]string(nil), agentIDs[:i]...)
		}
		agents = append(agents, a)
	}
	if err := s.update(func(st *State) error {
		st.Tasks[id] = task
		for _, a := range agents {
			st.Agents[a.ID] = a
		}
		st.Events = append(st.Events, event("task_planned", id, string(team)+":"+kind))
		return nil
	}); err != nil {
		return Task{}, nil, err
	}
	return task, agents, nil
}

// CreateLinkedAcceptanceTask makes an independent App task. Its agent cannot
// run until it receives a change_ready artifact for the current Platform head.
func (s *Store) CreateLinkedAcceptanceTask(platformTaskID, title string) (Task, Agent, error) {
	if strings.TrimSpace(platformTaskID) == "" || strings.TrimSpace(title) == "" {
		return Task{}, Agent{}, errors.New("platform task and acceptance title are required")
	}
	taskID, err := newID()
	if err != nil {
		return Task{}, Agent{}, err
	}
	agentID, err := newID()
	if err != nil {
		return Task{}, Agent{}, err
	}
	var task Task
	var agent Agent
	err = s.update(func(st *State) error {
		source, ok := st.Tasks[platformTaskID]
		if !ok || source.Team != Platform || source.Kind != "change" {
			return errors.New("source must be a Platform change task")
		}
		now := time.Now().UTC()
		task = Task{ID: taskID, MissionID: source.MissionID, Team: App, Kind: "acceptance", Title: title, Status: "ready", ContractVersion: source.ContractVersion, SourceTaskID: source.ID, CreatedAt: now, UpdatedAt: now}
		agent = Agent{ID: agentID, TaskID: taskID, Team: App, Role: "app_acceptance", Provider: "claude", Model: "opus", SessionGeneration: 1, Status: "ready", UpdatedAt: now}
		st.Tasks[task.ID] = task
		st.Agents[agent.ID] = agent
		st.Events = append(st.Events, event("acceptance_linked", task.ID, source.ID))
		return nil
	})
	return task, agent, err
}
