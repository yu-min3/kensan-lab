package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// These artifacts are evidence and decisions, not deployment authority.
const PlatformFeedbackSchemaVersion = 1

type PlatformFeedback struct {
	SchemaVersion int    `json:"schema_version"`
	Category      string `json:"category"`
	Summary       string `json:"summary"`
	Expected      string `json:"expected"`
	Observed      string `json:"observed"`
	Reproduce     string `json:"reproduce"`
}

type PlatformDecision struct {
	SchemaVersion     int    `json:"schema_version"`
	FeedbackMessageID string `json:"feedback_message_id"`
	Verdict           string `json:"verdict"` // adopt, reject, defer
	Reason            string `json:"reason"`
}

func (s *Store) PutPlatformFeedback(agentID string, feedback PlatformFeedback) (Artifact, error) {
	if err := feedback.validate(); err != nil {
		return Artifact{}, err
	}
	return putPlatformEnvelope(s, agentID, "platform_feedback", feedback)
}

func (s *Store) PutPlatformDecision(agentID string, decision PlatformDecision) (Artifact, error) {
	if err := decision.validate(); err != nil {
		return Artifact{}, err
	}
	return putPlatformEnvelope(s, agentID, "platform_decision", decision)
}

func putPlatformEnvelope(s *Store, agentID, kind string, value any) (Artifact, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return Artifact{}, err
	}
	return s.PutArtifact(agentID, kind, data)
}

func (s *Store) ReadPlatformFeedback(messageID string) (PlatformFeedback, error) {
	m, ok := s.Snapshot().Messages[messageID]
	if !ok || m.Kind != "platform_feedback" || len(m.ArtifactRefs) != 1 {
		return PlatformFeedback{}, errors.New("platform feedback message not found")
	}
	data, err := s.ReadArtifact(m.ArtifactRefs[0].ID)
	if err != nil {
		return PlatformFeedback{}, err
	}
	var f PlatformFeedback
	if err := decodePlatformEnvelope(data, &f); err != nil {
		return PlatformFeedback{}, err
	}
	return f, f.validate()
}

func (s *Store) ReadPlatformDecision(messageID string) (PlatformDecision, error) {
	m, ok := s.Snapshot().Messages[messageID]
	if !ok || m.Kind != "platform_decision" || len(m.ArtifactRefs) != 1 {
		return PlatformDecision{}, errors.New("platform decision message not found")
	}
	data, err := s.ReadArtifact(m.ArtifactRefs[0].ID)
	if err != nil {
		return PlatformDecision{}, err
	}
	var d PlatformDecision
	if err := decodePlatformEnvelope(data, &d); err != nil {
		return PlatformDecision{}, err
	}
	return d, d.validate()
}

func (f PlatformFeedback) validate() error {
	if f.SchemaVersion != PlatformFeedbackSchemaVersion || strings.TrimSpace(f.Category) == "" || strings.TrimSpace(f.Summary) == "" || strings.TrimSpace(f.Expected) == "" || strings.TrimSpace(f.Observed) == "" || strings.TrimSpace(f.Reproduce) == "" {
		return errors.New("platform feedback requires version 1, category, summary, expected, observed and reproduction")
	}
	return nil
}

func (d PlatformDecision) validate() error {
	if d.SchemaVersion != PlatformFeedbackSchemaVersion || d.FeedbackMessageID == "" || strings.TrimSpace(d.Reason) == "" {
		return errors.New("platform decision requires version 1, feedback message and reason")
	}
	switch d.Verdict {
	case "adopt", "reject", "defer":
		return nil
	default:
		return errors.New("platform decision verdict must be adopt, reject or defer")
	}
}

func decodePlatformEnvelope[T any](data []byte, out *T) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("trailing platform envelope content")
	}
	return nil
}

// validatePlatformMessage checks the typed artifact before SendMessage records
// the cross-team envelope. The generic message checks still enforce ownership,
// mission, contract version, immutable artifact references and reply pairing.
func (s *Store) validatePlatformMessage(st State, m Message) error {
	if m.Kind != "platform_feedback" && m.Kind != "platform_decision" {
		return nil
	}
	from, fromOK := st.Agents[m.FromAgent]
	_, toOK := st.Agents[m.ToAgent]
	if !fromOK || !toOK || len(m.ArtifactRefs) != 1 {
		return errors.New("platform envelope needs one artifact and known agents")
	}
	if err := validatePlatformRoute(&st, m); err != nil {
		return err
	}
	a, ok := st.Artifacts[m.ArtifactRefs[0].ID]
	if !ok || a.Kind != m.Kind || a.AgentID != from.ID || a.TaskID != m.SourceTask {
		return errors.New("platform envelope artifact ownership or kind mismatch")
	}
	data, err := s.ReadArtifact(a.ID)
	if err != nil {
		return fmt.Errorf("platform envelope artifact: %w", err)
	}
	switch m.Kind {
	case "platform_feedback":
		var f PlatformFeedback
		if err := decodePlatformEnvelope(data, &f); err != nil {
			return err
		}
		if err := f.validate(); err != nil {
			return err
		}
	case "platform_decision":
		var d PlatformDecision
		if err := decodePlatformEnvelope(data, &d); err != nil {
			return err
		}
		if err := d.validate(); err != nil {
			return err
		}
		if d.FeedbackMessageID != m.ReplyTo {
			return errors.New("platform decision must reply to received feedback")
		}
	}
	return nil
}

func validatePlatformRoute(st *State, m Message) error {
	if m.Kind != "platform_feedback" && m.Kind != "platform_decision" {
		return nil
	}
	from, fromOK := st.Agents[m.FromAgent]
	to, toOK := st.Agents[m.ToAgent]
	if !fromOK || !toOK || len(m.ArtifactRefs) != 1 {
		return errors.New("platform envelope needs one artifact and known agents")
	}
	if m.Kind == "platform_feedback" {
		if from.Team != App || to.Team != Platform || to.Role != "feedback" || st.Tasks[to.TaskID].Kind != "analysis" || m.ReplyTo != "" {
			return errors.New("platform feedback must go from App to Platform feedback analysis")
		}
		return nil
	}
	prior, ok := st.Messages[m.ReplyTo]
	if from.Team != Platform || from.Role != "feedback" || to.Team != App || st.Tasks[from.TaskID].Kind != "analysis" || !ok || prior.Kind != "platform_feedback" || prior.Status != "received" || prior.FromAgent != to.ID || prior.ToAgent != from.ID || prior.CorrelationID != m.CorrelationID {
		return errors.New("platform decision must reply to received feedback")
	}
	for _, previous := range st.Messages {
		if previous.Kind == "platform_decision" && previous.ReplyTo == prior.ID && previous.ID != m.ID {
			return errors.New("platform feedback already has a decision")
		}
	}
	return nil
}
