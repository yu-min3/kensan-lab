package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

type AcceptanceResult struct {
	SchemaVersion    int                `json:"schema_version"`
	Verdict          string             `json:"verdict"`
	ScenarioID       string             `json:"scenario_id"`
	ContractVersion  string             `json:"contract_version"`
	HeadSHA          string             `json:"head_sha"`
	Expected         string             `json:"expected"`
	Observed         string             `json:"observed"`
	PlatformFeedback []PlatformFeedback `json:"platform_feedback,omitempty"`
}

func parseAcceptanceResult(body []byte, task Task) (AcceptanceResult, error) {
	if len(body) == 0 || len(body) > 64<<10 || task.SourceTaskID == "" || !fullSHA(task.HeadSHA) {
		return AcceptanceResult{}, errors.New("linked acceptance output or target is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var result AcceptanceResult
	if err := decoder.Decode(&result); err != nil {
		return AcceptanceResult{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return AcceptanceResult{}, errors.New("acceptance output has trailing data")
	}
	if result.SchemaVersion != 1 || result.Verdict != "pass" && result.Verdict != "fail" || result.ScenarioID != task.ID || result.ContractVersion != task.ContractVersion || result.HeadSHA != task.HeadSHA || result.Expected != task.Title || strings.TrimSpace(result.Observed) == "" {
		return AcceptanceResult{}, errors.New("acceptance verdict does not match pinned scenario, contract or head")
	}
	if len(result.PlatformFeedback) > 4 {
		return AcceptanceResult{}, errors.New("too many platform feedback items")
	}
	for _, feedback := range result.PlatformFeedback {
		if err := feedback.validate(); err != nil {
			return AcceptanceResult{}, err
		}
	}
	return result, nil
}

// ReconcileAcceptanceOutcomes records one versioned App result as a received
// source-team message. A failed result does not count as accepted or done.
func (s *Store) ReconcileAcceptanceOutcomes() (int, error) {
	st := s.Snapshot()
	if st.Stopped || st.PausedUntil != nil && time.Now().UTC().Before(*st.PausedUntil) {
		return 0, nil
	}
	count := 0
	for _, app := range st.Tasks {
		if app.Team != App || app.Kind != "acceptance" || app.SourceTaskID == "" || app.Status != "done" && app.Status != "revision_wait" {
			continue
		}
		var agent Agent
		for _, candidate := range st.Agents {
			if candidate.TaskID == app.ID && candidate.Role == "app_acceptance" {
				agent = candidate
				break
			}
		}
		if agent.ID == "" || agent.Status != "completed" {
			continue
		}
		var attempt Attempt
		for _, candidate := range st.Attempts {
			if candidate.AgentID == agent.ID && candidate.Generation == agent.SessionGeneration && candidate.Status == "completed" && candidate.OutputRef != nil && (attempt.ID == "" || candidate.StartedAt.After(attempt.StartedAt)) {
				attempt = candidate
			}
		}
		if attempt.ID == "" || attempt.HeadSHA != app.HeadSHA || attempt.ContractVersion != app.ContractVersion {
			continue
		}
		body, err := s.ReadArtifact(attempt.OutputRef.ID)
		if err != nil {
			if err := s.quarantineAcceptance(app.ID, attempt.ID, "artifact_unreadable"); err != nil {
				return count, err
			}
			continue
		}
		result, err := parseAcceptanceResult(body, app)
		if err != nil {
			if err := s.quarantineAcceptance(app.ID, attempt.ID, "result_invalid"); err != nil {
				return count, err
			}
			continue
		}
		messageID := "acceptance-" + attempt.ID
		created := false
		err = s.update(func(current *State) error {
			target := current.Tasks[app.ID]
			source := current.Tasks[app.SourceTaskID]
			activeAgent := current.Agents[agent.ID]
			activeAttempt := current.Attempts[attempt.ID]
			if !acceptanceSourceDeployed(*current, source) {
				if target.Status == "done" {
					target.Status, target.UpdatedAt = "decision_wait", time.Now().UTC()
					current.Tasks[target.ID] = target
					current.Events = append(current.Events, event("acceptance_needs_inspection", target.ID, "deployment_changed"))
				}
				return nil
			}
			if target.Status != "done" && target.Status != "revision_wait" || target.HeadSHA != result.HeadSHA || target.ContractVersion != result.ContractVersion || target.SourceTaskID != source.ID || !validTeam(source.Team) || source.Kind != "change" || source.MissionID != target.MissionID || source.ContractVersion != target.ContractVersion || activeAgent.Team != App || activeAgent.Status != "completed" || activeAgent.SessionGeneration != attempt.Generation || activeAttempt.Status != "completed" || activeAttempt.OutputRef == nil || *activeAttempt.OutputRef != *attempt.OutputRef {
				return nil
			}
			if prior, exists := current.Messages[messageID]; exists {
				if prior.SourceTask != target.ID || prior.TargetTask != source.ID || prior.FromAgent != agent.ID || prior.HeadSHA != result.HeadSHA || len(prior.ArtifactRefs) != 1 || prior.ArtifactRefs[0] != *attempt.OutputRef || prior.Status != "received" {
					return errors.New("acceptance message ID collision")
				}
				return nil
			}
			var recipient Agent
			for _, candidate := range current.Agents {
				if candidate.TaskID == source.ID && candidate.Role == "implementation" {
					recipient = candidate
					break
				}
			}
			if recipient.ID == "" || recipient.Team != source.Team {
				return errors.New("source-team correction agent missing")
			}
			kind := "accepted"
			if result.Verdict == "fail" {
				kind = "acceptance_failed"
			} else if source.Status != "publish_wait" || source.HeadSHA != result.HeadSHA {
				kind = "acceptance_stale"
			}
			now := time.Now().UTC()
			current.Messages[messageID] = Message{ID: messageID, CorrelationID: target.MissionID, FromAgent: agent.ID, ToAgent: recipient.ID, SourceTask: target.ID, TargetTask: source.ID, Kind: kind, ArtifactRefs: []ArtifactRef{*attempt.OutputRef}, ContractVersion: target.ContractVersion, HeadSHA: result.HeadSHA, ScenarioID: result.ScenarioID, Expected: result.Expected, Observed: result.Observed, Status: "received", CreatedAt: now, ReceivedAt: &now}
			// A reviewed App candidate is complete only after its deployed user path passes.
			if kind == "accepted" && source.Team == App && appDeploymentReady(*current, source) {
				source.Status, source.UpdatedAt = "done", now
				current.Tasks[source.ID] = source
				current.Events = append(current.Events, event("app_change_completed", source.ID, messageID))
				if source.ImageSourceTaskID != "" {
					completeImageSource(current, source, messageID, now)
				}
			}
			if result.Verdict == "fail" || kind == "acceptance_stale" {
				target.Status, target.UpdatedAt = "revision_wait", now
				current.Tasks[target.ID] = target
			}
			if result.Verdict == "fail" {
				if source.Status == "publish_wait" && source.HeadSHA == result.HeadSHA && source.ImageSourceTaskID != "" {
					if err := startImageCorrection(current, source, target, agent.ID, messageID, *attempt.OutputRef, now); err != nil {
						return err
					}
				} else if source.Status == "publish_wait" && source.HeadSHA == result.HeadSHA {
					if source.CorrectionCount >= 2 {
						source.Status = "decision_wait"
						current.Events = append(current.Events, event("correction_limit_reached", source.ID, messageID))
					} else {
						source.CorrectionCount++
						source.Status = "ready"
						for id, stage := range current.Agents {
							if stage.TaskID != source.ID || stage.Role != "implementation" && stage.Role != "verification" && stage.Role != "implementation_review" {
								continue
							}
							stage.Status, stage.SessionID, stage.InputHash, stage.RetryAfter, stage.AttemptCount = "ready", "", "", nil, 0
							stage.SessionGeneration++
							stage.UpdatedAt = now
							current.Agents[id] = stage
						}
						current.Events = append(current.Events, event("correction_started", source.ID, messageID))
					}
					source.UpdatedAt = now
					current.Tasks[source.ID] = source
				}
			}
			current.Events = append(current.Events, event("message_sent", messageID, kind), event("message_received", messageID, recipient.ID))
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
	return count, nil
}

func (s *Store) quarantineAcceptance(taskID, attemptID, reason string) error {
	return s.update(func(st *State) error {
		task, ok := st.Tasks[taskID]
		attempt := st.Attempts[attemptID]
		if !ok || task.Team != App || task.Kind != "acceptance" || task.SourceTaskID == "" || attempt.TaskID != taskID || attempt.Status != "completed" || task.Status != "done" && task.Status != "revision_wait" {
			return nil
		}
		task.Status, task.UpdatedAt = "decision_wait", time.Now().UTC()
		st.Tasks[task.ID] = task
		st.Events = append(st.Events, event("acceptance_needs_inspection", taskID, reason))
		return nil
	})
}
