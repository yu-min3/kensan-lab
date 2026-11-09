package jobs

// RejectDispatch records a definitive pre-admission runtime rejection. The
// caller must have received a 4xx response; transport/5xx uncertainty must never
// use this transition. Waiting retains the same input/key for a safe replay.
func (s *Store) RejectDispatch(owner Owner, id, state, reason string) (Record, error) {
	if (state == "waiting" && reason != "runtime_auth_required" && reason != "runtime_quota_wait") || (state == "failed" && reason != "runtime_request_rejected") {
		return Record{}, ErrInvalid
	}
	return s.change(owner, id, func(r *Record) error {
		if r.RuntimeJobID != "" || (r.Phase != "uncertain" && r.Phase != "reconciling") || (state != "waiting" && state != "failed") || reason == "" || len(reason) > 512 {
			return ErrInvalid
		}
		r.State = state
		r.Reason = reason
		r.Phase = "prepared"
		if state == "failed" {
			r.Phase = "terminal"
		}
		return nil
	})
}

func validBeforeLink(r Record) bool {
	return r.RuntimeJobID == "" && ((r.State == "queued" && r.Reason == "") || (r.State == "waiting" && (r.Reason == "runtime_auth_required" || r.Reason == "runtime_quota_wait")))
}
func validUnlinkedTerminal(r Record) bool {
	return r.RuntimeJobID == "" && ((r.State == "cancelled" && r.Reason == "cancelled_before_dispatch") || (r.State == "failed" && r.Reason == "runtime_request_rejected"))
}
