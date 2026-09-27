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
		if app.Team != App || app.Kind != "acceptance" || app.SourceTaskID == "" || app.Status != "ready" || app.HeadSHA != "" || app.BaseSHA != "" {
			continue
		}
		source := st.Tasks[app.SourceTaskID]
		if source.Status != "publish_wait" || !fullSHA(source.HeadSHA) {
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
			if target.SourceTaskID != change.ID || target.Status != "ready" || target.HeadSHA != "" || target.BaseSHA != "" || change.HeadSHA != source.HeadSHA || change.ContractVersion != target.ContractVersion || change.MissionID != target.MissionID || !releaseProofStillCurrent(current, change.ID, proof) {
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
			if reviewer.ID == "" || recipient.ID == "" || recipient.Status != "ready" {
				return nil
			}
			now := time.Now().UTC()
			message := Message{ID: id, CorrelationID: change.MissionID, FromAgent: reviewer.ID, ToAgent: recipient.ID, SourceTask: change.ID, TargetTask: target.ID, Kind: "change_ready", ArtifactRefs: []ArtifactRef{proof.Implementation, proof.Verification, proof.QualityReview}, ContractVersion: change.ContractVersion, HeadSHA: change.HeadSHA, Status: "received", CreatedAt: now, ReceivedAt: &now}
			current.Messages[id] = message
			target.HeadSHA, target.UpdatedAt = change.HeadSHA, now
			current.Tasks[target.ID] = target
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
