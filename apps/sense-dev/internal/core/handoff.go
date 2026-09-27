package core

import (
	"errors"
	"time"
)

// reviewedChangeProof reuses the release evidence validator so App never
// accepts a completed-but-rejected quality review as a ready change.
func (s *Store) reviewedChangeProof(taskID string) (releaseProof, error) {
	st := s.Snapshot()
	var author Agent
	for _, agent := range st.Agents {
		if agent.TaskID == taskID && agent.Role == "implementation" {
			if author.ID != "" {
				return releaseProof{}, errors.New("multiple implementation agents")
			}
			author = agent
		}
	}
	if author.ID == "" {
		return releaseProof{}, errors.New("implementation agent missing")
	}
	var refs []ArtifactRef
	for _, attempt := range st.Attempts {
		if attempt.AgentID == author.ID && attempt.Status == "completed" && attempt.OutputRef != nil {
			refs = append(refs, *attempt.OutputRef)
		}
	}
	return s.releaseReady(author.ID, refs)
}

// ReconcileReviewedHandoffs atomically delivers pinned change evidence to
// linked App tasks. Repeated ticks and restarts do not duplicate messages.
func (s *Store) ReconcileReviewedHandoffs() (int, error) {
	st := s.Snapshot()
	if st.Stopped || st.PausedUntil != nil && time.Now().UTC().Before(*st.PausedUntil) {
		return 0, nil
	}
	delivered := 0
	for _, app := range st.Tasks {
		initial := app.Status == "ready" && app.HeadSHA == "" && app.BaseSHA == ""
		retest := app.Status == "revision_wait" && fullSHA(app.HeadSHA) && app.BaseSHA == app.HeadSHA
		if app.Team != App || app.Kind != "acceptance" || app.SourceTaskID == "" || !initial && !retest {
			continue
		}
		source := st.Tasks[app.SourceTaskID]
		if source.Status != "publish_wait" || !fullSHA(source.HeadSHA) || retest && source.HeadSHA == app.HeadSHA {
			continue
		}
		proof, err := s.reviewedChangeProof(source.ID)
		if err != nil {
			continue // Incomplete or rejected proof keeps App waiting.
		}
		id, err := newID()
		if err != nil {
			return delivered, err
		}
		created := false
		err = s.update(func(current *State) error {
			if current.Stopped || current.PausedUntil != nil && time.Now().UTC().Before(*current.PausedUntil) {
				return nil
			}
			target := current.Tasks[app.ID]
			change := current.Tasks[source.ID]
			stillInitial := target.Status == "ready" && target.HeadSHA == "" && target.BaseSHA == ""
			stillRetest := target.Status == "revision_wait" && fullSHA(target.HeadSHA) && target.BaseSHA == target.HeadSHA && target.HeadSHA != change.HeadSHA
			if target.SourceTaskID != change.ID || !stillInitial && !stillRetest || change.HeadSHA != source.HeadSHA || change.ContractVersion != target.ContractVersion || change.MissionID != target.MissionID || !releaseProofStillCurrent(current, change.ID, proof) {
				return nil
			}
			var reviewer, recipient Agent
			for _, agent := range current.Agents {
				if agent.TaskID == change.ID && agent.Role == "implementation_review" {
					reviewer = agent
				}
				if agent.TaskID == target.ID && agent.Role == "app_acceptance" {
					recipient = agent
				}
			}
			if reviewer.ID == "" || recipient.ID == "" || stillInitial && recipient.Status != "ready" || stillRetest && recipient.Status != "completed" {
				return nil
			}
			now := time.Now().UTC()
			message := Message{ID: id, CorrelationID: change.MissionID, FromAgent: reviewer.ID, ToAgent: recipient.ID, SourceTask: change.ID, TargetTask: target.ID, Kind: "change_ready", ArtifactRefs: []ArtifactRef{proof.Implementation, proof.Verification, proof.QualityReview}, ContractVersion: change.ContractVersion, HeadSHA: change.HeadSHA, Status: "received", CreatedAt: now, ReceivedAt: &now}
			current.Messages[id] = message
			target.HeadSHA, target.Status, target.UpdatedAt = change.HeadSHA, "ready", now
			current.Tasks[target.ID] = target
			if stillRetest {
				recipient.Status, recipient.SessionID, recipient.InputHash, recipient.RetryAfter, recipient.AttemptCount = "ready", "", "", nil, 0
				recipient.SessionGeneration++
				recipient.UpdatedAt = now
				current.Agents[recipient.ID] = recipient
			}
			current.Events = append(current.Events, event("message_sent", id, message.Kind), event("message_received", id, recipient.ID))
			created = true
			return nil
		})
		if err != nil {
			return delivered, err
		}
		if created {
			delivered++
		}
	}
	return delivered, nil
}

// AdvanceAcceptanceBase records a verified fast-forward of the App checkout.
// The ordinary SetBaseSHA remains immutable; only a received new revision of
// the same linked scenario may advance this base.
func (s *Store) AdvanceAcceptanceBase(taskID, fromHead, toHead string) error {
	if !fullSHA(fromHead) || !fullSHA(toHead) || fromHead == toHead {
		return errors.New("invalid acceptance revision")
	}
	return s.update(func(st *State) error {
		app, ok := st.Tasks[taskID]
		if !ok || app.Team != App || app.Kind != "acceptance" || app.SourceTaskID == "" || app.Status != "ready" || app.BaseSHA != fromHead || app.HeadSHA != toHead {
			return errors.New("App revision is not awaiting checkout advance")
		}
		source := st.Tasks[app.SourceTaskID]
		if source.Status != "publish_wait" || source.HeadSHA != toHead || source.MissionID != app.MissionID || source.ContractVersion != app.ContractVersion {
			return errors.New("Platform revision changed before checkout advance")
		}
		found := false
		for _, message := range st.Messages {
			if message.Status == "received" && message.Kind == "change_ready" && message.SourceTask == source.ID && message.TargetTask == app.ID && message.HeadSHA == toHead && message.ContractVersion == app.ContractVersion {
				found = true
				break
			}
		}
		if !found {
			return errors.New("new acceptance revision was not received")
		}
		app.BaseSHA, app.UpdatedAt = toHead, time.Now().UTC()
		st.Tasks[app.ID] = app
		st.Events = append(st.Events, event("acceptance_base_advanced", app.ID, toHead))
		return nil
	})
}
