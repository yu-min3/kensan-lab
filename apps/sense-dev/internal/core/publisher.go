package core

import (
	"context"
	"errors"
	"time"
)

// PublishTransport is deliberately separate from model workers. Its process
// holds a repository-scoped credential and must inspect remote state before a
// side effect. An ambiguous result is left for operator reconciliation.
type PublishTransport interface {
	Inspect(context.Context, PublishIntent) (externalID string, exists bool, err error)
	Execute(context.Context, PublishIntent) (externalID string, err error)
}

// RunPublish accepts only a freshly prepared, live allow intent. A recovered
// sending/unknown attempt can be inspected, but never automatically retried.
func (s *Store) RunPublish(ctx context.Context, decisionID string, transport PublishTransport) (PublishIntent, error) {
	if transport == nil {
		return PublishIntent{}, errors.New("publisher transport required")
	}
	st := s.Snapshot()
	d, ok := st.Decisions[decisionID]
	if !ok || d.Operation != "branch_push" && d.Operation != "pr_create" {
		return PublishIntent{}, errors.New("publisher supports only branch push and PR create")
	}
	var intent PublishIntent
	for _, old := range st.Intents {
		if old.DecisionID == decisionID {
			intent = old
			break
		}
	}
	if intent.ID == "" || intent.Status == "pending_reconcile" {
		var err error
		intent, err = s.PreparePublish(decisionID, d.Operation, d.Repository, d.Ref, d.HeadSHA)
		if err != nil {
			return PublishIntent{}, err
		}
	}
	if intent.Status == "sent" {
		return intent, nil
	}
	if intent.Status != "pending_reconcile" && intent.Status != "unknown" && intent.Status != "sending" {
		return PublishIntent{}, errors.New("publish intent is not reconcilable")
	}
	externalID, exists, err := transport.Inspect(ctx, intent)
	if err != nil {
		return PublishIntent{}, err
	}
	if exists {
		if externalID == "" {
			return PublishIntent{}, errors.New("remote result has no identifier")
		}
		return s.finishPublish(intent.ID, "sent", externalID)
	}
	if intent.Status != "pending_reconcile" {
		return PublishIntent{}, errors.New("prior publish attempt is ambiguous; operator must reconcile")
	}
	if _, err := s.PreparePublish(decisionID, intent.Operation, intent.Repository, intent.Ref, intent.HeadSHA); err != nil {
		return PublishIntent{}, err
	}
	// The decision may expire during remote inspection. Recheck it under the
	// single writer lock immediately before the external operation.
	err = s.update(func(st *State) error {
		current := st.Intents[intent.ID]
		decision := st.Decisions[decisionID]
		if current.Status != "pending_reconcile" || decision.Verdict != "allow" || !time.Now().Before(decision.ExpiresAt) || decision.HeadSHA != intent.HeadSHA || st.Tasks[st.Agents[decision.AuthorAgentID].TaskID].HeadSHA != intent.HeadSHA {
			return errors.New("publish authorization changed before execution")
		}
		current.Status, current.UpdatedAt = "sending", time.Now().UTC()
		st.Intents[intent.ID] = current
		st.Events = append(st.Events, event("publish_sending", intent.ID, intent.Operation))
		return nil
	})
	if err != nil {
		return PublishIntent{}, err
	}
	externalID, err = transport.Execute(ctx, intent)
	if err != nil || externalID == "" {
		_, _ = s.finishPublish(intent.ID, "unknown", "")
		if err == nil {
			err = errors.New("publisher returned no external identifier")
		}
		return PublishIntent{}, err
	}
	return s.finishPublish(intent.ID, "sent", externalID)
}

func (s *Store) finishPublish(id, status, externalID string) (PublishIntent, error) {
	var out PublishIntent
	err := s.update(func(st *State) error {
		intent, ok := st.Intents[id]
		if !ok || intent.Status != "sending" && intent.Status != "unknown" && intent.Status != "pending_reconcile" {
			return errors.New("publish intent state changed")
		}
		if status == "sent" && externalID == "" || status != "sent" && externalID != "" {
			return errors.New("invalid publish result")
		}
		intent.Status, intent.ExternalID, intent.UpdatedAt = status, externalID, time.Now().UTC()
		st.Intents[id] = intent
		st.Events = append(st.Events, event("publish_"+status, id, externalID))
		out = intent
		return nil
	})
	return out, err
}
