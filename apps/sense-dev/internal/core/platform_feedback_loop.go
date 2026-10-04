package core

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// ReconcilePlatformFeedback advances the independent improvement loop without
// holding the originating App task while Platform decides. It never publishes.
func (s *Store) ReconcilePlatformFeedback() (int, error) {
	st := s.Snapshot()
	if st.Stopped || st.PausedUntil != nil && time.Now().UTC().Before(*st.PausedUntil) {
		return 0, nil
	}
	count := 0
	for _, step := range []func() (int, error){s.collectPlatformFeedback, s.deliverPlatformDecisions, s.advancePlatformImprovements} {
		n, err := step()
		count += n
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

func (s *Store) collectPlatformFeedback() (int, error) {
	st := s.Snapshot()
	count := 0
	for _, attempt := range st.Attempts {
		if attempt.Role != "app_acceptance" || attempt.Status != "completed" || attempt.OutputRef == nil {
			continue
		}
		app := st.Tasks[attempt.TaskID]
		source := st.Tasks[app.SourceTaskID]
		acceptance, ok := st.Messages["acceptance-"+attempt.ID]
		if !ok || acceptance.Status != "received" || app.Team != App || app.Kind != "acceptance" || source.Team != App || source.Kind != "change" || !appDeploymentReady(st, source) || !fullSHA(source.BaseSHA) {
			continue
		}
		body, err := s.ReadArtifact(attempt.OutputRef.ID)
		if err != nil {
			return count, err
		}
		result, err := parseAcceptanceResult(body, Task{SourceTaskID: app.SourceTaskID, HeadSHA: attempt.HeadSHA, ID: app.ID, ContractVersion: app.ContractVersion, Title: app.Title})
		if err != nil {
			return count, err
		}
		for i, feedback := range result.PlatformFeedback {
			id := "platform-feedback-" + attempt.ID + "-" + strconv.Itoa(i)
			if _, exists := s.Snapshot().FeedbackLoops[id]; exists {
				continue
			}
			artifact, err := s.PutPlatformFeedback(attempt.AgentID, feedback)
			if err != nil {
				return count, err
			}
			taskID, err := newID()
			if err != nil {
				return count, err
			}
			agentID, err := newID()
			if err != nil {
				return count, err
			}
			created := false
			err = s.update(func(current *State) error {
				if _, exists := current.FeedbackLoops[id]; exists {
					return nil
				}
				origin := current.Messages["acceptance-"+attempt.ID]
				parent := current.Tasks[app.SourceTaskID]
				if origin.Status != "received" || origin.FromAgent != attempt.AgentID || parent.HeadSHA != attempt.HeadSHA || !appDeploymentReady(*current, parent) || parent.BaseSHA != source.BaseSHA {
					return nil
				}
				now := time.Now().UTC()
				task := Task{ID: taskID, MissionID: app.MissionID, Team: Platform, Kind: "analysis", Title: "Assess platform feedback: " + feedback.Summary, Status: "ready", ContractVersion: app.ContractVersion, BaseSHA: parent.BaseSHA, FeedbackMessageID: id, CreatedAt: now, UpdatedAt: now}
				agent := Agent{ID: agentID, TaskID: taskID, Team: Platform, Role: "feedback", Provider: "claude", Model: "claude-opus-5-5", SessionGeneration: 1, Status: "ready", UpdatedAt: now}
				msg := Message{ID: id, CorrelationID: id, FromAgent: attempt.AgentID, ToAgent: agentID, SourceTask: app.ID, TargetTask: taskID, Kind: "platform_feedback", ArtifactRefs: []ArtifactRef{artifactRef(artifact)}, ContractVersion: app.ContractVersion, HeadSHA: attempt.HeadSHA, Status: "received", CreatedAt: now, ReceivedAt: &now}
				current.Tasks[taskID], current.Agents[agentID], current.Messages[id] = task, agent, msg
				current.FeedbackLoops[id] = FeedbackLoop{FeedbackID: id, AnalysisTaskID: taskID, Status: "pending_decision"}
				current.Events = append(current.Events, event("task_planned", taskID, "platform:analysis"), event("message_sent", id, msg.Kind), event("message_received", id, agentID))
				created = true
				return nil
			})
			if err != nil {
				return count, err
			}
			if created {
				count++
			}
		}
	}
	return count, nil
}

func (s *Store) deliverPlatformDecisions() (int, error) {
	count := 0
	for id, loop := range s.Snapshot().FeedbackLoops {
		if loop.Status != "pending_decision" {
			continue
		}
		st := s.Snapshot()
		analysis := st.Tasks[loop.AnalysisTaskID]
		if analysis.Status != "done" {
			continue
		}
		var agent Agent
		var attempt Attempt
		for _, candidate := range st.Agents {
			if candidate.TaskID == analysis.ID && candidate.Role == "feedback" {
				agent = candidate
				break
			}
		}
		for _, candidate := range st.Attempts {
			if candidate.AgentID == agent.ID && candidate.Status == "completed" && candidate.OutputRef != nil && (attempt.ID == "" || candidate.StartedAt.After(attempt.StartedAt)) {
				attempt = candidate
			}
		}
		if attempt.ID == "" {
			continue
		}
		body, err := s.ReadArtifact(attempt.OutputRef.ID)
		if err != nil {
			return count, err
		}
		var decision PlatformDecision
		if err := decodePlatformEnvelope(body, &decision); err != nil || decision.validate() != nil || decision.FeedbackMessageID != id {
			return count, errors.New("completed platform analysis has invalid decision")
		}
		messageID := "platform-decision-" + id
		if _, exists := st.Messages[messageID]; !exists {
			artifact, err := s.PutPlatformDecision(agent.ID, decision)
			if err != nil {
				return count, err
			}
			feedback := st.Messages[id]
			message := Message{ID: messageID, CorrelationID: feedback.CorrelationID, ReplyTo: id, FromAgent: agent.ID, ToAgent: feedback.FromAgent, SourceTask: analysis.ID, TargetTask: feedback.SourceTask, Kind: "platform_decision", ArtifactRefs: []ArtifactRef{artifactRef(artifact)}, ContractVersion: feedback.ContractVersion}
			if _, err := s.SendMessage(message); err != nil {
				return count, err
			}
		}
		if err := s.ReceiveMessage(st.Messages[id].FromAgent, messageID); err != nil {
			return count, err
		}
		err = s.update(func(current *State) error {
			item := current.FeedbackLoops[id]
			if item.Status != "pending_decision" || current.Messages[messageID].Status != "received" {
				return nil
			}
			item.DecisionID = messageID
			switch decision.Verdict {
			case "adopt":
				item.Status = "adopted"
			case "reject":
				item.Status = "rejected"
			case "defer":
				item.Status = "deferred"
			}
			current.FeedbackLoops[id] = item
			current.Events = append(current.Events, event("platform_decision", id, decision.Verdict))
			return nil
		})
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *Store) advancePlatformImprovements() (int, error) {
	count := 0
	for id, loop := range s.Snapshot().FeedbackLoops {
		if loop.Status == "adopted" && loop.ImprovementTaskID == "" {
			n, err := s.createPlatformImprovement(id)
			count += n
			if err != nil {
				return count, err
			}
			continue
		}
		if loop.ImprovementTaskID == "" {
			continue
		}
		st := s.Snapshot()
		improvement := st.Tasks[loop.ImprovementTaskID]
		if loop.RetestTaskID == "" && appDeploymentReady(st, improvement) {
			n, err := s.createPlatformRetest(id)
			count += n
			if err != nil {
				return count, err
			}
			continue
		}
		if loop.RetestTaskID != "" && loop.Status != "resolved" {
			retest := st.Tasks[loop.RetestTaskID]
			if retest.BaseSHA == "" && fullSHA(retest.HeadSHA) && appDeploymentReady(st, improvement) {
				if err := s.SetBaseSHA(retest.ID, retest.HeadSHA); err != nil {
					return count, err
				}
				count++
			}
			for _, message := range st.Messages {
				if message.SourceTask == retest.ID && message.Status == "received" && message.Kind == "accepted" {
					if err := s.update(func(current *State) error {
						item := current.FeedbackLoops[id]
						change := current.Tasks[item.ImprovementTaskID]
						if item.Status == "resolved" || !appDeploymentReady(*current, change) || change.HeadSHA != message.HeadSHA || current.Tasks[item.RetestTaskID].HeadSHA != message.HeadSHA {
							return nil
						}
						item.Status = "resolved"
						change.Status, change.UpdatedAt = "done", time.Now().UTC()
						current.Tasks[change.ID] = change
						current.FeedbackLoops[id] = item
						current.Events = append(current.Events, event("platform_improvement_resolved", change.ID, id))
						return nil
					}); err != nil {
						return count, err
					}
					count++
					break
				}
			}
		}
	}
	return count, nil
}

func (s *Store) createPlatformImprovement(feedbackID string) (int, error) {
	st := s.Snapshot()
	feedback := st.Messages[feedbackID]
	app := st.Tasks[feedback.SourceTask]
	source := st.Tasks[app.SourceTaskID]
	if source.Team != App || !appDeploymentReady(st, source) || source.HeadSHA != feedback.HeadSHA {
		return 0, nil
	}
	content, err := s.ReadPlatformFeedback(feedbackID)
	if err != nil {
		return 0, err
	}
	categories, reasons := ClassifyPlatformProposal(nil, content.Category+"\n"+content.Summary+"\n"+content.Expected+"\n"+content.Observed+"\n"+content.Reproduce)
	taskID, err := newID()
	if err != nil {
		return 0, err
	}
	stages, err := defaultStages("change")
	if err != nil {
		return 0, err
	}
	agentIDs := make([]string, len(stages))
	for i := range stages {
		agentIDs[i], err = newID()
		if err != nil {
			return 0, err
		}
	}
	created := false
	err = s.update(func(current *State) error {
		item := current.FeedbackLoops[feedbackID]
		base := current.Tasks[app.SourceTaskID]
		if item.Status != "adopted" || item.ImprovementTaskID != "" || !appDeploymentReady(*current, base) || base.HeadSHA != feedback.HeadSHA {
			return nil
		}
		now := time.Now().UTC()
		task := Task{ID: taskID, MissionID: app.MissionID, Team: Platform, Kind: "change", Title: "Improve platform: " + content.Summary, Status: "ready", ContractVersion: app.ContractVersion, BaseSHA: current.Deployments[base.ID].Revision, CheckoutSourceTaskID: base.ID, FeedbackMessageID: feedbackID, CreatedAt: now, UpdatedAt: now}
		current.Tasks[taskID] = task
		for i, stage := range stages {
			a := Agent{ID: agentIDs[i], TaskID: taskID, Team: Platform, Role: stage.role, Provider: stage.provider, Model: stage.model, SessionGeneration: 1, Status: "ready", UpdatedAt: now.Add(time.Duration(i) * time.Nanosecond)}
			if i > 0 {
				a.DependsOn = append([]string(nil), agentIDs[:i]...)
			}
			current.Agents[a.ID] = a
		}
		item.ImprovementTaskID, item.Status = taskID, "improving"
		item.HumanCategories, item.HumanReasons = categories, reasons
		if len(categories) > 0 {
			item.Status = "needs_human"
			current.Events = append(current.Events, event("platform_proposal_needs_human", taskID, feedbackID))
		}
		current.FeedbackLoops[feedbackID] = item
		current.Events = append(current.Events, event("platform_improvement_started", taskID, feedbackID))
		created = true
		return nil
	})
	if err != nil {
		return 0, err
	}
	if created {
		return 1, nil
	}
	return 0, nil
}

func (s *Store) createPlatformRetest(feedbackID string) (int, error) {
	taskID, err := newID()
	if err != nil {
		return 0, err
	}
	agentID, err := newID()
	if err != nil {
		return 0, err
	}
	created := false
	err = s.update(func(st *State) error {
		item := st.FeedbackLoops[feedbackID]
		change := st.Tasks[item.ImprovementTaskID]
		feedback := st.Messages[feedbackID]
		origin := st.Tasks[feedback.SourceTask]
		if item.ImprovementTaskID == "" || item.RetestTaskID != "" || change.Team != Platform || change.Kind != "change" || change.FeedbackMessageID != feedbackID || !appDeploymentReady(*st, change) || origin.Team != App || origin.Kind != "acceptance" {
			return nil
		}
		now := time.Now().UTC()
		task := Task{ID: taskID, MissionID: origin.MissionID, Team: App, Kind: "acceptance", Title: origin.Title, Status: "ready", ContractVersion: origin.ContractVersion, SourceTaskID: change.ID, FeedbackMessageID: feedbackID, CreatedAt: now, UpdatedAt: now}
		agent := Agent{ID: agentID, TaskID: taskID, Team: App, Role: "app_acceptance", Provider: "claude", Model: "opus", SessionGeneration: 1, Status: "ready", UpdatedAt: now}
		st.Tasks[taskID], st.Agents[agentID] = task, agent
		item.RetestTaskID, item.Status = taskID, "retest_ready"
		st.FeedbackLoops[feedbackID] = item
		st.Events = append(st.Events, event("platform_retest_planned", taskID, feedbackID))
		created = true
		return nil
	})
	if err != nil {
		return 0, err
	}
	if created {
		return 1, nil
	}
	return 0, nil
}

func feedbackLoopForTask(st State, taskID string) (FeedbackLoop, error) {
	t := st.Tasks[taskID]
	if t.FeedbackMessageID == "" {
		return FeedbackLoop{}, errors.New("task has no platform feedback")
	}
	loop, ok := st.FeedbackLoops[t.FeedbackMessageID]
	if !ok || loop.FeedbackID != t.FeedbackMessageID {
		return FeedbackLoop{}, fmt.Errorf("platform feedback loop %s missing", t.FeedbackMessageID)
	}
	return loop, nil
}

func acceptanceSourceDeployed(st State, source Task) bool {
	if source.Team == Platform && source.FeedbackMessageID == "" {
		return true // Existing Platform handoffs keep their prior contract.
	}
	return appDeploymentReady(st, source)
}
