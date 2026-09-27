package core

import (
	"errors"
	"strings"
	"time"
)

func (s *Store) AskQuestion(agentID, prompt string) (Question, error) {
	if strings.TrimSpace(prompt) == "" || len(prompt) > 20000 {
		return Question{}, errors.New("question must be 1 to 20000 characters")
	}
	id, err := newID()
	if err != nil {
		return Question{}, err
	}
	var q Question
	err = s.update(func(st *State) error {
		a, ok := st.Agents[agentID]
		if !ok {
			return errors.New("question agent not found")
		}
		t := st.Tasks[a.TaskID]
		if t.Status == "done" || t.Status == "failed" {
			return errors.New("task cannot ask a new question")
		}
		q = Question{ID: id, TaskID: t.ID, AgentID: a.ID, Prompt: prompt, ContractVersion: t.ContractVersion, HeadSHA: t.HeadSHA, Status: "pending", CreatedAt: time.Now().UTC()}
		st.Questions[id] = q
		st.Events = append(st.Events, event("question_asked", id, t.ID))
		return nil
	})
	return q, err
}

func (s *Store) AnswerQuestion(questionID, actionID, answer string) (Question, error) {
	if actionID == "" || strings.TrimSpace(answer) == "" || len(answer) > 20000 {
		return Question{}, errors.New("action ID and answer of 1 to 20000 characters required")
	}
	var out Question
	err := s.update(func(st *State) error {
		q, ok := st.Questions[questionID]
		if !ok {
			return errors.New("question not found")
		}
		if q.Status == "answered" {
			if q.ActionID == actionID && q.Answer == answer {
				out = q
				return nil
			}
			return errors.New("question already answered with another action")
		}
		t, ok := st.Tasks[q.TaskID]
		if !ok || t.ContractVersion != q.ContractVersion || t.HeadSHA != q.HeadSHA {
			return errors.New("question belongs to a stale contract or SHA")
		}
		for _, existing := range st.Questions {
			if existing.ActionID == actionID && existing.ID != questionID {
				return errors.New("action ID already used")
			}
		}
		now := time.Now().UTC()
		q.Status, q.Answer, q.ActionID, q.AnsweredAt = "answered", answer, actionID, &now
		st.Questions[questionID] = q
		st.Events = append(st.Events, event("question_answered", questionID, actionID))
		out = q
		return nil
	})
	return out, err
}

func (s *Store) RequestApproval(decisionID string) (ApprovalRequest, error) {
	id, err := newID()
	if err != nil {
		return ApprovalRequest{}, err
	}
	var out ApprovalRequest
	err = s.update(func(st *State) error {
		d, ok := st.Decisions[decisionID]
		if !ok || d.Verdict != "needs_human" || !time.Now().Before(d.ExpiresAt) {
			return errors.New("live needs_human release decision required")
		}
		if st.Tasks[st.Agents[d.AuthorAgentID].TaskID].HeadSHA != d.HeadSHA {
			return errors.New("approval request head is stale")
		}
		for _, old := range st.Approvals {
			if old.DecisionID == decisionID {
				out = old
				return nil
			}
		}
		out = ApprovalRequest{ID: id, DecisionID: decisionID, Operation: d.Operation, Repository: d.Repository, Ref: d.Ref, HeadSHA: d.HeadSHA, Environment: d.TargetEnvironment, Reason: d.Reason, Status: "pending", CreatedAt: time.Now().UTC(), ExpiresAt: d.ExpiresAt}
		st.Approvals[id] = out
		st.Events = append(st.Events, event("approval_requested", id, decisionID))
		return nil
	})
	return out, err
}

// DecideApproval records Yu's judgment. It does not turn a needs_human gate
// verdict into allow or invoke a publisher; a fresh independent gate decision
// is still necessary for a private-stage external operation.
func (s *Store) DecideApproval(requestID, actionID, operation, sha, verdict string) (ApprovalRequest, error) {
	if actionID == "" || (verdict != "approved" && verdict != "denied") {
		return ApprovalRequest{}, errors.New("action ID and approved/denied verdict required")
	}
	var out ApprovalRequest
	err := s.update(func(st *State) error {
		request, ok := st.Approvals[requestID]
		if !ok {
			return errors.New("approval request not found")
		}
		if request.Status != "pending" {
			if request.ActionID == actionID && request.Status == verdict && request.Operation == operation && request.HeadSHA == sha {
				out = request
				return nil
			}
			return errors.New("approval request already decided")
		}
		if request.Operation != operation || request.HeadSHA != sha || !time.Now().Before(request.ExpiresAt) {
			return errors.New("approval operation, SHA or expiry mismatch")
		}
		d, ok := st.Decisions[request.DecisionID]
		if !ok || d.Verdict != "needs_human" || d.Operation != operation || d.HeadSHA != sha || d.PolicyVersion != ReleasePolicyVersion {
			return errors.New("release gate decision changed")
		}
		if st.Tasks[st.Agents[d.AuthorAgentID].TaskID].HeadSHA != sha {
			return errors.New("task head changed after approval request")
		}
		for _, existing := range st.Approvals {
			if existing.ActionID == actionID && existing.ID != requestID {
				return errors.New("action ID already used")
			}
		}
		now := time.Now().UTC()
		request.Status, request.ActionID, request.DecidedAt = verdict, actionID, &now
		st.Approvals[requestID] = request
		st.Events = append(st.Events, event("approval_"+verdict, requestID, actionID))
		out = request
		return nil
	})
	return out, err
}
