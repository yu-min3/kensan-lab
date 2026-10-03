package core

import (
	"errors"
	"time"
)

// resumable は人が中身を見てから手で戻す状態。配車側は拾わないので、
// ここを通さない限り作業は止まったままになる。
var resumable = map[string]bool{
	"auth_required": true,
	"interrupted":   true,
	"failed":        true,
}

// ResumeAgent は止まった agent を人の確認後に再開させる。
//
// 自動では戻さない。auth_required は本人の再認証、interrupted は worker 出力の
// 確認、failed は原因の手当てが先にあり、どれも controller には判断できない。
//
// 再開は必ず新しい session generation で始める。止まっている間に契約・仕様・
// 対象 SHA が動いている可能性があり、古い provider session を継ぎ足すと
// 入力が混ざるため（goal.md「入力・記憶・session の寿命」）。
func (s *Store) ResumeAgent(agentID, reason string) error {
	if reason == "" {
		return errors.New("resume reason is required")
	}
	return s.update(func(st *State) error {
		a, ok := st.Agents[agentID]
		if !ok {
			return errors.New("unknown agent")
		}
		if !resumable[a.Status] {
			return errors.New("agent is not in a resumable state: " + a.Status)
		}
		now := time.Now().UTC()
		a.Status, a.RetryAfter, a.AttemptCount = "ready", nil, 0
		a.SessionID, a.InputHash = "", ""
		a.SessionGeneration++
		a.UpdatedAt = now
		st.Agents[agentID] = a
		st.Events = append(st.Events, event("agent_resumed", agentID, reason))
		return nil
	})
}
